#!/usr/bin/env python3
# /// script
# requires-python = ">=3.10"
# dependencies = ["indra-ocr==0.3.2", "av>=14,<20", "websocket-client>=1.8,<2"]
# ///
"""独立 Indra OCR 测试：Vision H.264 -> OCR -> 播放器绿框。

uv run --python 3.12 scripts/vision/indra_ocr_service.py --serial DEVICE
不修改 OCR 配置，不发送设备动作；Ctrl+C 退出。详见 indra_ocr_README.md。
"""

import argparse
import json
import logging
import math
import multiprocessing as mp
import queue
import signal
import threading
import time
from dataclasses import dataclass
from fractions import Fraction
from urllib.parse import urlencode, urlsplit, urlunsplit

import websocket

LOG = logging.getLogger("indra-test")


@dataclass
class Frame:
    frame_id: int
    generation: int
    received_at: float
    image: object


def line_objects(page, width, height, min_confidence):
    """Indra 像素框 -> Vision 归一化框；尺寸是实际送入 OCR 的图像尺寸。"""
    if page.rotation != 0:
        raise ValueError("OCR 意外旋转画面，拒绝发送坐标")
    objects = []
    for line in page.lines:
        confidence = float(line.conf)
        coords = [float(v) for v in line.box]
        if len(coords) != 4 or not all(math.isfinite(v) for v in coords):
            raise ValueError("Indra 返回无效坐标")
        if not math.isfinite(confidence) or not 0 <= confidence <= 1:
            raise ValueError("Indra 返回无效置信度")
        if not line.text.strip() or confidence < min_confidence:
            continue
        x0, y0, x1, y1 = coords
        x0, x1 = max(0, min(width, x0)), max(0, min(width, x1))
        y0, y1 = max(0, min(height, y0)), max(0, min(height, y1))
        if x1 > x0 and y1 > y0:
            x, y = x0 / width, y0 / height
            objects.append(dict(label=line.text, confidence=confidence, x=x, y=y,
                                w=min(x1 / width - x, 1 - x),
                                h=min(y1 / height - y, 1 - y)))
    return objects


def ocr_worker(requests, results, options):
    """模型独占一个进程；队列容量各为 1，父进程负责超时及退出回收。"""
    signal.signal(signal.SIGINT, signal.SIG_IGN)
    try:
        import cv2
        from indra_ocr import OrtOCR, model_dir

        engine = OrtOCR(model_dir(options.model_dir), threads=options.threads)
        results.put(("ready", None))
        while True:
            image = requests.get()
            if image is None:
                return
            started = time.monotonic()
            gray = cv2.cvtColor(image, cv2.COLOR_BGR2GRAY)
            height, width = gray.shape
            if options.max_side and max(width, height) > options.max_side:
                scale = options.max_side / max(width, height)
                gray = cv2.resize(gray, (max(1, round(width * scale)),
                                        max(1, round(height * scale))))
            height, width = gray.shape
            page = engine(gray, turn=False)
            objects = line_objects(page, width, height, options.min_confidence)
            results.put(("result", (objects, (time.monotonic() - started) * 1000)))
    except Exception as exc:
        LOG.exception("Indra OCR 进程失败")
        results.put(("error", f"{type(exc).__name__}: {exc}"))


