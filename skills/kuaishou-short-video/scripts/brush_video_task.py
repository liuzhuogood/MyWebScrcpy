#!/usr/bin/env python3
"""快手极速版：刷视频任务脚本 (brush_video_task.py)

严格遵循 tasks/刷视频任务.md 定义的流程：
1. 定位至任务中心页面；
2. 指定区域 OCR 识别 (默认 BBox: x:19, y:365, w:420, h:85，支持基准分辨率换算与归一化)；
3. 解析已领金币、当前进度 current / 目标 target 以及按键文案；
4. 若 current >= target 或无“去看视频”按钮，判定任务完成；
5. 若待继续，点击区域内『去看视频』进入视频流；
6. 拟人化浏览短视频（停留 5~15s，随机轻微滑动），完成单轮批次；
7. 返回并复核进度，进入下一轮循环直至完成或达到安全轮次上限。
"""

import argparse
from dataclasses import asdict, dataclass
import json
import logging
import os
from pathlib import Path
import random
import re
import sys
import time
from typing import Any
from urllib.error import URLError
from urllib.request import Request, urlopen

# 添加当前目录与通用技能目录至 path
CURRENT_DIR = Path(__file__).resolve().parent
COMMON_SCRIPTS_DIR = CURRENT_DIR.parents[1] / "my-scrcpy-common" / "scripts"

if str(CURRENT_DIR) not in sys.path:
    sys.path.insert(0, str(CURRENT_DIR))
if str(COMMON_SCRIPTS_DIR) not in sys.path:
    sys.path.insert(0, str(COMMON_SCRIPTS_DIR))

from navigate import KuaishouNavigator, get_base_url, resolve_device_serial

logging.basicConfig(
    level=logging.INFO,
    format="[%(asctime)s] [%(levelname)s] %(message)s",
    datefmt="%H:%M:%S",
)
logger = logging.getLogger("brush_video_task")

# 任务定义中的默认基准区域（基准分辨率 440 x 960 或原尺寸比例）
DEFAULT_BBOX_PIXELS = {"x": 19, "y": 365, "width": 420, "height": 85}
DEFAULT_REFERENCE_WIDTH = 440.0
DEFAULT_REFERENCE_HEIGHT = 960.0

# 默认快手极速版包名
KUAISHOU_PACKAGE = "com.kuaishou.nebula"


@dataclass
class TaskProgress:
    """刷视频任务进度数据模型"""
    claimed_coins: int | None = None
    current: int = 0
    target: int = 0
    is_completed: bool = False
    button_text: str = ""
    raw_text: str = ""
    click_target: dict[str, float] | None = None

    def to_dict(self) -> dict[str, Any]:
        return asdict(self)


