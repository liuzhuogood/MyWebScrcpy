#!/usr/bin/env python3
"""快手极速版当前页面类型识别（基于 YAML 规则与四区域粗定位）

读取 page_rules.yaml（由 config.py 解析）：
- 区域 (1, 2, 3, 4)
- 文字列表 (words: [...])
- 可信度 (confidence: 0.0 ~ 1.0)
"""

import argparse
from dataclasses import dataclass, field
import json
import os
from pathlib import Path
import re
import sys
from typing import Any
from urllib.request import Request, urlopen

# 导入配置管理器
sys.path.insert(0, str(Path(__file__).parent))
from config import PageType, config  # noqa: E402


@dataclass
class PageDetectionResult:
    """页面判定结果"""
    page: PageType
    confidence: float
    matched_features: list[str] = field(default_factory=list)
    scores: dict[str, float] = field(default_factory=dict)
    regional_box_counts: dict[str, int] = field(default_factory=dict)
    raw_text_length: int = 0
    activity: str | None = None

    def to_dict(self) -> dict[str, Any]:
        return {
            "page": self.page.value,
            "page_name": self.page.name,
            "confidence": round(self.confidence, 2),
            "matched_features": self.matched_features,
            "scores": {k: round(v, 2) for k, v in self.scores.items()},
            "regional_box_counts": self.regional_box_counts,
            "raw_text_length": self.raw_text_length,
            "activity": self.activity,
        }


