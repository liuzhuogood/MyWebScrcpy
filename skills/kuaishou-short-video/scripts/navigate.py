#!/usr/bin/env python3
"""快手极速版：页面跳转与导航脚本

实现能力：
1. 点击“返回”键及尝试点击“坚持退出”弹窗，等待 2 秒，OCR 判断底部是否出现 “去赚钱”、“首页”、“朋友”、“我” 等导航项，
   若未出现则重复最多 5 次判断（每轮点击返回/坚持退出后等待 2 秒并 OCR 验证）。
2. 跳转到任务中心：在确认处于含主底栏的界面后，点击“去赚钱”对应的坐标跳转至任务中心。
"""

import argparse
import json
import logging
import os
from pathlib import Path
import sys
import time
from typing import Any
from urllib.error import URLError
from urllib.request import Request, urlopen

# 日志输出配置
logging.basicConfig(
    level=logging.INFO,
    format="[%(asctime)s] [%(levelname)s] %(message)s",
    datefmt="%H:%M:%S",
)
logger = logging.getLogger("kuaishou_navigate")

# 底部 Tab 默认关键字
BOTTOM_BAR_KEYWORDS = ["去赚钱", "首页", "朋友", "我"]

# 退出挽留弹窗关键字
EXIT_DIALOG_KEYWORDS = ["坚持退出"]


def get_base_url() -> str:
    """获取服务端根地址"""
    url = (
            os.environ.get("MYWEBSCRCPY_BASE_URL")
            or os.environ.get("MYWEBSCRCPY_API_URL")
            or "http://127.0.0.1:8080"
    )
    return url.rstrip("/")


def resolve_device_serial(arg_serial: str | None = None, base_url: str | None = None) -> str:
    """解析设备 serial：参数 > 环境变量 > 在线设备列表"""
    if arg_serial and arg_serial.strip():
        return arg_serial.strip()

    env_serial = os.environ.get("MYWEBSCRCPY_SERIAL") or os.environ.get("SERIAL")
    if env_serial and env_serial.strip():
        return env_serial.strip()

    base = (base_url or get_base_url()).rstrip("/")
    try:
        req = Request(f"{base}/api/devices")
        with urlopen(req, timeout=5) as resp:
            data = json.loads(resp.read().decode("utf-8"))
            devices = [d["serial"] for d in data if d.get("state") == "device"]
            if len(devices) == 1:
                return devices[0]
            elif len(devices) > 1:
                raise ValueError(
                    f"检测到多台在线设备 ({', '.join(devices)})，请显式指定 serial"
                )
            else:
                raise ValueError("未检测到在线设备")
    except Exception as e:
        if isinstance(e, ValueError):
            raise
        raise ValueError(f"获取在线设备列表失败: {e}") from e