class BrushVideoTask:
    """快手极速版看视频得金币任务执行器"""

    def __init__(
        self,
        serial: str,
        base_url: str | None = None,
        bbox: dict[str, float] | None = None,
        ref_width: float = DEFAULT_REFERENCE_WIDTH,
        ref_height: float = DEFAULT_REFERENCE_HEIGHT,
        batch_video_count: int = 5,
        min_stay: float = 5.0,
        max_stay: float = 15.0,
        max_cycles: int = 20,
    ):
        self.serial = serial
        self.base_url = (base_url or get_base_url()).rstrip("/")
        self.bbox_cfg = bbox or DEFAULT_BBOX_PIXELS
        self.ref_width = ref_width
        self.ref_height = ref_height
        self.batch_video_count = batch_video_count
        self.min_stay = min_stay
        self.max_stay = max_stay
        self.max_cycles = max_cycles
        self.navigator = KuaishouNavigator(serial=self.serial, base_url=self.base_url)

    def _post_json(self, path: str, payload: dict[str, Any], timeout: float = 15.0) -> dict[str, Any]:
        """向服务端发送 JSON POST 请求"""
        url = f"{self.base_url}{path}"
        data = json.dumps(payload).encode("utf-8")
        req = Request(url, data=data, headers={"Content-Type": "application/json"})
        try:
            with urlopen(req, timeout=timeout) as resp:
                raw = resp.read().decode("utf-8")
                return json.loads(raw) if raw else {}
        except URLError as e:
            raise RuntimeError(f"请求 {url} 失败: {e}") from e

    def _get_normalized_bbox(self) -> list[float]:
        """将像素配置换算为 [0.0, 1.0] 的归一化 [x, y, w, h] 坐标"""
        x = self.bbox_cfg.get("x", 19.0)
        y = self.bbox_cfg.get("y", 365.0)
        w = self.bbox_cfg.get("width", self.bbox_cfg.get("w", 420.0))
        h = self.bbox_cfg.get("height", self.bbox_cfg.get("h", 85.0))

        # 若已经是归一化比例 (<= 1.0) 则直接使用
        if x <= 1.0 and y <= 1.0 and w <= 1.0 and h <= 1.0:
            return [float(x), float(y), float(w), float(h)]

        norm_x = max(0.0, min(1.0, x / self.ref_width))
        norm_y = max(0.0, min(1.0, y / self.ref_height))
        norm_w = max(0.01, min(1.0 - norm_x, w / self.ref_width))
        norm_h = max(0.01, min(1.0 - norm_y, h / self.ref_height))
        return [round(norm_x, 4), round(norm_y, 4), round(norm_w, 4), round(norm_h, 4)]

    def ensure_task_center(self) -> bool:
        """确保当前处于任务中心，必要时调用导航回到任务中心"""
        logger.info(f"[{self.serial}] 检查并定位至任务中心页面...")
        nav_res = self.navigator.navigate_to_task_center(max_retries=3, wait_seconds=2.0)
        if not nav_res.get("success"):
            logger.warning(f"[{self.serial}] 导航到任务中心未完全确认，继续尝试后续 OCR: {nav_res}")
        return True

    def ocr_task_area(self, retry_limit: int = 3) -> TaskProgress | None:
        """指定区域 OCR 识别与数据解析 (包含重试与滚动防抖)"""
        norm_bbox = self._get_normalized_bbox()
        logger.info(f"[{self.serial}] 开始指定区域 OCR 识别 (BBox 归一化: {norm_bbox})...")

        for attempt in range(1, retry_limit + 1):
            try:
                payload = {
                    "serial": self.serial,
                    "bbox": norm_bbox,
                    "stabilization_ms": 300,
                    "mark": True,
                }
                res = self._post_json("/api/vision/ocr", payload, timeout=12.0)
                raw_text = res.get("text", "")
                boxes = res.get("boxes", [])

                progress = self._parse_ocr_result(raw_text, boxes, norm_bbox)
                if progress:
                    logger.info(
                        f"[{self.serial}] 第 {attempt} 次 OCR 解析成功 -> 已领金币: {progress.claimed_coins}, "
                        f"进度: {progress.current}/{progress.target}, 按钮: '{progress.button_text}', 完成: {progress.is_completed}"
                    )
                    return progress

                # 若指定区域内文本为空，尝试在更广范围或微调滚动重试
                logger.warning(
                    f"[{self.serial}] 第 {attempt}/{retry_limit} 次 OCR 未能提取有效任务数据 (返回文本: {raw_text.strip()})"
                )
            except Exception as e:
                logger.warning(f"[{self.serial}] 第 {attempt}/{retry_limit} 次 OCR 请求异常: {e}")

            if attempt < retry_limit:
                # 短暂等待与微调微滑
                logger.info(f"[{self.serial}] 等待 1.5 秒后进行微调并重试 OCR...")
                time.sleep(1.5)
                # 微量上滑微调页面（防止被活动浮层遮挡或偏移）
                self.navigator.tap(0.5, 0.4)
                time.sleep(1.0)

        return None

    def _parse_ocr_result(
        self,
        raw_text: str,
        boxes: list[dict[str, Any]],
        norm_bbox: list[float],
    ) -> TaskProgress | None:
        """依据正则与关键字解析 OCR 结果"""
        clean_text = raw_text.replace(" ", "")

        # 1. 已领金币正则匹配：已领(\d+)金币
        coin_match = re.search(r"已领(\d+)金币", clean_text)
        claimed_coins = int(coin_match.group(1)) if coin_match else None

        # 2. 任务进度正则匹配：(\d+)/(\d+)
        progress_match = re.search(r"(\d+)/(\d+)", clean_text)
        current = 0
        target = 0
        if progress_match:
            current = int(progress_match.group(1))
            target = int(progress_match.group(2))

        # 3. 识别按键文案及点击坐标
        button_text = ""
        click_target = None

        # 优先查找精确的交互按钮（如“去看视频”、“去完成”），排除“已领...得金币”等长说明文案
        target_button_keywords = ["去看视频", "去完成", "立即观看"]
        for b in boxes:
            t = b.get("text", "").replace(" ", "")
            if any(k in t for k in target_button_keywords) and "已领" not in t and "继续看视频得金币" not in t:
                button_text = t
                bx = float(b.get("x", 0.0))
                by = float(b.get("y", 0.0))
                bw = float(b.get("w", 0.0))
                bh = float(b.get("h", 0.0))
                click_target = {
                    "x": bx + bw / 2 if bw > 0 else bx,
                    "y": by + bh / 2 if bh > 0 else by,
                }
                break

        # 次级容错查找
        if not button_text:
            for b in boxes:
                t = b.get("text", "").replace(" ", "")
                if ("看视频" in t or "去完成" in t) and "已领" not in t:
                    button_text = t
                    bx = float(b.get("x", 0.0))
                    by = float(b.get("y", 0.0))
                    bw = float(b.get("w", 0.0))
                    bh = float(b.get("h", 0.0))
                    click_target = {
                        "x": bx + bw / 2 if bw > 0 else bx,
                        "y": by + bh / 2 if bh > 0 else by,
                    }
                    break

        # 如果没有单独匹配到具体 box，但文本中存在“去看视频”，默认点击 BBox 右侧按钮区域
        if not click_target and ("去看视频" in clean_text or "继续看视频" in clean_text):
            button_text = "去看视频"
            # 按钮通常在卡片右侧 (x 约占 75% ~ 90%)
            click_target = {
                "x": norm_bbox[0] + norm_bbox[2] * 0.82,
                "y": norm_bbox[1] + norm_bbox[3] * 0.50,
            }

        # 4. 完成状态判断
        is_completed = False
        if "已完成" in clean_text or "明日再来" in clean_text:
            is_completed = True
        elif target > 0 and current >= target:
            is_completed = True
        elif not button_text and progress_match and current > 0:
            # 提取到了进度但无“去看视频”按钮，可能是已领完状态
            is_completed = True

        # 只要命中了进度或金币或按钮任一核心特征，即认为解析成功
        if progress_match or claimed_coins is not None or button_text:
            return TaskProgress(
                claimed_coins=claimed_coins,
                current=current,
                target=target,
                is_completed=is_completed,
                button_text=button_text,
                raw_text=raw_text,
                click_target=click_target,
            )

        return None

    def trigger_watch_video(self, progress: TaskProgress) -> bool:
        """点击『去看视频』按钮进入短视频信息流"""
        target = progress.click_target
        if not target:
            norm_bbox = self._get_normalized_bbox()
            target = {
                "x": norm_bbox[0] + norm_bbox[2] * 0.82,
                "y": norm_bbox[1] + norm_bbox[3] * 0.50,
            }

        logger.info(f"[{self.serial}] 点击『去看视频』位置: ({target['x']:.4f}, {target['y']:.4f})")
        ok = self.navigator.tap(target["x"], target["y"])
        if ok:
            # 等待页面跳转进入视频流
            time.sleep(2.5)
        return ok

    def run_video_feed_batch(self) -> dict[str, Any]:
        """在短视频信息流中执行单轮刷视频批次（拟人化滑动与停留）"""
        count = self.batch_video_count
        logger.info(
            f"[{self.serial}] 开始执行单轮刷视频批次 (目标数量: {count} 条, 停留时长: {self.min_stay}~{self.max_stay}s)..."
        )

        watched = 0
        cycle_start = time.monotonic()

        for i in range(1, count + 1):
            stay_seconds = round(random.uniform(self.min_stay, self.max_stay), 2)
            logger.info(f"[{self.serial}] 正在观看第 {i}/{count} 条短视频，停留 {stay_seconds}s...")
            time.sleep(stay_seconds)

            # 拟人化轻微上下滑动与上滑翻页
            swipe_ok = self._humanized_swipe_next()
            if swipe_ok:
                watched += 1
            else:
                logger.warning(f"[{self.serial}] 第 {i} 条视频翻页滑动失败")

            # 短暂停顿让视频开始播放
            time.sleep(random.uniform(0.8, 1.5))

        total_elapsed = round(time.monotonic() - cycle_start, 2)
        logger.info(f"[{self.serial}] 单轮刷视频批次结束: 观看 {watched}/{count} 条视频，耗时 {total_elapsed}s")
        return {
            "watched_count": watched,
            "target_count": count,
            "elapsed_seconds": total_elapsed,
        }

    def _humanized_swipe_next(self) -> bool:
        """执行拟人化随机上滑动作"""
        x1 = round(0.50 + random.uniform(-0.05, 0.05), 4)
        y1 = round(0.78 + random.uniform(-0.03, 0.03), 4)
        x2 = round(0.50 + random.uniform(-0.05, 0.05), 4)
        y2 = round(0.22 + random.uniform(-0.03, 0.03), 4)
        duration_ms = random.randint(380, 550)

        payload = {
            "serial": self.serial,
            "x1": x1,
            "y1": y1,
            "x2": x2,
            "y2": y2,
            "duration_ms": duration_ms,
            "humanize": True,
        }
        try:
            res = self._post_json("/api/v1/actions/swipe", payload, timeout=10.0)
            return res.get("status") == "executed" or res.get("executed") is True
        except Exception as e:
            logger.warning(f"[{self.serial}] 上滑请求异常: {e}")
            return False

    def return_to_task_center(self) -> bool:
        """退出短视频流返回任务中心"""
        logger.info(f"[{self.serial}] 退出视频流，返回任务中心...")
        # 优先直接按一次返回键
        self.navigator.press_back()
        time.sleep(2.0)

        # 检查是否回到了含主底栏或任务中心的界面，若有退出挽留弹窗则处理
        return self.ensure_task_center()

    def run(self) -> dict[str, Any]:
        """执行完整的刷视频任务循环主流程"""
        started_at = time.monotonic()
        logger.info("=" * 60)
        logger.info(f"【开始执行快手极速版刷视频任务】设备: {self.serial}")
        logger.info("=" * 60)

        # 步骤 1: 定位至任务中心页面
        self.ensure_task_center()

        cycles = 0
        total_watched = 0
        last_progress: TaskProgress | None = None

        while cycles < self.max_cycles:
            cycles += 1
            logger.info(f"\n--- [第 {cycles}/{self.max_cycles} 轮任务循环] ---")

            # 步骤 2 & 3: 指定区域 OCR 识别与解析
            progress = self.ocr_task_area(retry_limit=3)
            if not progress:
                logger.error(f"[{self.serial}] 无法识别或解析任务中心指定区域数据，任务异常退出")
                return {
                    "status": "failed",
                    "reason": "ocr_parse_failed",
                    "cycles": cycles,
                    "total_watched": total_watched,
                    "elapsed_seconds": round(time.monotonic() - started_at, 2),
                }

            last_progress = progress

            # 步骤 4: 检查任务是否已达成完成条件
            if progress.is_completed:
                logger.info("=" * 60)
                logger.info(f"【刷视频任务已达标完成】进度: {progress.current}/{progress.target}, 已领金币: {progress.claimed_coins}")
                logger.info("=" * 60)
                return {
                    "status": "completed",
                    "reason": "target_reached",
                    "claimed_coins": progress.claimed_coins,
                    "current": progress.current,
                    "target": progress.target,
                    "cycles": cycles,
                    "total_watched": total_watched,
                    "elapsed_seconds": round(time.monotonic() - started_at, 2),
                }

            # 步骤 5: 触发“去看视频”
            if not self.trigger_watch_video(progress):
                logger.warning(f"[{self.serial}] 点击『去看视频』未成功，尝试直接跳转回任务中心重试")
                time.sleep(2.0)
                continue

            # 步骤 6: 进入短视频流，拟人化看视频批次
            batch_result = self.run_video_feed_batch()
            total_watched += batch_result.get("watched_count", 0)

            # 步骤 7: 执行返回操作回到任务中心
            self.return_to_task_center()
            logger.info(f"[{self.serial}] 已返回任务中心，等待刷新后复核进度...")
            time.sleep(2.5)

        logger.warning(f"[{self.serial}] 达到最大循环轮次 {self.max_cycles}，任务退出")
        return {
            "status": "max_cycles_reached",
            "last_progress": last_progress.to_dict() if last_progress else None,
            "cycles": cycles,
            "total_watched": total_watched,
            "elapsed_seconds": round(time.monotonic() - started_at, 2),
        }


