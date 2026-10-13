"""Exact wire fields and monotonic call deadlines, shared by SDK modules."""
import math
import time

from .errors import ProtocolError


def field(value, name, kind, default=...):
    try:
        result = value[name] if default is ... else value.get(name, default)
        valid = type(result) in {int, float} and math.isfinite(result) if kind is float else type(result) is kind
        if not valid:
            raise ValueError(name)
        return result
    except (KeyError, TypeError, ValueError, AttributeError) as error:
        raise ProtocolError(f"缺少有效字段：{name}") from error


def seconds(value):
    if type(value) not in {int, float} or not math.isfinite(value) or not 0 < value <= 600:
        raise ValueError("timeout 必须在 0 到 600 秒之间")
    return value


class Deadline:
    def __init__(self, timeout):
        self.end = time.monotonic() + seconds(timeout)

    def remaining(self, error=TimeoutError):
        remaining = self.end - time.monotonic()
        if remaining <= 0:
            raise error("调用总期限已到")
        return remaining
