import logging
import math

from .errors import CommandError, ProtocolError, SDKError
from .models import CommandResult, PageInfo

logger = logging.getLogger(__name__)


def _seconds(value):
    if type(value) not in {int, float} or not math.isfinite(value) or not 0 < value <= 600:
        raise ValueError("timeout 必须在 0 到 600 秒之间")
    return value


class Device:
    """Basic HTTP operations. Vision and matching APIs are added separately."""

    def __init__(self, client, serial: str):
        if not isinstance(serial, str) or not serial.strip():
            raise ValueError("serial 必须为非空字符串")
        self._client = client
        self.serial = serial
        self._closed = False

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

    def close(self):
        self._closed = True

    def __enter__(self):
        self._check_open()
        return self

    def __exit__(self, exc_type, exc_value, traceback):
        self.close()
