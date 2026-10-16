"""独立测试脚本的协议、解码、坐标和过期保护回归。"""

import json
import queue
import re
import threading
import time
import unittest
from fractions import Fraction
from types import SimpleNamespace
from unittest.mock import patch

import av
import numpy as np
import websocket

from indra_ocr_service import Frame, VisionStream, line_objects


class FakeSocket:
    def __init__(self):
        self.incoming = queue.Queue()
        self.sent = []
        self.closed = False
        self.incoming.put(json.dumps(dict(type="hello", protocol_version="1", format="scrcpy-frame",
                                         codec="h264", device_id="test", session_id="session")))

    def recv(self):
        try:
            return self.incoming.get(timeout=.02)
        except queue.Empty:
            raise websocket.WebSocketTimeoutException()

    def send(self, message):
        self.sent.append(json.loads(message))

    def settimeout(self, timeout):
        pass

    def close(self, **kwargs):
        self.closed = True
        self.incoming.put("")

    def packet(self, frame_id, kind, payload, *, pts=0, replayed=False, snapshot_complete=False, corrupt=False):
        self.incoming.put(json.dumps(dict(type="frame", device_id="test", session_id="session",
                                         frame_id=frame_id, kind=kind, pts=pts, timestamp=123,
                                         width=320, height=240, codec="h264", replayed=replayed,
                                         snapshot_complete=snapshot_complete)))
        self.incoming.put(bytes([kind + 1 if corrupt else kind]) + pts.to_bytes(8, "big") + payload)


def encoded_frame(avcc=False):
    encoder = av.CodecContext.create("libx264", "w")
    encoder.width, encoder.height = 320, 240
    encoder.pix_fmt, encoder.time_base = "yuv420p", Fraction(1, 30)
    encoder.options = {"preset": "ultrafast", "tune": "zerolatency"}
    frame = av.VideoFrame.from_ndarray(np.full((240, 320, 3), 220, dtype=np.uint8), format="bgr24")
    packets = encoder.encode(frame) + encoder.encode(None)
    nals = [n for n in re.split(b"\x00\x00\x00?\x01", bytes(packets[0])) if n]
    sps = next(n for n in nals if n[0] & 31 == 7)
    pps = next(n for n in nals if n[0] & 31 == 8)
    if avcc:
        config = (bytes([1]) + sps[1:4] + bytes([255, 225]) + len(sps).to_bytes(2, "big")
                  + sps + bytes([1]) + len(pps).to_bytes(2, "big") + pps)
        payload = b"".join(len(n).to_bytes(4, "big") + n for n in nals)
    else:
        config = b"\x00\x00\x00\x01" + sps + b"\x00\x00\x00\x01" + pps
        payload = bytes(packets[0])
    return config, payload


class IndraTest(unittest.TestCase):
    def test_boxes_clip_normalize_and_filter(self):
        lines = [SimpleNamespace(box=[-2, 10, 105, 60], text="中文 Hello", conf=.9),
                 SimpleNamespace(box=[0, 0, 2, 2], text="low", conf=.1),
                 SimpleNamespace(box=[0, 0, 2, 2], text=" ", conf=.9)]
        boxes = line_objects(SimpleNamespace(rotation=0, lines=lines), 100, 200, .5)
        self.assertEqual(boxes, [dict(label="中文 Hello", confidence=.9, x=0., y=.05, w=1., h=.25)])

    def test_invalid_ocr_data_fails(self):
        with self.assertRaises(ValueError):
            line_objects(SimpleNamespace(rotation=90, lines=[]), 100, 200, .5)
        for box, conf in (([0, 0, float("nan"), 1], .9), ([0, 0, 1, 1], 1.1)):
            with self.assertRaises(ValueError):
                line_objects(SimpleNamespace(rotation=0, lines=[SimpleNamespace(box=box, text="x", conf=conf)]),
                             100, 200, .5)

    def test_stale_results_and_generation(self):
        stream = VisionStream.__new__(VisionStream)
        stream.lock, stream.stop = threading.RLock(), threading.Event()
        stream.error, stream.generation = None, 1
        sent = []
        stream.send = lambda kind, **fields: sent.append(fields)
        old = Frame(1, 1, time.monotonic() - 10, None)
        stream.latest = old
        self.assertTrue(stream.publish(old, [], 1500))  # 静止画面可以续期。
        stream.latest = Frame(2, 1, time.monotonic(), None)
        self.assertFalse(stream.publish(old, [], 1500))
        old.received_at = time.monotonic() - .1
        self.assertTrue(stream.publish(old, [], 1500))
        self.assertLessEqual(sent[-1]["expires_ms"], 500)
        stream.generation = 2
        self.assertFalse(stream.publish(old, [], 1500))

    def test_real_h264_decode_and_rotation(self):
        for avcc in (False, True):
            with self.subTest(avcc=avcc):
                socket = FakeSocket()
                with patch("indra_ocr_service.websocket.create_connection", return_value=socket):
                    stream = VisionStream("http://localhost:8080", "test")
                try:
                    config, payload = encoded_frame(avcc)
                    socket.packet(1, 0, config, replayed=True)
                    socket.packet(2, 1, payload, pts=10, replayed=True, snapshot_complete=True)
                    deadline = time.monotonic() + 2
                    while stream.current() is None and time.monotonic() < deadline:
                        time.sleep(.01)
                    frame = stream.current()
                    self.assertIsNotNone(frame)
                    self.assertEqual(frame.frame_id, 2)
                    self.assertEqual(frame.image.shape, (240, 320, 3))
                    self.assertEqual(socket.sent[0]["max_fps"], 0)
                    self.assertTrue(stream.publish(frame, [], 1500))
                    socket.packet(3, 3, (320).to_bytes(4, "big") + (240).to_bytes(4, "big"))
                    deadline = time.monotonic() + 2
                    while stream.generation == frame.generation and time.monotonic() < deadline:
                        time.sleep(.01)
                    self.assertIsNone(stream.current())
                    self.assertFalse(stream.publish(frame, [], 1500))
                finally:
                    stream.close()
                self.assertFalse(stream.thread.is_alive())
                self.assertTrue(socket.closed)

    def test_protocol_mismatch_exits_receiver(self):
        socket = FakeSocket()
        with patch("indra_ocr_service.websocket.create_connection", return_value=socket):
            stream = VisionStream("http://localhost:8080", "test")
        try:
            socket.packet(1, 0, b"config", corrupt=True)
            deadline = time.monotonic() + 2
            while stream.error is None and time.monotonic() < deadline:
                time.sleep(.01)
            with self.assertRaises(RuntimeError):
                stream.current()
        finally:
            stream.close()


if __name__ == "__main__":
    unittest.main()