def evaluate_page(
    ocr_boxes: list[dict[str, Any]] | None = None,
    ocr_text: str | None = None,
    template_matches: list[dict[str, Any]] | None = None,
    activity: str | None = None,
) -> PageDetectionResult:
    """综合四区域文字粗定位、全屏文本、模板匹配与 Activity 进行页面判定"""
    boxes = ocr_boxes or []
    matches = template_matches or []
    if not ocr_text:
        ocr_text = "\n".join(b.get("text", "") for b in boxes)

    if not ocr_text.strip() and not matches:
        return PageDetectionResult(
            page=PageType.UNKNOWN,
            confidence=0.0,
            matched_features=["画面无文字识别且无模板命中结果"],
            scores={},
            regional_box_counts={},
            raw_text_length=0,
            activity=activity,
        )

    clean_text = ocr_text.replace(" ", "").replace("\n", " ")
    scores: dict[str, float] = {p.value: 0.0 for p in PageType if p != PageType.UNKNOWN}
    matched_by_page: dict[str, list[str]] = {p.value: [] for p in PageType if p != PageType.UNKNOWN}

    # 1. 四区域文本框分布统计与文本归集
    regional_texts: dict[int, list[str]] = {r: [] for r in config.regions}
    regional_counts: dict[str, int] = {config.get_region_name(r): 0 for r in config.regions}

    for box in boxes:
        reg_id = config.locate_box_region(box)
        text = box.get("text", "").strip()
        if text:
            regional_texts[reg_id].append(text)
            regional_counts[config.get_region_name(reg_id)] += 1

    regional_joined: dict[int, str] = {
        r: "".join(texts).replace(" ", "") for r, texts in regional_texts.items()
    }

    # 2. 遍历各页面 YAML 规则进行匹配打分
    for page_name, page_rule in config.pages.items():
        if page_name not in scores:
            scores[page_name] = 0.0
            matched_by_page[page_name] = []

        # 匹配词组规则 (words + regions + confidence)
        for rule_item in page_rule.rules:
            for word in rule_item.words:
                if word not in clean_text:
                    continue

                # 检查文字是否出现在指定的区域内
                matched_in_target_region = False
                hit_reg = None
                for target_reg in rule_item.regions:
                    if word in regional_joined.get(target_reg, ""):
                        matched_in_target_region = True
                        hit_reg = target_reg
                        break

                if matched_in_target_region:
                    # 在目标区域命中：获得 100% 可信度得分
                    score = rule_item.confidence * 1.5
                    reg_label = f"[区域{hit_reg}:{config.get_region_name(hit_reg)}]"
                    matched_by_page[page_name].append(f"{word}{reg_label}")
                elif regional_boxes_available := bool(boxes):
                    # 有坐标但落在其他区域：衰减为 40% 得分
                    score = rule_item.confidence * 0.4
                    matched_by_page[page_name].append(f"{word}[偏离预期区域]")
                else:
                    # 纯文本模式：取 80% 基础分
                    score = rule_item.confidence * 0.8
                    matched_by_page[page_name].append(f"{word}")

                scores[page_name] += score

        # 匹配模板规则 (template/templates + regions + confidence)
        if matches:
            for rule_item in page_rule.rules:
                if not rule_item.templates:
                    continue
                for target_tmpl in rule_item.templates:
                    for match in matches:
                        match_name = match.get("template_name") or match.get("template_id") or ""
                        match_id = match.get("template_id") or ""
                        if target_tmpl == match_name or target_tmpl == match_id:
                            match_conf = float(match.get("confidence", 0.9))
                            match_reg = config.locate_box_region(match)
                            in_target_reg = not rule_item.regions or match_reg in rule_item.regions

                            if in_target_reg:
                                score = rule_item.confidence * match_conf * 1.5
                                reg_name = config.get_region_name(match_reg)
                                matched_by_page[page_name].append(
                                    f"[模板]{match_name}[区域{match_reg}:{reg_name}, 相似度{match_conf:.2f}]"
                                )
                            else:
                                score = rule_item.confidence * match_conf * 0.4
                                matched_by_page[page_name].append(
                                    f"[模板]{match_name}[偏离预期区域{match_reg}]"
                                )
                            scores[page_name] += score

        # 匹配正则模式
        for pattern_item in page_rule.patterns:
            if re.search(pattern_item.regex, clean_text):
                score = pattern_item.confidence * 1.2
                scores[page_name] += score
                matched_by_page[page_name].append(f"[正则]{pattern_item.regex}")

    # 3. Activity 加权修正
    if activity:
        act_lower = activity.lower()
        if "live" in act_lower and PageType.LIVE.value in scores:
            scores[PageType.LIVE.value] += 2.0
            matched_by_page[PageType.LIVE.value].append(f"[Activity]{activity}")
        elif ("reward" in act_lower or "ad" in act_lower) and PageType.AD.value in scores:
            scores[PageType.AD.value] += 2.0
            matched_by_page[PageType.AD.value].append(f"[Activity]{activity}")

    # 4. 冲突仲裁与裁决
    # 若配置声明为 priority (如弹窗) 且达到阈值，优先裁决为该页面
    priority_page = None
    for p_name, p_rule in config.pages.items():
        if p_rule.priority and scores.get(p_name, 0.0) >= 2.0:
            priority_page = p_name
            break

    if priority_page:
        top_page_name = priority_page
    else:
        # 按综合最高分裁决
        sorted_scores = sorted(scores.items(), key=lambda x: x[1], reverse=True)
        top_page_name, best_score = sorted_scores[0]
        if best_score < 1.0:
            top_page_name = PageType.UNKNOWN.value

    top_page = next((p for p in PageType if p.value == top_page_name), PageType.UNKNOWN)
    total_score = sum(scores.values())
    confidence = (scores[top_page.value] / total_score) if total_score > 0 and top_page != PageType.UNKNOWN else 0.0

    return PageDetectionResult(
        page=top_page,
        confidence=min(1.0, max(0.0, confidence)),
        matched_features=matched_by_page.get(top_page.value, []),
        scores=scores,
        regional_box_counts=regional_counts,
        raw_text_length=len(ocr_text),
        activity=activity,
    )


def evaluate_page_from_text(ocr_text: str, activity: str | None = None) -> PageDetectionResult:
    """仅凭文本计算页面类型（向后兼容纯文本输入）"""
    return evaluate_page(ocr_boxes=None, ocr_text=ocr_text, activity=activity)


# ==================== 设备与服务端通信 ====================

