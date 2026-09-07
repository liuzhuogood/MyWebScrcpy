#!/usr/bin/env python3
"""MyWebScrcpy Vision 最小演示：收帧、解码、显示，并回传一个演示框。

依赖：pip install websocket-client av opencv-python
用法：python3 scripts/vision/demo_vision.py --serial 10.0.0.30:5555

按 q 或 Esc 退出。脚本只发送 detection.result，不发送 action.request。
"""

import argparse
import json
import sys
import time

try:
    import av
    import cv2
    import websocket
except ImportError as exc:
    print("缺少依赖，请先执行：python3 -m pip install -r scripts/vision/requirements.txt", file=sys.stderr)
    raise SystemExit(2) from exc


def annexb_nals(data: bytes):
    """把 4 字节长度前缀的 H264 NAL 转成 Annex-B；已是 Annex-B 则原样返回。"""
    if data.startswith(b"\x00\x00\x00\x01") or data.startswith(b"\x00\x00\x01"):
        return data
    out = bytearray()
    pos = 0
    while pos + 4 <= len(data):
        size = int.from_bytes(data[pos:pos + 4], "big")
        pos += 4
        if size <= 0 or pos + size > len(data):
            return data
        out += b"\x00\x00\x00\x01" + data[pos:pos + size]
        pos += size
    return bytes(out) if pos == len(data) and out else data


def avcc_description_to_annexb(data: bytes) -> bytes:
    """提取 avcC 中的 SPS/PPS，供 PyAV 解码器接收。"""
    if len(data) < 7 or data[0] != 1:
        return annexb_nals(data)
    pos = 5
    sps_count = data[pos] & 0x1F
    pos += 1
    out = bytearray()
    for _ in range(sps_count):
        if pos + 2 > len(data):
            return data
        size = int.from_bytes(data[pos:pos + 2], "big")
        pos += 2
        if pos + size > len(data):
            return data
        out += b"\x00\x00\x00\x01" + data[pos:pos + size]
        pos += size
    if pos >= len(data):
        return bytes(out)
    pps_count = data[pos]
    pos += 1
    for _ in range(pps_count):
        if pos + 2 > len(data):
            break
        size = int.from_bytes(data[pos:pos + 2], "big")
        pos += 2
        if pos + size > len(data):
            break
        out += b"\x00\x00\x00\x01" + data[pos:pos + size]
        pos += size
    return bytes(out) or data


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--serial", default="10.0.0.30:5555")
    parser.add_argument("--url", default="ws://127.0.0.1:8080")
    parser.add_argument("--max-fps", type=int, default=5, help="Vision 订阅的 delta 帧上限，0 表示不采样")
    args = parser.parse_args()
    url = f"{args.url}/api/vision/stream?serial={args.serial.replace(':', '%3A')}"
    # 首次启动 scrcpy-server 需要 push jar 并完成设备端握手，网络 ADB
    # 可能超过普通帧读取超时；连接建立阶段单独给 30 秒。
    ws = websocket.create_connection(url, timeout=30)
    decoder = None
    meta = {}
    sent_at = 0.0
    hello = json.loads(ws.recv())
    print("已连接 Vision：", hello)
    ws.settimeout(1)
    ws.send(json.dumps({"type": "hello", "max_fps": max(0, args.max_fps)}))
    last_heartbeat = time.monotonic()
    try:
        while True:
            now = time.monotonic()
            if now - last_heartbeat >= 5:
                ws.send(json.dumps({"type": "heartbeat"}))
                last_heartbeat = now
            try:
                message = ws.recv()
            except websocket.WebSocketTimeoutException:
                continue
            if isinstance(message, str):
                info = json.loads(message)
                if info.get("type") == "frame":
                    meta = info
                elif info.get("type") == "action.result":
                    print("动作结果：", info)
                continue

            if len(message) < 9:
                continue
            kind = message[0]
            payload = message[9:]
            if kind == 0:
                if decoder is None:
                    decoder = av.CodecContext.create("h264", "r")
                payload = avcc_description_to_annexb(payload)
            elif kind not in (1, 2):
                continue
            if decoder is None:
                continue

            # 先走 FFmpeg parser，避免 scrcpy 的 NAL 边界与 AVPacket 边界不一致。
            packets = decoder.parse(annexb_nals(payload))
            for packet in packets:
                try:
                    decoded = decoder.decode(packet)
                except av.error.InvalidDataError:
                    # 配置帧/旋转瞬间可能被单独送达，忽略该包，等待下一关键帧。
                    continue
                for frame in decoded:
                    image = frame.to_ndarray(format="bgr24")
                    height, width = image.shape[:2]
                # Python 本地显示一个绿色演示框。
                    x, y, w, h = int(width * .25), int(height * .25), int(width * .5), int(height * .5)
                    cv2.rectangle(image, (x, y), (x + w, y + h), (0, 230, 118), 3)
                    cv2.putText(image, "demo-box", (x, max(24, y - 8)), cv2.FONT_HERSHEY_SIMPLEX, .8, (0, 230, 118), 2)
                    cv2.imshow("MyWebScrcpy Vision Demo", image)

                # 同一个框以归一化坐标回传，浏览器播放器会在 overlay Canvas 上显示。
                    now = time.monotonic()
                    if now - sent_at >= .2:
                        ws.send(json.dumps({
                        "type": "detection.result",
                        "device_id": args.serial,
                        "frame_id": meta.get("frame_id", 0),
                        "timestamp": meta.get("timestamp", 0),
                        "objects": [{"label": "demo-box", "confidence": .99, "x": .25, "y": .25, "w": .5, "h": .5}],
                        }))
                        sent_at = now
                    if cv2.waitKey(1) & 0xFF in (ord("q"), 27):
                        return
    finally:
        ws.close()
        cv2.destroyAllWindows()


if __name__ == "__main__":
    main()
