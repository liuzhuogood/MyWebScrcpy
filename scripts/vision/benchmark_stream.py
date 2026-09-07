#!/usr/bin/env python3
"""只读测量 Vision H.264 旁路及 Python 侧 JPEG 转码成本。

该脚本不发送识别结果或设备动作。它用于在真实设备上取得基线，不能代表
Go 端 JPEG 推流的最终性能；后者需要在服务端增加独立编码路径后再比较。
依赖：pip install websocket-client av opencv-python
"""

import argparse
import json
import resource
import sys
import time

try:
    import av
    import cv2
    import websocket
except ImportError as exc:
    print("缺少依赖，请先执行：python3 -m pip install -r scripts/vision/requirements.txt", file=sys.stderr)
    raise SystemExit(2) from exc

from demo_vision import annexb_nals, avcc_description_to_annexb


def main():
    parser = argparse.ArgumentParser(description="测量 Vision H.264 接收/解码和本地 JPEG 转码成本")
    parser.add_argument("--serial", default="10.0.0.30:5555")
    parser.add_argument("--url", default="ws://127.0.0.1:8080")
    parser.add_argument("--seconds", type=float, default=10)
    parser.add_argument("--max-fps", type=int, default=0, help="Vision delta 帧上限，0 表示不采样")
    parser.add_argument("--jpeg-quality", type=int, default=80)
    args = parser.parse_args()
    if args.seconds <= 0 or not 1 <= args.jpeg_quality <= 100:
        parser.error("seconds 必须大于 0，jpeg-quality 必须在 1~100")

    url = f"{args.url}/api/vision/stream?serial={args.serial.replace(':', '%3A')}"
    # 首次启动 scrcpy-server 包含 ADB push 和设备端握手，单独给足建立时间。
    ws = websocket.create_connection(url, timeout=30)
    decoder = None
    hello = json.loads(ws.recv())
    ws.send(json.dumps({"type": "hello", "max_fps": max(0, args.max_fps)}))
    # ack 可能在首个 frame 元数据之前，忽略直到收到它。
    while True:
        ack = json.loads(ws.recv())
        if ack.get("type") == "hello.ack":
            break
    ws.settimeout(1)

    started = time.perf_counter()
    rss_before = resource.getrusage(resource.RUSAGE_SELF).ru_maxrss
    received = decoded = jpeg_count = payload_bytes = 0
    decode_seconds = jpeg_seconds = 0.0
    frame_timestamp = 0
    transport_latencies = []
    try:
        while time.perf_counter() - started < args.seconds:
            try:
                message = ws.recv()
            except websocket.WebSocketTimeoutException:
                continue
            if isinstance(message, str):
                try:
                    info = json.loads(message)
                    if info.get("type") == "frame":
                        frame_timestamp = int(info.get("timestamp") or 0)
                except (TypeError, ValueError, json.JSONDecodeError):
                    pass
                continue
            if len(message) < 9:
                continue
            if frame_timestamp > 0:
                transport_latencies.append(max(0, time.time_ns() // 1_000_000 - frame_timestamp))
            received += 1
            payload = message[9:]
            payload_bytes += len(payload)
            if message[0] == 0:
                decoder = av.CodecContext.create("h264", "r")
                payload = avcc_description_to_annexb(payload)
            elif message[0] not in (1, 2):
                continue
            if decoder is None:
                continue
            packets = decoder.parse(annexb_nals(payload))
            for packet in packets:
                decode_started = time.perf_counter()
                try:
                    frames = decoder.decode(packet)
                except av.error.InvalidDataError:
                    continue
                decode_seconds += time.perf_counter() - decode_started
                for frame in frames:
                    decoded += 1
                    image = frame.to_ndarray(format="bgr24")
                    jpeg_started = time.perf_counter()
                    ok, _ = cv2.imencode(".jpg", image, [cv2.IMWRITE_JPEG_QUALITY, args.jpeg_quality])
                    jpeg_seconds += time.perf_counter() - jpeg_started
                    if ok:
                        jpeg_count += 1
    finally:
        ws.close()

    elapsed = max(time.perf_counter() - started, 1e-9)
    rss_after = resource.getrusage(resource.RUSAGE_SELF).ru_maxrss
    # macOS reports bytes, Linux reports KiB.
    rss_scale = 1 if sys.platform == "darwin" else 1024
    report = {
        "serial": args.serial,
        "session_id": hello.get("session_id"),
        "seconds": round(elapsed, 3),
        "received_packets": received,
        "decoded_frames": decoded,
        "jpeg_encoded_frames": jpeg_count,
        "payload_bytes": payload_bytes,
        "received_fps": round(received / elapsed, 2),
        "decoded_fps": round(decoded / elapsed, 2),
        "payload_mbps": round(payload_bytes * 8 / elapsed / 1_000_000, 3),
        "decode_ms_per_frame": round(decode_seconds * 1000 / max(decoded, 1), 3),
        "jpeg_ms_per_frame": round(jpeg_seconds * 1000 / max(jpeg_count, 1), 3),
        "peak_rss_delta_mb": round(max(0, rss_after - rss_before) * rss_scale / 1_000_000, 3),
        "transport_latency_avg_ms": round(sum(transport_latencies) / max(len(transport_latencies), 1), 3),
        "transport_latency_max_ms": max(transport_latencies, default=0),
        "jpeg_quality": args.jpeg_quality,
    }
    print(json.dumps(report, ensure_ascii=False, indent=2))


if __name__ == "__main__":
    main()