def get_base_url() -> str:
    url = os.environ.get("MYWEBSCRCPY_BASE_URL") or os.environ.get("MYWEBSCRCPY_API_URL") or "http://127.0.0.1:8080"
    return url.rstrip("/")


def resolve_device_serial(arg_serial: str | None, base_url: str) -> str:
    """自动解析设备 serial：CLI参数 > 环境变量 > 唯一在线设备推断"""
    if arg_serial and arg_serial.strip():
        return arg_serial.strip()

    env_serial = os.environ.get("MYWEBSCRCPY_SERIAL") or os.environ.get("SERIAL")
    if env_serial and env_serial.strip():
        return env_serial.strip()

    try:
        req = Request(f"{base_url}/api/devices")
        with urlopen(req, timeout=5) as resp:
            data = json.loads(resp.read().decode())
            devices = [d["serial"] for d in data if d.get("state") == "device"]
            if len(devices) == 1:
                return devices[0]
            elif len(devices) > 1:
                raise ValueError(f"检测到多台在线设备 ({', '.join(devices)})，请使用 --serial 显式指定")
            else:
                raise ValueError("未检测到在线设备")
    except Exception as e:
        if isinstance(e, ValueError):
            raise
        raise ValueError(f"获取在线设备列表失败: {e}") from e


def fetch_device_state(serial: str, base_url: str) -> dict[str, Any]:
    """调用 /api/device-state/{serial} 获取当前总状态（含 OCR 与页面信息）"""
    url = f"{base_url}/api/device-state/{serial}"
    req = Request(url)
    with urlopen(req, timeout=10) as resp:
        if resp.status != 200:
            raise RuntimeError(f"HTTP {resp.status} 读取设备状态失败")
        return json.loads(resp.read().decode())


def identify_page(serial: str | None = None, base_url: str | None = None) -> PageDetectionResult:
    """采集并识别指定设备的当前页面类型"""
    base = (base_url or get_base_url()).rstrip("/")
    resolved_serial = resolve_device_serial(serial, base)
    state = fetch_device_state(resolved_serial, base)
    ocr_data = state.get("ocr") or {}
    ocr_boxes = ocr_data.get("boxes") or []
    ocr_text = ocr_data.get("text", "")
    template_matches = state.get("template_matches") or []
    page_info = state.get("page_info") or {}
    activity = page_info.get("current_activity")
    return evaluate_page(
        ocr_boxes=ocr_boxes,
        ocr_text=ocr_text,
        template_matches=template_matches,
        activity=activity,
    )


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--serial", help="设备序列号（若只有1台设备可省略自动绑定）")
    parser.add_argument("--base-url", default=None, help="MyWebScrcpy 服务根地址")
    parser.add_argument("--json", action="store_true", help="输出完整结构化 JSON")
    args = parser.parse_args()

    base_url = args.base_url or get_base_url()
    try:
        serial = resolve_device_serial(args.serial, base_url)
    except ValueError as e:
        if args.json:
            print(json.dumps({"error": str(e)}, ensure_ascii=False))
        else:
            print(f"[错误] {e}", file=sys.stderr)
        return 1

    try:
        result = identify_page(serial, base_url)
    except Exception as e:
        if args.json:
            print(json.dumps({"error": f"识别失败: {e}"}, ensure_ascii=False))
        else:
            print(f"[错误] 识别失败: {e}", file=sys.stderr)
        return 1

    if args.json:
        print(json.dumps(result.to_dict(), ensure_ascii=False, indent=2))
    else:
        print(f"当前页面: {result.page.value}")
        print(f"置信度:   {result.confidence:.0%}")
        if result.matched_features:
            print(f"命中文本: {', '.join(result.matched_features)}")
        if result.regional_box_counts:
            summary_str = ", ".join(f"{k}: {v}个文本框" for k, v in result.regional_box_counts.items())
            print(f"区域分布: {summary_str}")
        print(f"得分明细: {result.scores}")

    return 0


if __name__ == "__main__":
    sys.argv = ["page.py", "--serial", "10.0.0.30:5555"]
    raise SystemExit(main())
