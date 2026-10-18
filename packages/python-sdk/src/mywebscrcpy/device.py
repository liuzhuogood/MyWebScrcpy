import logging
import math
import threading
import time
from contextlib import contextmanager

from .contracts import Deadline
from .errors import ActionTimeout, FrameTimeout, MatchTimeout
from .templates import TemplateCache

from .errors import CommandError, ProtocolError, SDKError
from .models import CommandResult, PageInfo

logger = logging.getLogger(__name__)


class VideoStream:
    """Active stream exposed by Device.startVideoStream()."""

    def __init__(self, session, first_frame):
        self._session = session
        self.first_frame = first_frame
        self.session_id = session.session_id
        self.synced_to_vision_frame = False

    @property
    def closed(self):
        return self._session._closed

    def current_frame(self, timeout=10):
        return self._session.current_frame(Deadline(_seconds(timeout)))


def _seconds(value):
    if type(value) not in {int, float} or not math.isfinite(value) or not 0 < value <= 600:
        raise ValueError("timeout 必须在 0 到 600 秒之间")
    return value


class Device:
    """Device-bound HTTP, Vision actions and local template matching."""

    def __init__(self, client, serial: str):
        if not isinstance(serial, str) or not serial.strip():
            raise ValueError("serial 必须为非空字符串")
        self._client = client
        self.serial = serial
        self._closed = False
        self._vision = None
        self._video_stream = None
        self._watch = None
        self._lock = threading.RLock()
        self._stop = threading.Event()
        self._cache = TemplateCache(client, serial)

    def _check_open(self):
        if self._closed:
            raise SDKError("Device 已关闭")
        self._client._check_open()

    def adb(self, args: list[str], timeout: float = 30) -> CommandResult:
        self._check_open()
        seconds = _seconds(timeout)
        if not isinstance(args, list) or not 1 <= len(args) <= 128 or any(not isinstance(v, str) or not v or "\x00" in v for v in args):
            raise ValueError("args 必须包含 1 到 128 个非空字符串")
        forbidden = {"adb", "-s", "--serial", "-H", "-P", "-L"}
        if any(v in forbidden or v.startswith(("-H", "-P", "-L")) for v in args):
            raise ValueError("args 不得改变 ADB 宿主或目标设备")
        logger.debug("ADB command started: serial=%s timeout=%s", self.serial, seconds)
        value = self._client._request("POST", "/api/adb/commands", serial=self.serial,
                                      timeout=seconds + 10,
                                      body={"args": args, "timeout_ms": max(1, round(seconds * 1000))})
        try:
            status = value["status"]
            if status not in {"queued", "running", "completed", "failed", "timed_out"}:
                raise ValueError("status")
            # A successful synchronous result must contain its actual exit code.
            exit_code = value["exit_code"] if status == "completed" else value.get("exit_code")
            if (status == "completed" and type(exit_code) is not int) or (exit_code is not None and type(exit_code) is not int):
                raise ValueError("exit_code")
            stdout, stderr = value.get("stdout", ""), value.get("stderr", "")
            truncated = value.get("truncated", False)
            if not isinstance(stdout, str) or not isinstance(stderr, str) or type(truncated) is not bool:
                raise ValueError("output")
        except (KeyError, ValueError, TypeError) as error:
            raise ProtocolError("ADB 响应缺少字段或字段类型不合法") from error
        result = CommandResult(status, exit_code, stdout, stderr, truncated)
        if status != "completed" or exit_code != 0 or truncated:
            logger.warning("ADB command failed: serial=%s status=%s exit_code=%s truncated=%s", self.serial, status, exit_code, truncated)
            raise CommandError(result)
        logger.debug("ADB command completed: serial=%s", self.serial)
        return result

    def page_info(self, timeout: float = 10) -> PageInfo:
        self._check_open()
        value = self._client._request("GET", "/api/ui/page-info", serial=self.serial, timeout=_seconds(timeout))
        try:
            display = value["display"]
            width, height = display["width"], display["height"]
            if type(width) is not int or type(height) is not int or width <= 0 or height <= 0:
                raise ValueError("display dimensions")
        except (KeyError, ValueError, TypeError) as error:
            raise ProtocolError("响应缺少有效 display.width/height") from error
        return PageInfo(width, height)

    def _session(self, deadline, *, video=False):
        from .vision import Vision
        self._check_open()
        with self._lock:
            self._check_open()
            if self._vision is None:
                self._vision = Vision(self._client, self.serial, deadline, video=video)
            self._vision._check()
            return self._vision

    def _frame(self, deadline):
        from .dependencies import vision_dependencies
        vision_dependencies()
        return self._session(deadline, video=True).frame(deadline)

    def frame(self, timeout=10):
        return self._frame(Deadline(timeout))

    @contextmanager
    def startVideoStream(self, timeout=15, *, sync_vision_frame=False):
        """Open a video stream, yield its first decoded frame, then close it."""
        from .vision import Vision

        self._check_open()
        deadline = Deadline(_seconds(timeout))
        session = Vision(self._client, self.serial, deadline, video=True)
        with self._lock:
            self._check_open()
            if self._video_stream is not None and not self._video_stream.closed:
                raise SDKError("Device 已有活动的视频流")
            self._vision = session
            stream = VideoStream(session, None)
            self._video_stream = stream
        try:
            first_frame = session.current_frame(deadline)
            stream.first_frame = first_frame
            if sync_vision_frame:
                stream.synced_to_vision_frame = self._client._sync_vision_frame(self.serial, first_frame)
            logger.info("Vision video stream started: serial=%s session_id=%s", self.serial, session.session_id)
            yield stream
        finally:
            session.close()
            with self._lock:
                if self._vision is session:
                    self._vision = None
                if self._video_stream is stream:
                    self._video_stream = None

    start_video_stream = startVideoStream

    def match(self, template_id, timeout=10):
        from .matcher import Matcher
        deadline = Deadline(timeout)
        template, image = self._cache.get(template_id, deadline)
        result = Matcher().match_frame(self._frame(deadline), template, image)
        deadline.remaining(FrameTimeout)
        return result

    def wait_for(self, template_id, timeout=10):
        from .matcher import Matcher
        deadline = Deadline(timeout)
        template, image = self._cache.get(template_id, deadline)
        refresh_at = time.monotonic() + 10
        while True:
            try:
                if time.monotonic() >= refresh_at:
                    template, image = self._cache.get(template_id, deadline)
                    refresh_at = time.monotonic() + 10
                matches = Matcher().match_frame(self._frame(deadline), template, image)
                deadline.remaining(MatchTimeout)
                if matches:
                    return max(matches, key=lambda match: match.confidence)
            except FrameTimeout as error:
                raise MatchTimeout('期限内未取得匹配目标') from error

    def watch_templates(self, publish=False, max_fps=5, timeout=10):
        from .watch import Watch
        return Watch(self, publish=publish, max_fps=max_fps, timeout=timeout)

    def ocr(
        self,
        bbox: list[float] | tuple[float, float, float, float] | None = None,
        *,
        keywords: list[str] | str | None = None,
        mark: bool = False,
        mark_duration_ms: int = 3000,
        engine: str = "auto",
        macocr_url: str | None = None,
        macocr_auth: str | None = None,
        paddleocr_url: str | None = None,
        indraocr_url: str | None = None,
        stabilization_ms: int = 0,
        timeout: float = 10,
    ):
        from .ocr import OCRBox, OCRResult, crop_image, perform_ocr
        if type(stabilization_ms) is not int or not 0 <= stabilization_ms <= 5000:
            raise ValueError("stabilization_ms 必须是 0 到 5000 的整数")
        if stabilization_ms:
            from .ocr_sampling import stabilized_ocr
            return stabilized_ocr(self, stabilization_ms=stabilization_ms, bbox=bbox,
                keywords=keywords, mark=mark, mark_duration_ms=mark_duration_ms,
                timeout=timeout, engine=engine, macocr_url=macocr_url, macocr_auth=macocr_auth,
                paddleocr_url=paddleocr_url, indraocr_url=indraocr_url)
        deadline = Deadline(timeout)
        started = time.monotonic()
        frame = self._session(deadline, video=True).current_frame(deadline)
        image = frame.image

        norm_bbox = list(bbox) if bbox is not None else [0.0, 0.0, 1.0, 1.0]
        if len(norm_bbox) != 4 or any(not (0 <= v <= 1) for v in norm_bbox):
            raise ValueError("bbox 必须包含 4 个 0 到 1 之间的数值 [x, y, w, h]")

        crop, _ = crop_image(image, norm_bbox)
        result, engine_used, errors = perform_ocr(
            crop,
            engine=engine,
            macocr_url=macocr_url,
            macocr_auth=macocr_auth,
            paddleocr_url=paddleocr_url,
            indraocr_url=indraocr_url,
            timeout=deadline.remaining(FrameTimeout),
        )

        wanted = []
        if isinstance(keywords, str):
            wanted = [k.strip().casefold() for k in keywords.split(",") if k.strip()]
        elif isinstance(keywords, (list, tuple)):
            wanted = [str(k).strip().casefold() for k in keywords if str(k).strip()]

        ch, cw = crop.shape[:2]
        ox, oy, ow, oh = norm_bbox
        boxes: list[OCRBox] = []
        for raw in result.get("ocr_boxes", []):
            text = str(raw.get("text", "")).strip()
            if not text:
                continue
            if wanted and not any(k in text.casefold() for k in wanted):
                continue
            rx, ry = float(raw.get("x", 0)), float(raw.get("y", 0))
            rw, rh = float(raw.get("w", 0)), float(raw.get("h", 0))
            # 兼容像素坐标与已归一化坐标
            if engine_used == "indraocr" or max(abs(rx), abs(ry), abs(rw), abs(rh)) > 1.0:
                nx, ny = rx / max(cw, 1), ry / max(ch, 1)
                nw, nh = rw / max(cw, 1), rh / max(ch, 1)
                px, py, pw, ph = round(rx), round(ry), round(rw), round(rh)
            else:
                nx, ny, nw, nh = rx, ry, rw, rh
                px, py, pw, ph = round(rx * cw), round(ry * ch), round(rw * cw), round(rh * ch)

            gx = ox + nx * ow
            gy = oy + ny * oh
            gw = nw * ow
            gh = nh * oh
            boxes.append(
                OCRBox(
                    text=text,
                    confidence=float(raw.get("confidence", 1.0)),
                    x=gx,
                    y=gy,
                    w=gw,
                    h=gh,
                    pixel_x=px,
                    pixel_y=py,
                    pixel_w=pw,
                    pixel_h=ph,
                )
            )

        full_text = str(result.get("ocr_result", "")).strip()
        ocr_result = OCRResult(
            status="matched" if boxes else "not_found",
            matched=bool(boxes),
            text=full_text,
            boxes=boxes,
            engine=engine_used,
            frame_id=frame.frame_id,
            elapsed_ms=round((time.monotonic() - started) * 1000),
            fallback_reason=errors,
            samples=1,
            engines_used=result["engines_used"] if engine_used == "parallel" else [engine_used],
        )

        if mark and boxes:
            self._session(deadline).publish_current(frame, boxes, expires_ms=mark_duration_ms)

        return ocr_result

    def tap_text(self, text: str, *, timeout: float = 10, **kwargs):
        """识别并点击指定文本所在中心位置。"""
        res = self.ocr(keywords=[text], timeout=timeout, **kwargs)
        matched = res.find(text)
        if not matched:
            raise MatchTimeout(f"未在画面中找到文字: {text}")
        target = max(matched, key=lambda b: b.confidence)
        return self.tap(target.center, timeout=timeout)

    def wait_for_text(self, text: str, *, timeout: float = 10, interval: float = 0.5, **kwargs):
        """持续识别画面，直到出现指定文字或达到超时；找到时返回 OCRResult。"""
        if not isinstance(text, str) or not text.strip():
            raise ValueError("text 必须为非空字符串")
        total = _seconds(timeout)
        if type(interval) not in {int, float} or not math.isfinite(interval) or not 0 < interval <= 60:
            raise ValueError("interval 必须在 0 到 60 秒之间")

        end = time.monotonic() + total
        while True:
            self._check_open()
            remaining = end - time.monotonic()
            if remaining <= 0:
                raise MatchTimeout(f"{total:g} 秒内未找到文字: {text}")
            try:
                result = self.ocr(keywords=text, timeout=remaining, **kwargs)
            except FrameTimeout as error:
                raise MatchTimeout(f"{total:g} 秒内未找到文字: {text}") from error
            if result.find(text):
                return result

            remaining = end - time.monotonic()
            if remaining <= 0:
                raise MatchTimeout(f"{total:g} 秒内未找到文字: {text}")
            if self._stop.wait(min(float(interval), remaining)):
                self._check_open()

    @staticmethod
    def _point(point):
        if not isinstance(point, (tuple, list)) or len(point) != 2 or any(type(v) not in {int, float} or not math.isfinite(v) or not 0 <= v <= 1 for v in point):
            raise ValueError('坐标必须是两个 0 到 1 的有限数值')
        return point

    def tap(self, point, timeout=10):
        x, y = self._point(point)
        deadline = Deadline(timeout)
        return self._session(deadline).action('tap', deadline, x=x, y=y)

    def swipe(self, start, end, duration_ms=300, timeout=10):
        x, y = self._point(start); x2, y2 = self._point(end)
        if type(duration_ms) is not int or not 0 < duration_ms <= 60000:
            raise ValueError('duration_ms 必须是 1 到 60000 的整数')
        deadline = Deadline(timeout)
        return self._session(deadline).action('swipe', deadline, x=x, y=y, x2=x2, y2=y2, duration_ms=duration_ms)

    def key(self, keycode, meta_state=0, timeout=10):
        if any(type(v) is not int or not 0 <= v <= 0xffffffff for v in (keycode, meta_state)):
            raise ValueError('keycode/meta_state 必须为 uint32')
        deadline = Deadline(timeout)
        return self._session(deadline).action('key', deadline, keycode=keycode, meta_state=meta_state)

    def text(self, text, timeout=10):
        if not isinstance(text, str) or not text or len(text.encode('utf-8')) > 300:
            raise ValueError('text 必须为 1 到 300 UTF-8 字节')
        deadline = Deadline(timeout)
        return self._session(deadline).action('text', deadline, text=text)

    def tap_match(self, match, max_age_ms=600, timeout=10):
        from .models import Match
        self._check_open()
        if not isinstance(match, Match) or type(max_age_ms) not in {int, float} or not math.isfinite(max_age_ms) or not 0 < max_age_ms <= 600:
            raise ValueError('match 和 max_age_ms 不合法')
        session = self._vision
        if match.replayed:
            raise SDKError('回放画面匹配结果不能用于点击')
        if session is None or match.device_id != self.serial or match.session_id != session.session_id or match.generation != session.generation or match.connection_id != session.connection_id or match.frame_id <= 0:
            raise SDKError('匹配结果属于其他设备、旧会话或重置前画面')
        age = (time.monotonic() - match.received_at) * 1000
        if age < 0 or age >= max_age_ms:
            raise SDKError('匹配画面已过期')
        if any(type(v) not in {int, float} or not math.isfinite(v) for v in (match.x, match.y, match.w, match.h, match.received_at)) or not (match.x >= 0 and match.y >= 0 and match.w > 0 and match.h > 0 and match.x + match.w <= 1 and match.y + match.h <= 1):
            raise ValueError("匹配框不合法")
        x, y = self._point(match.center)
        deadline = Deadline(min(_seconds(timeout), (max_age_ms - age) / 1000))
        return session.action('tap', deadline, x=x, y=y, frame_id=match.frame_id)

    def close(self):
        with self._lock:
            if self._closed:
                return
            self._closed = True
            self._stop.set()
            if self._watch is not None:
                self._watch.close()
            if self._vision is not None:
                self._vision.close()

    def __enter__(self):
        self._check_open()
        return self

    def __exit__(self, exc_type, exc_value, traceback):
        self.close()
