import logging
from urllib.parse import urlsplit

try:
    import httpx
except ImportError as error:
    raise ImportError("请在当前 Python 环境安装 mywebscrcpy-sdk，或安装 httpx") from error

from .errors import ConnectionError, HTTPError, ProtocolError, SDKError

logger = logging.getLogger(__name__)


class Client:
    """HTTP client. No device process, service or Vision session starts implicitly."""

    def __init__(self, base_url: str, *, headers: dict[str, str] | None = None,
                 timeout: float = 30, verify: bool | str = True):
        parsed = urlsplit(base_url)
        if parsed.scheme not in {"http", "https"} or not parsed.netloc or parsed.username or parsed.password or parsed.query or parsed.fragment:
            raise ValueError("base_url 必须是无凭据的 HTTP(S) 服务地址")
        if parsed.path not in {"", "/"}:
            raise ValueError("base_url 不能包含接口路径")
        if timeout <= 0:
            raise ValueError("timeout 必须为正数秒")
        if verify is False:
            logger.warning("TLS certificate verification explicitly disabled")
        self._http = httpx.Client(base_url=base_url.rstrip("/"), headers=headers,
                                  timeout=timeout, verify=verify, trust_env=False)
        self._closed = False

    def device(self, serial: str):
        from .device import Device
        self._check_open()
        return Device(self, serial)

    def _check_open(self):
        if self._closed:
            raise SDKError("Client 已关闭")

    def _request(self, method: str, path: str, *, serial: str, timeout: float, body=None):
        self._check_open()
        try:
            response = self._http.request(method, path, params={"serial": serial}, json=body, timeout=timeout)
        except httpx.RequestError as error:
            logger.warning("HTTP connection failed: method=%s path=%s type=%s", method, path, type(error).__name__)
            raise ConnectionError(f"{method} {path}: {type(error).__name__}") from error
        try:
            value = response.json()
        except ValueError as error:
            raise ProtocolError(f"{method} {path}: HTTP {response.status_code} 返回非 JSON") from error
        if not isinstance(value, dict):
            raise ProtocolError("API 响应必须是 JSON 对象")
        if response.is_error:
            # ErrorResponse's message is optional; code is always taken from error.
            code = value.get("error", "")
            detail = value.get("message", "")
            if not isinstance(code, str) or not isinstance(detail, str):
                raise ProtocolError("错误响应字段类型不合法")
            raise HTTPError(response.status_code, code, detail)
        return value

    def close(self):
        if not self._closed:
            self._closed = True
            self._http.close()

    def __enter__(self):
        self._check_open()
        return self

    def __exit__(self, exc_type, exc_value, traceback):
        self.close()
