#!/usr/bin/env python3
"""针对设备 10.0.0.104:5555 的钉钉自动化打卡流程。

步骤：
1. 桌面通过 OCR 识别并点击“我的笔记”APP（拉起后台 8888 模拟定位服务）。
2. 调用 android-location-control 技能设置目标经纬度。
3. 校验逆地理编码是否确认进入目标城市（如“长沙市”）。
4. 发送 HOME 回到系统桌面，通过 OCR 识别并点击钉钉小组件“去打卡”。
5. 最多等待 60 秒，轮询“上班打卡”或“下班打卡”按钮。
6. 点击识别到的打卡按钮。
7. 抓取新画面 OCR 全文，判定并输出打卡结果证据。
"""

import argparse
import json
import logging
from pathlib import Path
import re
import subprocess
import sys
import time
import urllib.error
import urllib.parse
import urllib.request
import uuid

PROJECT_ROOT = Path(__file__).resolve().parents[4]
sys.path.insert(0, str(PROJECT_ROOT / "packages" / "python-sdk" / "src"))

from mywebscrcpy import Client

logging.basicConfig(level=logging.INFO, format="%(asctime)s [%(levelname)s] %(message)s")
logger = logging.getLogger("dingtalk_punch")

LOCATION_CLIENT_SCRIPT = PROJECT_ROOT / "skills" / "my-scrcpy-use" / "scripts" / "location_client.py"


def capture_screen_adb(serial: str, timeout: float = 15.0) -> bytes:
    """通过 ADB 抓取原始 PNG 截图二进制流。"""
    cmd = ["adb", "-s", serial, "exec-out", "screencap", "-p"]
    try:
        res = subprocess.run(cmd, capture_output=True, timeout=timeout, check=False)
    except subprocess.TimeoutExpired as exc:
        raise RuntimeError(f"ADB 截屏超时 ({timeout}s)") from exc
    if res.returncode != 0 or not res.stdout:
        raise RuntimeError(f"ADB 截屏失败 (exit={res.returncode}): {res.stderr.decode('utf-8', errors='ignore')}")
    return res.stdout


def ocr_detect(backend_url: str, image_bytes: bytes, keywords: list[str] | None = None, serial: str | None = None) -> dict:
    """调用 MyWebScrcpy 后端 /api/vision/ocr 接口执行文字检测。"""
    boundary = uuid.uuid4().hex
    parts = []

    header = (
        f"--{boundary}\r\n"
        f'Content-Disposition: form-data; name="file"; filename="screen.png"\r\n'
        f"Content-Type: image/png\r\n\r\n"
    ).encode("latin1")
    parts.append(header + image_bytes + b"\r\n")

    if keywords:
        kw_str = ",".join(keywords) if isinstance(keywords, list) else str(keywords)
        part_kw = (
            f"--{boundary}\r\n"
            f'Content-Disposition: form-data; name="keywords"\r\n\r\n'
            f"{kw_str}\r\n"
        ).encode("utf-8")
        parts.append(part_kw)

    if serial:
        part_serial = (
            f"--{boundary}\r\n"
            f'Content-Disposition: form-data; name="serial"\r\n\r\n'
            f"{serial}\r\n"
        ).encode("utf-8")
        parts.append(part_serial)

    parts.append(f"--{boundary}--\r\n".encode("latin1"))
    body = b"".join(parts)

    url = f"{backend_url.rstrip('/')}/api/vision/ocr"
    req = urllib.request.Request(
        url,
        data=body,
        headers={"Content-Type": f"multipart/form-data; boundary={boundary}"},
        method="POST",
    )
    try:
        with urllib.request.urlopen(req, timeout=20) as resp:
            data = resp.read()
            return json.loads(data.decode("utf-8"))
    except urllib.error.HTTPError as exc:
        err_msg = exc.read().decode("utf-8", errors="ignore")
        raise RuntimeError(f"OCR 接口调用失败 ({exc.code}): {err_msg}") from exc
    except urllib.error.URLError as exc:
        raise RuntimeError(f"无法连接 OCR 接口 {url}: {exc.reason}") from exc


