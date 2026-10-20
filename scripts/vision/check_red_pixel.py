#!/usr/bin/env python3
"""单次取色并判断是否接近设备的亮红色样本；使用 curl 请求 HTTPS。"""

import argparse
import json
import subprocess
import sys


DEVICE_POINTS = {
    "10.0.0.114:5555": (60, 245, (255, 27, 72)),
    "10.0.0.30:5555": (54, 210, (255, 46, 66)),
}


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("serial", nargs="?", choices=DEVICE_POINTS, help="设备序列号；省略时检查所有已配置设备")
    parser.add_argument("--base-url", default="https://10.0.0.6:8080")
    parser.add_argument("--tolerance", type=int, default=40, help="RGB 各通道允许偏差，默认 40")
    args = parser.parse_args()
    if not 0 <= args.tolerance <= 255:
        parser.error("--tolerance 必须在 0 到 255 之间")

    serials = [args.serial] if args.serial is not None else list(DEVICE_POINTS)
    exit_code = 0
    for serial in serials:
        if check_device(serial, args.base_url, args.tolerance) != 0:
            exit_code = 1
    return exit_code


def check_device(serial: str, base_url: str, tolerance: int) -> int:
    x, y, reference = DEVICE_POINTS[serial]
    command = [
        "curl", "-k", "-sS", "--fail-with-body",
        "--connect-timeout", "5", "--max-time", "15", "--get",
        f"{base_url.rstrip('/')}/api/v1/vision/pixel",
    ]
    for key, value in (("serial", serial), ("x", x), ("y", y), ("normalized", "false")):
        command.extend(["--data-urlencode", f"{key}={value}"])
    try:
        response = subprocess.run(command, capture_output=True, text=True, check=True, timeout=20)
        data = json.loads(response.stdout)
        if data["serial"] != serial or data["x"] != x or data["y"] != y:
            raise ValueError("响应设备或坐标与请求不一致")
        rgb = data["rgb"]
        if not isinstance(rgb, list) or len(rgb) != 3 or any(
            type(value) is not int or not 0 <= value <= 255 for value in rgb
        ):
            raise ValueError("响应 rgb 必须包含三个 0 到 255 的整数")
        is_red = all(abs(value - target) <= tolerance for value, target in zip(rgb, reference))
        print(json.dumps({
            "serial": serial, "x": x, "y": y,
            "rgb": rgb, "hex": data["hex"], "is_red": is_red,
        }, ensure_ascii=False))
        return 0
    except (OSError, subprocess.SubprocessError, ValueError, KeyError, TypeError) as exc:
        print(f"{serial} 取色失败：{exc}", file=sys.stderr)
        return 1


if __name__ == "__main__":
    sys.exit(main())