class VisionStream:
    """收包线程持续解码，只保留最新一帧；发送和状态分别加锁。"""

    def __init__(self, base_url, serial):
        parsed = urlsplit(base_url)
        if (parsed.scheme not in {"http", "https"} or not parsed.netloc
                or parsed.path not in {"", "/"} or parsed.query or parsed.fragment
                or parsed.username or parsed.password):
            raise ValueError("--url 必须是无凭据、无路径的 HTTP(S) 服务地址")
        url = urlunsplit(("wss" if parsed.scheme == "https" else "ws", parsed.netloc,
                          "/api/vision/stream",
                          urlencode(dict(serial=serial, current_frame="true")), ""))
        self.serial = serial
        self.lock = threading.RLock()
        self.send_lock = threading.Lock()
        self.stop = threading.Event()
        self.latest = None
        self.generation = 0
        self.error = None
        self.session = ""
        self.ws = websocket.create_connection(url, timeout=30, enable_multithread=True,
                                              http_no_proxy=["*"])
        try:
            hello = json.loads(self.ws.recv())
            if (hello["type"] != "hello" or hello["protocol_version"] != "1"
                    or hello["format"] != "scrcpy-frame" or hello["codec"] != "h264"
                    or hello["device_id"] != serial or not hello["session_id"]):
                raise ValueError("不支持的 Vision 握手")
            self.session = hello["session_id"]
            self.send("hello", max_fps=0)  # 不丢 H.264 预测帧；只限制 OCR 频率。
            self.ws.settimeout(.2)
        except BaseException:
            self.ws.close()
            raise
        self.thread = threading.Thread(target=self._receive, name="vision-receiver", daemon=True)
        self.thread.start()
        LOG.info("Vision 已连接：device=%s session=%s", serial, self.session)

    def send(self, kind, **fields):
        with self.send_lock:
            self.ws.send(json.dumps(dict(type=kind, protocol_version="1", device_id=self.serial,
                                         session_id=self.session, **fields), ensure_ascii=False))

    def current(self):
        with self.lock:
            if self.error is not None:
                raise RuntimeError("Vision 收帧失败") from self.error
            return self.latest

    def publish(self, frame, objects, expires_ms):
        with self.lock:
            if self.error is not None or self.stop.is_set() or self.latest is None:
                return False
            if frame.generation != self.generation:
                return False
            if frame is not self.latest:
                remaining = int(600 - (time.monotonic() - frame.received_at) * 1000)
                if remaining <= 0:
                    return False
                expires_ms = min(expires_ms, remaining)
            self.send("detection.result", frame_id=frame.frame_id, timestamp=time.time_ns() // 1_000_000,
                      expires_ms=expires_ms, objects=objects)
            return True

    def clear(self):
        self.send("detection.result", timestamp=time.time_ns() // 1_000_000,
                  expires_ms=600, objects=[])

    def _receive(self):
        import av  # 留在收帧进程，避免 macOS 中 PyAV / OpenCV 的 FFmpeg 库冲突。

        decoder, meta, metadata = None, None, {}
        waiting_key, last_id = True, 0
        heartbeat_at, pending_heartbeat = time.monotonic(), None
        try:
            while not self.stop.is_set():
                now = time.monotonic()
                if pending_heartbeat is not None and now - pending_heartbeat > 5:
                    raise TimeoutError("Vision 心跳响应超时")
                if pending_heartbeat is None and now - heartbeat_at >= 5:
                    self.send("heartbeat")
                    heartbeat_at = pending_heartbeat = now
                try:
                    message = self.ws.recv()
                except websocket.WebSocketTimeoutException:
                    continue
                if message in ("", b""):
                    raise ConnectionError("Vision 连接已关闭")
                if isinstance(message, str):
                    info = json.loads(message)
                    kind = info["type"]
                    if kind == "error":
                        raise RuntimeError(f"Vision 返回错误：{info}")
                    if info["session_id"] != self.session or info["device_id"] != self.serial:
                        raise ValueError("Vision 设备或会话发生变化")
                    if kind == "frame":
                        if meta is not None or info["frame_id"] <= last_id or info["codec"] != "h264":
                            raise ValueError("Vision 帧元数据不合法")
                        if type(info["replayed"]) is not bool:
                            raise ValueError("Vision replayed 必须是布尔值")
                        meta, last_id = info, info["frame_id"]
                        meta["received_at"] = time.monotonic()
                    elif kind == "heartbeat":
                        pending_heartbeat = None
                    elif kind != "hello.ack" or info["max_fps"] != 0:
                        raise ValueError(f"未知 Vision 消息：{kind}")
                    continue
                if (meta is None or len(message) < 9 or message[0] != meta["kind"]
                        or int.from_bytes(message[1:9], "big") != meta["pts"]):
                    raise ValueError("Vision 元数据与二进制包不匹配")
                source, meta = meta, None
                kind, payload = message[0], message[9:]
                if kind in {0, 3}:
                    with self.lock:
                        self.generation += 1
                        self.latest = None
                        self.clear()
                    metadata.clear()
                    decoder, waiting_key = None, True
                    if kind == 3:
                        if (len(payload) != 8 or int.from_bytes(payload[:4], "big") != source["width"]
                                or int.from_bytes(payload[4:], "big") != source["height"]):
                            raise ValueError("Vision 旋转包尺寸不匹配")
                        continue
                    decoder = av.CodecContext.create("h264", "r")
                    decoder.thread_count = 1
                    decoder.extradata = payload  # FFmpeg 原生处理 Annex-B / avcC 配置。
                    continue
                if kind not in {1, 2} or decoder is None:
                    raise ValueError("Vision 视频包先于配置或类型无效")
                if waiting_key and kind != 1:
                    continue
                waiting_key = False
                packet = av.Packet(payload)
                packet.pts, packet.time_base = source["frame_id"], Fraction(1, 1_000_000)
                metadata[packet.pts] = source
                if len(metadata) > 128:
                    raise ValueError("解码帧元数据积压超过 128")
                for decoded in decoder.decode(packet):
                    origin = metadata.pop(decoded.pts)
                    if (decoded.width, decoded.height) != (origin["width"], origin["height"]):
                        raise ValueError("解码尺寸与元数据不匹配")
                    if origin["replayed"] and not origin["snapshot_complete"]:
                        continue  # 缓存关键帧仅用于初始化解码器。
                    image = decoded.to_ndarray(format="bgr24")
                    with self.lock:
                        self.latest = Frame(origin["frame_id"], self.generation, origin["received_at"], image)
        except Exception as exc:
            if not self.stop.is_set():
                LOG.exception("Vision 收包线程退出")
                with self.lock:
                    self.error, self.latest = exc, None

    def close(self):
        self.stop.set()
        try:
            self.clear()
        except (websocket.WebSocketException, OSError):
            LOG.warning("连接已关闭，绿框将按有效期自动消失")
        finally:
            self.ws.close(timeout=1)
            self.thread.join(timeout=2)
        if self.thread.is_alive():
            raise RuntimeError("Vision 收包线程未能退出")