def find_box_by_text(ocr_response: dict, target_text: str) -> tuple[float, float] | None:
    """在 OCR 响应的 boxes 中查找匹配文本，返回归一化中心坐标 (cx, cy)。"""
    boxes = ocr_response.get("boxes", [])
    for box in boxes:
        text = box.get("text", "")
        if target_text in text:
            x = float(box.get("x", 0))
            y = float(box.get("y", 0))
            w = float(box.get("w", 0))
            h = float(box.get("h", 0))
            if x > 1.0 or y > 1.0:
                # 如果返回的是绝对像素坐标，按 1080x2400 归一化
                x /= 1080.0
                y /= 2400.0
                w /= 1080.0
                h /= 2400.0
            return (x + w / 2.0, y + h / 2.0)
    return None


def check_city_in_address(address_data: dict, expected_city: str) -> bool:
    """检查 address 响应数据中是否包含预期城市名。"""
    if not address_data:
        return False
    data = address_data.get("data", {})
    address_str = ""
    if isinstance(data, dict):
        address_str = data.get("address", "") or json.dumps(data, ensure_ascii=False)
    elif isinstance(data, str):
        address_str = data
    return expected_city in address_str


def is_in_city_bbox(lat: float, lng: float, city: str = "长沙") -> bool:
    """基于经纬度地理围栏粗核验城市归属（如长沙市范围: 纬度 27.8~28.7, 经度 111.8~114.3）。"""
    if city == "长沙":
        return 27.8 <= lat <= 28.7 and 111.8 <= lng <= 114.3
    return True


def parse_punch_result(ocr_response: dict) -> dict:
    """分析打卡后页面的 OCR 文本，判定是否打卡成功。"""
    full_text = ocr_response.get("text", "")
    boxes = ocr_response.get("boxes", [])
    box_texts = [b.get("text", "") for b in boxes]
    combined = " ".join([full_text] + box_texts)

    success_keywords = ["打卡成功", "极速打卡成功", "已打卡", "上班打卡成功", "下班打卡成功"]
    failure_keywords = ["打卡失败", "不在考勤范围", "需要人脸识别", "无法打卡", "异常"]

    matched_success = [k for k in success_keywords if k in combined]
    matched_failure = [k for k in failure_keywords if k in combined]

    time_match = re.search(r"(\d{1,2}:\d{2}(?::\d{2})?)", combined)
    punch_time = time_match.group(1) if time_match else None

    if matched_success:
        status = "succeeded"
        summary = f"打卡成功: {', '.join(matched_success)}" + (f" (时间 {punch_time})" if punch_time else "")
    elif matched_failure:
        status = "failed"
        summary = f"打卡未通过: {', '.join(matched_failure)}"
    else:
        status = "unknown"
        summary = "未明确检测到打卡成功或失败关键字，请人工核查截图"

    return {
        "status": status,
        "summary": summary,
        "matched_success": matched_success,
        "matched_failure": matched_failure,
        "punch_time": punch_time,
        "raw_text": full_text,
    }


