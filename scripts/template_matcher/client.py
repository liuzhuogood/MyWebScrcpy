from __future__ import annotations

import json
import logging
import queue
import ssl
import threading
import time
from typing import Callable, Optional

import av
import websocket

from .matcher import TemplateMatcher
from .template_loader import TemplateLoader

logger = logging.getLogger("template_matcher")


def annexb_nals(data: bytes) -> bytes:
    """把 4 字节大端序长度前缀的 H264 NAL 转成 Annex-B 前缀 (00 00 00 01)"""
    if data.startswith(b"\x00\x00\x00\x01") or data.startswith(b"\x00\x00\x01"):
        return data
    out = bytearray()
    pos = 0
    while pos + 4 <= len(data):
        size = int.from_bytes(data[pos : pos + 4], "big")
        pos += 4
        if size <= 0 or pos + size > len(data):
            return data
        out += b"\x00\x00\x00\x01" + data[pos : pos + size]
        pos += size
    return bytes(out) if pos == len(data) and out else data


def avcc_description_to_annexb(data: bytes) -> bytes:
    """提取 avcC 结构中的 SPS/PPS 供 PyAV 初始化解码器"""
    if len(data) < 7 or data[0] != 1:
        return annexb_nals(data)
    pos = 5
    sps_count = data[pos] & 0x1F
    pos += 1
    out = bytearray()
    for _ in range(sps_count):
        if pos + 2 > len(data):
            return data
        size = int.from_bytes(data[pos : pos + 2], "big")
        pos += 2
        if pos + size > len(data):
            return data
        out += b"\x00\x00\x00\x01" + data[pos : pos + size]
        pos += size
    if pos >= len(data):
        return bytes(out)
    pps_count = data[pos]
    pos += 1
    for _ in range(pps_count):
        if pos + 2 > len(data):
            break
        size = int.from_bytes(data[pos : pos + 2], "big")
        pos += 2
        if pos + size > len(data):
            break
        out += b"\x00\x00\x00\x01" + data[pos : pos + size]
        pos += size
    return bytes(out) or data


class FrameDecoder:
    """视频帧解码器：直接消费数据包，不等待下一帧边界。"""

    def __init__(self):
        self.codec: Optional[av.CodecContext] = None

    def decode(self, kind: int, payload: bytes):
        if kind == 3:  # 尺寸变化或重置
            self.codec = None
            return []
        if kind == 0:  # Codec config (SPS/PPS)
            self.codec = av.CodecContext.create("h264", "r")
            self.codec.extradata = avcc_description_to_annexb(payload)
            return []
        if kind not in (1, 2) or self.codec is None:
            return []
        try:
            return self.codec.decode(av.Packet(annexb_nals(payload)))
        except av.error.InvalidDataError:
            return []


class LatestFrames:
    """持续解码；当匹配耗时时只保留最新画面（通过无锁队列丢弃积压帧，避免旧帧排队延迟）。"""

    def __init__(self, ws: websocket.WebSocket):
        self.ws = ws
        self.pending: queue.Queue = queue.Queue(maxsize=1)
        self.stopped = threading.Event()
        self.error: Optional[Exception] = None
        self.thread = threading.Thread(target=self._read, daemon=True)
        self.thread.start()

    def _offer(self, item):
        try:
            self.pending.get_nowait()
        except queue.Empty:
            pass
        try:
            self.pending.put_nowait(item)
        except queue.Full:
            pass

    def _read(self):
        decoder = FrameDecoder()
        meta = {}
        try:
            while not self.stopped.is_set():
                try:
                    message = self.ws.recv()
                except websocket.WebSocketTimeoutException:
                    continue
                if message == "":
                    raise websocket.WebSocketConnectionClosedException("Vision disconnected")
                if isinstance(message, str):
                    info = json.loads(message)
                    if info.get("type") == "frame":
                        meta = info
                    continue
                if len(message) < 9:
                    continue
                kind = message[0]
                if kind in (0, 3):
                    self._offer((dict(meta), None))
                for frame in decoder.decode(kind, message[9:]):
                    self._offer((dict(meta), frame))
        except Exception as exc:
            self.error = exc

    def get(self, timeout: float = 0.2):
        if self.error is not None:
            raise self.error
        return self.pending.get(timeout=timeout)

    def close(self):
        self.stopped.set()
        try:
            self.ws.close()
        except Exception:
            pass
        self.thread.join(timeout=2)