def run(options):
    context = mp.get_context("spawn")
    requests, results = context.Queue(maxsize=1), context.Queue(maxsize=1)
    worker = context.Process(target=ocr_worker, args=(requests, results, options), daemon=True)
    stream = None
    worker.start()
    try:
        LOG.info("加载 Indra OCR；首次运行会下载模型")
        deadline = time.monotonic() + options.load_timeout
        while True:
            try:
                status, value = results.get(timeout=.2)
                break
            except queue.Empty:
                if not worker.is_alive():
                    raise RuntimeError("OCR 进程在加载模型时退出")
                if time.monotonic() >= deadline:
                    raise TimeoutError("模型加载超时；可使用 --model-dir 指定本地模型")
        if status != "ready":
            raise RuntimeError(f"模型加载失败：{value}")
        LOG.info("模型已就绪")
        stream = VisionStream(options.url, options.serial)
        started = last_frame_at = time.monotonic()
        last_submitted, inflight, completed = None, None, None
        last_submit_at, last_publish_at, count = 0., 0., 0
        while not options.seconds or time.monotonic() - started < options.seconds:
            now = time.monotonic()
            current = stream.current()
            if current is not None:
                last_frame_at = now
            elif now - last_frame_at >= options.frame_timeout:
                raise TimeoutError("未收到可识别画面，请检查设备视频流")
            if not worker.is_alive():
                raise RuntimeError("OCR 进程意外退出")
            if inflight is not None:
                try:
                    status, value = results.get_nowait()
                except queue.Empty:
                    if now - last_submit_at > options.ocr_timeout:
                        raise TimeoutError("单帧 OCR 超时")
                else:
                    if status != "result":
                        raise RuntimeError(f"OCR 失败：{value}")
                    objects, elapsed = value
                    completed, inflight = (inflight, objects), None
                    count += 1
                    published = stream.publish(completed[0], objects, options.expires_ms)
                    last_publish_at = now
                    LOG.info("frame=%d boxes=%d OCR=%.1fms published=%s",
                             completed[0].frame_id, len(objects), elapsed, published)
            if completed is not None and now - last_publish_at >= options.expires_ms / 2000:
                if not stream.publish(*completed, options.expires_ms):
                    completed = None
                last_publish_at = now
            if (current is not None and current is not last_submitted and inflight is None
                    and now - last_submit_at >= options.interval):
                requests.put_nowait(current.image)
                last_submitted = inflight = current
                last_submit_at = now
            time.sleep(.02)
        if count == 0:
            raise RuntimeError("测试期间未完成任何 OCR，请增加 --seconds")
        LOG.info("测试结束，共识别 %d 帧", count)
    finally:
        try:
            if stream is not None:
                stream.close()
        finally:
            try:
                requests.put_nowait(None)
            except queue.Full:
                pass
            worker.join(timeout=2)
            if worker.is_alive():
                LOG.info("回收 OCR 子进程")
                worker.terminate()
                worker.join(timeout=2)
            if worker.is_alive():
                worker.kill()
                worker.join(timeout=2)
            for channel in (requests, results):
                channel.cancel_join_thread()
                channel.close()


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--serial", required=True, help="目标设备序列号，不自动选择设备")
    parser.add_argument("--url", default="http://127.0.0.1:8080")
    parser.add_argument("--threads", type=int, default=4, help="ONNX CPU 线程数")
    parser.add_argument("--interval", type=float, default=.5, help="两次 OCR 的最小间隔秒数")
    parser.add_argument("--min-confidence", type=float, default=.5)
    parser.add_argument("--max-side", type=int, default=0, help="识别图最长边，0 为原始分辨率")
    parser.add_argument("--expires-ms", type=int, default=1500, help="绿框有效期毫秒数")
    parser.add_argument("--model-dir", help="本地 Indra 模型目录；省略时按官方逻辑下载及缓存")
    parser.add_argument("--load-timeout", type=float, default=300)
    parser.add_argument("--ocr-timeout", type=float, default=30)
    parser.add_argument("--frame-timeout", type=float, default=30)
    parser.add_argument("--seconds", type=float, default=0, help="自动结束秒数，0 为持续运行")
    options = parser.parse_args()
    numbers = (options.interval, options.load_timeout, options.ocr_timeout,
               options.frame_timeout, options.seconds, options.min_confidence)
    if (not all(math.isfinite(n) for n in numbers) or options.threads < 1
            or min(options.interval, options.load_timeout, options.ocr_timeout, options.frame_timeout) <= 0
            or not 0 <= options.min_confidence <= 1 or options.max_side < 0
            or not 100 <= options.expires_ms <= 10000 or options.seconds < 0):
        parser.error("参数范围无效，请检查线程数、时间、分辨率和置信度")
    logging.basicConfig(level=logging.INFO, format="%(asctime)s %(levelname)s %(message)s")
    def stop_on_signal(signum, frame):
        raise KeyboardInterrupt

    signal.signal(signal.SIGTERM, stop_on_signal)
    try:
        run(options)
    except KeyboardInterrupt:
        LOG.info("已停止")
    except Exception:
        LOG.exception("测试服务失败")
        raise SystemExit(1)


if __name__ == "__main__":
    main()
