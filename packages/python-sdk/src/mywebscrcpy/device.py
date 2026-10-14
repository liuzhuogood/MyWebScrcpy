import logging
import math
import threading
import time

from .contracts import Deadline
from .errors import ActionTimeout, FrameTimeout, MatchTimeout
from .templates import TemplateCache

from .errors import CommandError, ProtocolError, SDKError
from .models import CommandResult, PageInfo

logger = logging.getLogger(__name__)


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