class KuaishouNavigator:
    """快手页面导航控制器"""

    def __init__(self, serial: str, base_url: str | None = None):
        self.serial = serial
        self.base_url = (base_url or get_base_url()).rstrip("/")

    def _post_json(self, path: str, payload: dict[str, Any], timeout: float = 10.0) -> dict[str, Any]:
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

    def press_back(self) -> bool:
        """发送系统返回键 (KEYCODE_BACK, 4)"""
        logger.info(f"[{self.serial}] 执行按键: KEYCODE_BACK (返回)")
        payload = {
            "serial": self.serial,
            "keycode": 4,  # KEYCODE_BACK
        }
        res = self._post_json("/api/v1/actions/send-key", payload)
        # 兼容 executed=True 或 status="executed"
        return res.get("executed") is True or res.get("status") == "executed"

    def tap(self, x: float, y: float) -> bool:
        """点击归一化坐标 (0.0 ~ 1.0)"""
        logger.info(f"[{self.serial}] 执行点击: ({x:.4f}, {y:.4f})")
        payload = {
            "serial": self.serial,
            "x": x,
            "y": y,
        }
        res = self._post_json("/api/v1/actions/tap", payload)
        return res.get("executed") is True or res.get("status") == "executed"

    def tap_text(self, text_or_keywords: str | list[str], timeout: float = 15.0) -> bool:
        """通过 OCR 检查屏幕中是否存在指定文字，若命中则点击其中心位置

        参数:
            text_or_keywords: 目标文字或文字列表（如 '坚持退出' 或 ['坚持退出']）
            timeout: OCR 请求超时时间（秒）
        返回:
            bool: 是否命中并成功触发点击
        """
        keywords = (
            [text_or_keywords] if isinstance(text_or_keywords, str) else list(text_or_keywords)
        )
        logger.info(f"[{self.serial}] 尝试检测并点击文字: {keywords}")
        payload = {
            "serial": self.serial,
            "kind": "text",
            "condition": {
                "keywords": keywords,
            },
        }

        try:
            res = self._post_json("/api/v1/vision/check", payload, timeout=timeout)
        except Exception as e:
            logger.warning(f"[{self.serial}] OCR 检测文字 {keywords} 异常: {e}")
            return False

        result_data = res.get("result") or {}
        objects = result_data.get("objects", [])

        matched_objects = []
        for obj in objects:
            lbl = obj.get("label", "")
            if any(k in lbl for k in keywords):
                matched_objects.append(obj)

        if not matched_objects:
            logger.info(f"[{self.serial}] 未发现匹配文字: {keywords}")
            return False

        # 按置信度排序优先点击可信度最高的文本块
        matched_objects.sort(key=lambda o: float(o.get("confidence", 0.0)), reverse=True)
        best = matched_objects[0]
        bx = float(best.get("x", 0.0))
        by = float(best.get("y", 0.0))
        bw = float(best.get("w", 0.0))
        bh = float(best.get("h", 0.0))
        click_x = bx + bw / 2 if bw > 0 else bx
        click_y = by + bh / 2 if bh > 0 else by

        logger.info(
            f"[{self.serial}] 发现文字『{best.get('label')}』(x={click_x:.4f}, y={click_y:.4f}, 置信度={best.get('confidence')})，执行点击"
        )
        return self.tap(click_x, click_y)

    def check_bottom_bar(self, required_keywords: list[str] | None = None) -> dict[str, Any]:
        """通过 OCR 检查底部区域是否出现指定主导航文字

        参数:
            required_keywords: 检查的目标关键字列表，默认为 ["去赚钱", "首页", "朋友", "我"]
        返回:
            dict: {
                "matched": bool,
                "found_keywords": list[str],
                "objects": list[dict], # 命中的目标及其坐标
                "raw_text": str
            }
        """
        keywords = required_keywords or BOTTOM_BAR_KEYWORDS
        # 底部导航通常在屏幕底部区域 (y >= 0.8)
        payload = {
            "serial": self.serial,
            "kind": "text",
            "condition": {
                "keywords": keywords,
            },
        }

        try:
            res = self._post_json("/api/v1/vision/check", payload, timeout=15.0)
        except Exception as e:
            logger.warning(f"[{self.serial}] OCR 检查异常: {e}")
            return {"matched": False, "found_keywords": [], "objects": [], "raw_text": ""}

        result_data = res.get("result") or {}
        raw_text = result_data.get("text", "")
        objects = result_data.get("objects", [])

        # 筛选落在底部区域 (y 坐标通常在 0.75 ~ 1.05) 的匹配项
        bottom_objects = []
        found_keywords = set()

        for obj in objects:
            lbl = obj.get("label", "")
            y = float(obj.get("y", 0.0))
            # 兼容原屏归一化以及略微偏移的情况（例如底栏 y 大约 0.85 ~ 1.0）
            # 同时排除掉顶部/中间恰巧有同名文字的误判
            if y >= 0.70 or any(k in lbl for k in keywords):
                bottom_objects.append(obj)
                for k in keywords:
                    if k in lbl:
                        found_keywords.add(k)

        # 只要命中了底部导航的关键项即可视为主底栏存在
        is_matched = len(found_keywords) > 0

        return {
            "matched": is_matched,
            "found_keywords": list(found_keywords),
            "objects": bottom_objects,
            "raw_text": raw_text,
        }

    def back_until_bottom_bar(
            self,
            max_retries: int = 5,
            wait_seconds: float = 2.0,
            required_keywords: list[str] | None = None,
            exit_keywords: list[str] | None = None,
    ) -> dict[str, Any]:
        """返回直到出现底部导航栏

        严格遵循流程：
        1. 检查当前是否已出现“坚持退出”弹窗，若有则优先点击退出并验证底栏；
        2. 每轮尝试点击“返回”键，等待 2 秒；
        3. OCR 判断底部是否出现 “去赚钱”、“首页”、“朋友”、“我”；
        4. 若未出现底栏，检查是否出现了“坚持退出”弹窗文字；若出现则尝试点击“坚持退出”并再次复核底栏；
        5. 若仍未出现则重复最多 max_retries 次判断。

        返回:
            dict: 包含 success, attempts, found_keywords, bar_info 等
        """
        keywords = required_keywords or BOTTOM_BAR_KEYWORDS
        dialog_keywords = exit_keywords or EXIT_DIALOG_KEYWORDS
        logger.info(
            f"[{self.serial}] 开始导航回主界面底栏 (最多尝试 {max_retries} 次，每次等待 {wait_seconds}s，退出弹窗关键字: {dialog_keywords})"
        )

        # 预先检查：若当前屏幕已经处于“坚持退出”挽留弹窗，优先点击消除
        if self.tap_text(dialog_keywords):
            logger.info(f"[{self.serial}] 当前画面已存在退出弹窗，已尝试点击，等待 {wait_seconds} 秒...")
            time.sleep(wait_seconds)
            initial_check = self.check_bottom_bar(keywords)
            if initial_check.get("matched"):
                found = initial_check.get("found_keywords", [])
                logger.info(f"[{self.serial}] 处理残留弹窗后成功回到主底栏 (命中: {', '.join(found)})")
                return {
                    "success": True,
                    "attempts": 0,
                    "found_keywords": found,
                    "bar_info": initial_check,
                    "dismissed_exit_dialog": True,
                }

        bar_info: dict[str, Any] = {}
        for attempt in range(1, max_retries + 1):
            logger.info(f"[{self.serial}] 第 {attempt}/{max_retries} 次尝试：点击返回键...")
            self.press_back()

            logger.info(f"[{self.serial}] 等待 {wait_seconds} 秒等待页面渲染...")
            time.sleep(wait_seconds)

            logger.info(f"[{self.serial}] 执行 OCR 检测底部主导航栏...")
            bar_info = self.check_bottom_bar(keywords)
            found = bar_info.get("found_keywords", [])
            logger.info(f"[{self.serial}] 检查结果: 命中关键字={found}")

            if bar_info.get("matched"):
                logger.info(
                    f"[{self.serial}] 成功检测到底部导航栏 (第 {attempt} 次尝试命中: {', '.join(found)})"
                )
                return {
                    "success": True,
                    "attempts": attempt,
                    "found_keywords": found,
                    "bar_info": bar_info,
                }

            # 未检测到底部导航栏，尝试识别并点击“坚持退出”弹窗文字
            logger.info(f"[{self.serial}] 未检测到底栏，检查是否出现『坚持退出』弹窗...")
            if self.tap_text(dialog_keywords):
                logger.info(f"[{self.serial}] 检测并点击了『坚持退出』，等待 {wait_seconds} 秒让页面响应...")
                time.sleep(wait_seconds)

                # 复核底栏
                logger.info(f"[{self.serial}] 点击『坚持退出』后复核 OCR 检测主底栏...")
                bar_info = self.check_bottom_bar(keywords)
                found = bar_info.get("found_keywords", [])
                if bar_info.get("matched"):
                    logger.info(
                        f"[{self.serial}] 点击『坚持退出』后成功到达主底栏 (第 {attempt} 次尝试命中: {', '.join(found)})"
                    )
                    return {
                        "success": True,
                        "attempts": attempt,
                        "found_keywords": found,
                        "bar_info": bar_info,
                        "dismissed_exit_dialog": True,
                    }

        logger.warning(f"[{self.serial}] 达到最大重试次数 {max_retries}，仍未检测到底部导航栏")
        return {
            "success": False,
            "attempts": max_retries,
            "found_keywords": [],
            "bar_info": bar_info,
        }

    def navigate_to_task_center(
            self,
            max_retries: int = 5,
            wait_seconds: float = 2.0,
            click_wait_seconds: float = 2.0,
            exit_keywords: list[str] | None = None,
    ) -> dict[str, Any]:
        """跳转到任务中心

        流程:
        1. 点击“返回”键或“坚持退出”，等待 2 秒，OCR 判断底部是否出现 “去赚钱”、“首页”、“朋友”、“我”，若没有重复 5 次判断
        2. 点击 “去赚钱” 跳转到任务中心
        3. 等待页面加载并可选复核任务中心状态
        """
        logger.info(f"[{self.serial}] 开始跳转至任务中心流程...")

        # 步骤 1: 点击“返回”键 /“坚持退出”，等待 2 秒，OCR 判断底部是否出现“去赚钱”、“首页”、“朋友”、“我”，若没有重复 5 次判断
        back_res = self.back_until_bottom_bar(
            max_retries=max_retries,
            wait_seconds=wait_seconds,
            required_keywords=BOTTOM_BAR_KEYWORDS,
            exit_keywords=exit_keywords,
        )
        if not back_res.get("success"):
            return {
                "success": False,
                "error": f"未能退回含主底栏的页面，重试 {max_retries} 次失败",
                "back_result": back_res,
            }

        bar_info = back_res.get("bar_info", {})
        target_objects = [
            obj for obj in bar_info.get("objects", []) if "去赚钱" in obj.get("label", "")
        ]

        # 查找“去赚钱”对应的坐标
        click_x: float | None = None
        click_y: float | None = None

        if target_objects:
            # 取置信度最高或匹配到的『去赚钱』中心点
            best_obj = target_objects[0]
            bx = float(best_obj.get("x", 0.0))
            by = float(best_obj.get("y", 0.0))
            bw = float(best_obj.get("w", 0.0))
            bh = float(best_obj.get("h", 0.0))
            # 若已有中心点或包围盒，计算中心坐标
            click_x = bx + bw / 2 if bw > 0 else bx
            click_y = by + bh / 2 if bh > 0 else by
            logger.info(
                f"[{self.serial}] 识别到『去赚钱』文字坐标: x={click_x:.4f}, y={click_y:.4f} (label={best_obj.get('label')})"
            )
        else:
            # 若底栏识别到了“首页/朋友/我”但未能单独拆分“去赚钱”，使用快手极速版主底栏第4个Tab标准位置
            logger.warning(
                f"[{self.serial}] OCR 未单独框选出『去赚钱』，采用快手底栏第四项推荐坐标 (x=0.70, y=0.95)"
            )
            click_x = 0.70
            click_y = 0.95

        # 步骤 2: 点击 “去赚钱” 就是跳任务中心
        logger.info(f"[{self.serial}] 点击『去赚钱』(x={click_x:.4f}, y={click_y:.4f}) 跳转任务中心...")
        tapped = self.tap(click_x, click_y)
        if not tapped:
            return {
                "success": False,
                "error": "发送点击『去赚钱』指令失败",
                "target_point": {"x": click_x, "y": click_y},
            }

        # 等待页面加载
        if click_wait_seconds > 0:
            logger.info(f"[{self.serial}] 等待 {click_wait_seconds} 秒让任务中心加载...")
            time.sleep(click_wait_seconds)

        return {
            "success": True,
            "action": "tap_earn_money",
            "target_point": {"x": click_x, "y": click_y},
            "back_attempts": back_res.get("attempts", 0),
        }


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--serial", help="设备序列号（若只有1台设备可省略自动绑定）")
    parser.add_argument("--base-url", default=None, help="MyWebScrcpy 服务根地址")
    parser.add_argument(
        "--action",
        choices=["back-to-bar", "task-center"],
        default="task-center",
        help="执行操作: back-to-bar (回退到底栏) / task-center (跳任务中心，默认)",
    )
    parser.add_argument(
        "--retries",
        type=int,
        default=5,
        help="最多判断/重试次数 (默认 5 次)",
    )
    parser.add_argument(
        "--wait",
        type=float,
        default=2.0,
        help="点击返回键后等待渲染秒数 (默认 2.0s)",
    )
    parser.add_argument(
        "--exit-keyword",
        action="append",
        dest="exit_keywords",
        help="退出挽留弹窗文字关键字（可指定多次，默认包含'坚持退出'）",
    )
    parser.add_argument("--json", action="store_true", help="以结构化 JSON 输出结果")
    args = parser.parse_args()

    base_url = args.base_url or get_base_url()
    try:
        serial = resolve_device_serial(args.serial, base_url)
    except ValueError as e:
        if args.json:
            print(json.dumps({"error": str(e), "success": False}, ensure_ascii=False))
        else:
            logger.error(str(e))
        return 1

    navigator = KuaishouNavigator(serial=serial, base_url=base_url)

    if args.action == "back-to-bar":
        res = navigator.back_until_bottom_bar(
            max_retries=args.retries,
            wait_seconds=args.wait,
            exit_keywords=args.exit_keywords,
        )
    else:  # task-center
        res = navigator.navigate_to_task_center(
            max_retries=args.retries,
            wait_seconds=args.wait,
            exit_keywords=args.exit_keywords,
        )

    if args.json:
        print(json.dumps(res, ensure_ascii=False, indent=2))
    else:
        if res.get("success"):
            logger.info(f"执行成功: {res}")
        else:
            logger.error(f"执行失败: {res}")

    return 0 if res.get("success") else 1


if __name__ == "__main__":
    if len(sys.argv) <= 1:
        sys.argv = ["navigate.py", "--serial", "10.0.0.30:5555", "--action", "task-center"]
    raise SystemExit(main())