class DingTalkPunchRunner:
    def __init__(self, backend_url: str, serial: str, location_url: str, lat: float, lng: float, coord_type: str = "wgs84", expected_city: str = "长沙"):
        self.backend_url = backend_url.rstrip("/")
        self.serial = serial
        self.location_url = location_url.rstrip("/")
        self.lat = lat
        self.lng = lng
        self.coord_type = coord_type
        self.expected_city = expected_city
        self.client = Client(self.backend_url)
        self.device = self.client.device(self.serial)

    def step1_open_notes_app(self) -> None:
        """步骤 1：桌面识别并点击“我的笔记”APP。"""
        logger.info("步骤 1: 正在桌面寻找并打开“我的笔记”APP...")
        png_bytes = capture_screen_adb(self.serial)
        ocr_res = ocr_detect(self.backend_url, png_bytes, keywords=["我的笔记"], serial=self.serial)
        target = find_box_by_text(ocr_res, "我的笔记")

        if target:
            logger.info("OCR 成功定位“我的笔记”，坐标: (%.1f, %.1f)，执行点击", target[0], target[1])
            self.device.tap(target)
        else:
            logger.warning("OCR 未找到“我的笔记”，回退使用已知桌面坐标归一化点击")
            self.device.tap((909.0 / 1080.0, 527.0 / 2400.0))

        logger.info("等待 8888 模拟定位服务端口就绪...")
        import socket
        parsed = urllib.parse.urlparse(self.location_url)
        host = parsed.hostname or "10.0.0.104"
        port = parsed.port or 8888
        deadline = time.monotonic() + 15.0
        while time.monotonic() < deadline:
            try:
                with socket.create_connection((host, port), timeout=1.0):
                    probe = subprocess.run(
                        [sys.executable, str(LOCATION_CLIENT_SCRIPT), "--base-url", self.location_url, "docs"],
                        capture_output=True,
                        text=True,
                        timeout=3,
                        check=False,
                    )
                    if probe.returncode == 0 and '"ok": true' in probe.stdout:
                        logger.info("8888 定位服务已就绪！")
                        return
            except Exception as e:
                logger.debug("8888 端口探测中: %s", e)
            time.sleep(1.0)
        raise RuntimeError(f"打开“我的笔记”后，8888 端口未在限定时间内响应文档: {self.location_url}")

    def step2_and_3_set_and_verify_location(self) -> None:
        """步骤 2 & 3：修改模拟定位并验证所在城市。"""
        logger.info("步骤 2: 调用 android-location-control 设置经纬度: %s, %s (坐标系: %s)", self.lat, self.lng, self.coord_type)
        cmd_set = [
            sys.executable,
            str(LOCATION_CLIENT_SCRIPT),
            "--base-url",
            self.location_url,
            "set",
            "--lat",
            str(self.lat),
            "--lng",
            str(self.lng),
            "--coord-type",
            self.coord_type,
        ]
        res_set = subprocess.run(cmd_set, capture_output=True, text=True, timeout=10, check=False)
        if res_set.returncode != 0:
            raise RuntimeError(f"设置定位接口执行失败: {res_set.stderr or res_set.stdout}")
        logger.info("定位已成功下发至设备")

        logger.info("步骤 3: 逆地理编码确认是否处于城市: %s...", self.expected_city)
        cmd_addr = [
            sys.executable,
            str(LOCATION_CLIENT_SCRIPT),
            "--base-url",
            self.location_url,
            "address",
        ]
        res_addr = subprocess.run(cmd_addr, capture_output=True, text=True, timeout=10, check=False)
        if res_addr.returncode != 0:
            out = res_addr.stderr + res_addr.stdout
            if "APP 服务被禁用" in out or "逆地址解析失败" in out:
                logger.warning("设备端逆地理服务返回禁用，转用经纬度地理围栏校验...")
                if not is_in_city_bbox(self.lat, self.lng, self.expected_city):
                    raise RuntimeError(f"经纬度 ({self.lat}, {self.lng}) 超出目标城市 {self.expected_city} 地理范围")
                logger.info("地理围栏校验通过: 坐标位于 %s 范围内", self.expected_city)
            else:
                raise RuntimeError(f"查询地址信息失败: {out}")
        else:
            try:
                addr_json = json.loads(res_addr.stdout)
                addr_obj = addr_json.get("address", {})
                if not check_city_in_address(addr_obj, self.expected_city):
                    raise RuntimeError(f"逆地理编码未包含目标城市 '{self.expected_city}': {res_addr.stdout}")
                logger.info("城市确认通过！逆地理编码包含 '%s'", self.expected_city)
            except json.JSONDecodeError as exc:
                raise RuntimeError(f"解析 address 返回 JSON 失败: {res_addr.stdout}") from exc

    def step4_return_home_and_tap_clock_in(self) -> None:
        """步骤 4：回到系统桌面并通过 OCR 点击“去打卡”。"""
        logger.info("步骤 4: 发送 KEYCODE_HOME 返回系统桌面...")
        self.device.key(3)  # KEYCODE_HOME
        time.sleep(1.5)

        logger.info("抓取桌面画面并通过 OCR 寻找“去打卡”按钮...")
        png_bytes = capture_screen_adb(self.serial)
        ocr_res = ocr_detect(self.backend_url, png_bytes, keywords=["去打卡", "考勤打卡"], serial=self.serial)
        target = find_box_by_text(ocr_res, "去打卡")
        if not target:
            target = find_box_by_text(ocr_res, "考勤打卡")

        if target:
            logger.info("OCR 成功定位“去打卡”，坐标: (%.1f, %.1f)，执行点击", target[0], target[1])
            self.device.tap(target)
        else:
            logger.warning("未通过 OCR 识别到“去打卡”，尝试使用桌面小组件实测坐标归一化点击")
            self.device.tap((330.0 / 1080.0, 1678.0 / 2400.0))

    def step5_and_6_wait_and_punch(self, timeout: float = 60.0) -> str:
        """步骤 5 & 6：最多等 1 分钟，轮询“上班打卡”或“下班打卡”并执行点击。"""
        logger.info("步骤 5: 轮询等待“上班打卡”或“下班打卡”按钮出现 (上限 %d 秒)...", int(timeout))
        deadline = time.monotonic() + timeout
        abnormal_signs = ["不在考勤范围", "外勤打卡", "无法获取位置", "人脸识别", "无法打卡"]

        while time.monotonic() < deadline:
            png_bytes = capture_screen_adb(self.serial)
            ocr_res = ocr_detect(self.backend_url, png_bytes, keywords=["上班打卡", "下班打卡"] + abnormal_signs, serial=self.serial)

            target_shangban = find_box_by_text(ocr_res, "上班打卡")
            target_xiaban = find_box_by_text(ocr_res, "下班打卡")

            if target_shangban:
                logger.info("检测到【上班打卡】按钮，坐标: (%.1f, %.1f)，开始执行点击 (步骤 6)", target_shangban[0], target_shangban[1])
                self.device.tap(target_shangban)
                return "上班打卡"
            elif target_xiaban:
                logger.info("检测到【下班打卡】按钮，坐标: (%.1f, %.1f)，开始执行点击 (步骤 6)", target_xiaban[0], target_xiaban[1])
                self.device.tap(target_xiaban)
                return "下班打卡"

            for sign in abnormal_signs:
                if find_box_by_text(ocr_res, sign):
                    raise RuntimeError(f"检测到异常考勤提示: '{sign}'，已终止自动打卡以防误操作")

            time.sleep(2.0)

        raise RuntimeError(f"在 {timeout} 秒内未检测到上班打卡或下班打卡按钮，页面可能卡顿或未加载完成")

    def step7_verify_punch_result(self) -> dict:
        """步骤 7：获取最新画面 OCR 全文，判定打卡最终状态。"""
        logger.info("步骤 7: 等待结果呈现，抓取最新屏幕画面进行 OCR 结果分析...")
        time.sleep(2.0)
        png_bytes = capture_screen_adb(self.serial)
        ocr_res = ocr_detect(self.backend_url, png_bytes, serial=self.serial)
        result = parse_punch_result(ocr_res)
        logger.info("打卡结果分析结论: [%s] %s", result["status"], result["summary"])
        return result

    def run(self) -> dict:
        """按序运行完整打卡流程。"""
        self.step1_open_notes_app()
        self.step2_and_3_set_and_verify_location()
        self.step4_return_home_and_tap_clock_in()
        action = self.step5_and_6_wait_and_punch(timeout=60.0)
        result = self.step7_verify_punch_result()
        result["action"] = action
        return result