class StreamMatcherClient:
    def __init__(
        self,
        base_url: str,
        serial: str,
        loader: TemplateLoader,
        matcher: TemplateMatcher,
        max_fps: int = 5,
        insecure: bool = False,
        on_match: Optional[Callable[[list], None]] = None,
    ):
        self.base_url = base_url.rstrip("/")
        self.serial = serial
        self.loader = loader
        self.matcher = matcher
        self.max_fps = max_fps
        self.insecure = insecure
        self.on_match = on_match

        self.ws: Optional[websocket.WebSocket] = None
        self.stopped = False
        self.last_reload_time = 0.0

    def start(self):
        self.stopped = False
        while not self.stopped:
            try:
                self._run_connection()
            except Exception as e:
                if self.stopped:
                    break
                logger.warning("连接异常断开: %s，将在 3 秒后重试...", e)
                time.sleep(3)

    def stop(self):
        self.stopped = True
        if self.ws:
            try:
                self.ws.close()
            except Exception:
                pass
            self.ws = None

    def _run_connection(self):
        url = self.base_url
        if url.startswith("http://"):
            url = "ws://" + url[7:]
        elif url.startswith("https://"):
            url = "wss://" + url[8:]
        elif not url.startswith("ws://") and not url.startswith("wss://"):
            url = "ws://" + url

        encoded_serial = self.serial.replace(":", "%3A")
        ws_url = f"{url}/api/vision/stream?serial={encoded_serial}"
        logger.info("正在连接视频流: %s", ws_url)

        sslopt = {}
        if self.insecure:
            sslopt = {"cert_reqs": ssl.CERT_NONE, "check_hostname": False}

        self.ws = websocket.create_connection(ws_url, timeout=30, sslopt=sslopt)

        # 接收服务端的 hello 消息
        hello_raw = self.ws.recv()
        hello_data = json.loads(hello_raw)
        session_id = hello_data.get("session_id", "")
        logger.info("已连接并收到握手问候: session_id=%s, format=%s", session_id, hello_data.get("format"))

        # 回传客户端参数（订阅协商 max_fps）
        self.ws.send(json.dumps({"type": "hello", "max_fps": max(0, self.max_fps)}))
        self.ws.settimeout(0.2)

        receiver = LatestFrames(self.ws)
        last_heartbeat = time.monotonic()
        self.last_reload_time = time.monotonic()
        self.loader.load_all()

        try:
            while not self.stopped:
                now = time.monotonic()

                # 每 5 秒发送一次心跳
                if now - last_heartbeat >= 5:
                    self.ws.send(json.dumps({"type": "heartbeat"}))
                    last_heartbeat = now

                # 每 10 秒自动检查模板热重载
                if now - self.last_reload_time >= 10:
                    self.loader.load_all()
                    self.last_reload_time = now

                try:
                    meta, frame = receiver.get(timeout=0.2)
                except queue.Empty:
                    continue

                if frame is None:
                    continue

                frame_bgr = frame.to_ndarray(format="bgr24")
                self._process_frame(frame_bgr, meta, session_id)
        finally:
            receiver.close()

    def _process_frame(self, image_bgr, meta: dict, session_id: str):
        matches = self.matcher.match_frame(image_bgr, self.loader.templates)
        objects = [m.to_dict() for m in matches]

        if self.on_match:
            try:
                self.on_match(objects)
            except Exception:
                pass

        frame_id = meta.get("frame_id", 0)
        timestamp = int(time.time() * 1000)

        result_payload = {
            "type": "detection.result",
            "device_id": self.serial,
            "session_id": session_id,
            "frame_id": frame_id,
            "timestamp": timestamp,
            "expires_ms": 600,
            "objects": objects,
        }

        if self.ws:
            self.ws.send(json.dumps(result_payload))
            if objects:
                logger.info(
                    "命中目标 [%d 个]: %s",
                    len(objects),
                    [o["label"] + f"({int(o['confidence']*100)}%)" for o in objects],
                )
