#!/usr/bin/env python3
"""快手极速版任务入口。"""

import argparse
from dataclasses import asdict, dataclass
import json
import logging
import sys
import time
from typing import Any

from navigate import get_base_url, resolve_device_serial
from mywebscrcpy import Client

logging.basicConfig(level=logging.INFO, format="[%(asctime)s] [%(levelname)s] %(message)s", datefmt="%H:%M:%S")
logger = logging.getLogger("kuaishou_main")


@dataclass
class VideoStreamInfo:
    serial: str
    session_id: str
    width: int
    height: int
    first_frame_id: int
    timestamp_ms: int
    synced_to_vision_frame: bool

    def to_dict(self) -> dict[str, Any]:
        return asdict(self)


def start_task(
    serial: str | None = None,
    base_url: str | None = None,
    timeout: float = 15.0,
    sync_vision_frame: bool = True,
) -> dict[str, Any]:
    base = (base_url or get_base_url()).rstrip("/")
    serial = resolve_device_serial(serial, base)
    started_at = time.monotonic()

    with Client(base) as client:
        with client.startVideoStream(
            serial, timeout=timeout, sync_vision_frame=sync_vision_frame
        ) as stream:
            frame = stream.first_frame
            info = VideoStreamInfo(
                serial=serial,
                session_id=stream.session_id,
                width=frame.width,
                height=frame.height,
                first_frame_id=frame.frame_id,
                timestamp_ms=frame.timestamp,
                synced_to_vision_frame=stream.synced_to_vision_frame,
            )
            return {
                "status": "video_stream_ready",
                "serial": serial,
                "elapsed_seconds": round(time.monotonic() - started_at, 2),
                "step": "acquire_video_stream",
                "video_stream": info.to_dict(),
            }


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--serial", help="设备序列号（若只有1台在线设备或配置了环境变量可省略）")
    parser.add_argument("--base-url", default=None, help="服务端根地址")
    parser.add_argument("--timeout", type=float, default=15.0, help="获取首帧超时秒数")
    parser.add_argument("--no-sync-frame", action="store_true", help="不上传首帧至设备总状态缓存")
    parser.add_argument("--json", action="store_true", help="以 JSON 格式输出执行结果")
    args = parser.parse_args()

    try:
        result = start_task(
            serial=args.serial,
            base_url=args.base_url,
            timeout=args.timeout,
            sync_vision_frame=not args.no_sync_frame,
        )
        if args.json:
            print(json.dumps(result, ensure_ascii=False, indent=2))
        else:
            stream = result["video_stream"]
            print("=" * 60)
            print("【快手极速版任务启动成功】")
            print(f"目标设备: {result['serial']}")
            print(f"当前状态: {result['status']}")
            print(
                f"视频流会话: {stream['session_id']} | "
                f"分辨率: {stream['width']}x{stream['height']} | "
                f"初始帧ID: {stream['first_frame_id']}"
            )
            print(f"耗时: {result['elapsed_seconds']}s")
            print("=" * 60)
        return 0
    except Exception as error:
        if args.json:
            print(json.dumps({"error": str(error), "status": "failed"}, ensure_ascii=False))
        else:
            logger.error("任务启动失败: %s", error)
        return 1


if __name__ == "__main__":
    if len(sys.argv) <= 1:
        sys.argv = ["main.py", "--serial", "10.0.0.30:5555"]
    sys.exit(main())