def main():
    parser = argparse.ArgumentParser(description="钉钉自动化打卡执行脚本")
    parser.add_argument("--backend-url", default="http://127.0.0.1:8080", help="MyWebScrcpy 后端地址")
    parser.add_argument("--serial", default="10.0.0.104:5555", help="目标手机 ADB serial")
    parser.add_argument("--location-url", default="http://10.0.0.104:8888", help="模拟定位服务根地址")
    parser.add_argument("--lat", type=float, default=28.15439276212, help="纬度")
    parser.add_argument("--lng", type=float, default=112.93746766578, help="经度")
    parser.add_argument("--coord-type", choices=["wgs84", "gcj02", "bd09ll"], default="wgs84", help="坐标系")
    parser.add_argument("--expected-city", default="长沙", help="校验逆地理编码包含的城市")
    args = parser.parse_args()

    runner = DingTalkPunchRunner(
        backend_url=args.backend_url,
        serial=args.serial,
        location_url=args.location_url,
        lat=args.lat,
        lng=args.lng,
        coord_type=args.coord_type,
        expected_city=args.expected_city,
    )
    res = runner.run()
    print(json.dumps(res, ensure_ascii=False, indent=2))
    if res.get("status") == "failed":
        sys.exit(1)


if __name__ == "__main__":
    main()