def run_task(
    serial: str | None = None,
    base_url: str | None = None,
    **kwargs: Any,
) -> dict[str, Any]:
    """供 main.py 或外部调用的统一函数入口"""
    base = (base_url or get_base_url()).rstrip("/")
    resolved_serial = resolve_device_serial(serial, base)

    task = BrushVideoTask(
        serial=resolved_serial,
        base_url=base,
        bbox=kwargs.get("bbox"),
        batch_video_count=kwargs.get("batch_count", 5),
        min_stay=kwargs.get("min_stay", 5.0),
        max_stay=kwargs.get("max_stay", 15.0),
        max_cycles=kwargs.get("max_cycles", 20),
    )
    return task.run()


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--serial", help="设备序列号（若只有1台设备可省略自动绑定）")
    parser.add_argument("--base-url", default=None, help="MyWebScrcpy 服务根地址")
    parser.add_argument("--batch-count", type=int, default=5, help="单轮看视频数量 (默认 5 条)")
    parser.add_argument("--min-stay", type=float, default=5.0, help="单视频最短停留秒数 (默认 5.0)")
    parser.add_argument("--max-stay", type=float, default=15.0, help="单视频最长停留秒数 (默认 15.0)")
    parser.add_argument("--max-cycles", type=int, default=20, help="最大轮次上限 (默认 20 轮)")
    parser.add_argument("--json", action="store_true", help="以 JSON 格式输出结果")
    args = parser.parse_args()

    try:
        result = run_task(
            serial=args.serial,
            base_url=args.base_url,
            batch_count=args.batch_count,
            min_stay=args.min_stay,
            max_stay=args.max_stay,
            max_cycles=args.max_cycles,
        )
        if args.json:
            print(json.dumps(result, ensure_ascii=False, indent=2))
        else:
            print(f"任务执行结果: {result.get('status')} (耗时: {result.get('elapsed_seconds')}s)")
        return 0 if result.get("status") == "completed" else 1
    except Exception as e:
        if args.json:
            print(json.dumps({"error": str(e), "status": "failed"}, ensure_ascii=False))
        else:
            logger.error(f"执行异常: {e}")
        return 1


if __name__ == "__main__":
    if len(sys.argv) <= 1:
        sys.argv = ["brush_video_task.py", "--serial", "10.0.0.30:5555"]
    raise SystemExit(main())
