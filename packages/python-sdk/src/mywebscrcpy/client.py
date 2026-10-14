import logging
import weakref
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
        self._base_url = base_url.rstrip("/")
        self._headers = dict(headers or {})
        self._verify = verify
        self._devices = weakref.WeakSet()
        from .templates import Templates, Devices
        self.templates = Templates(self)
        self.devices = Devices(self)

    def device(self, serial: str):
        from .device import Device
        self._check_open()
        device = Device(self, serial)
        self._devices.add(device)
        return device

    def _check_open(self):
        if self._closed:
            raise SDKError("Client 已关闭")

    def _request(self, method: str, path: str, *, serial: str, timeout: float, body=None):
        return self._json(method, path, params={"serial": serial}, body=body, timeout=timeout)

    def _response(self, method, path, *, params=None, timeout=30, body=None, files=None, data=None):
        from .contracts import Deadline
        self._check_open()
        deadline = Deadline(timeout)
        try:
            response = self._http.request(method, path, params=params, json=body, files=files, data=data, timeout=deadline.remaining())
        except httpx.RequestError as error:
            logger.warning("HTTP connection failed: method=%s path=%s type=%s", method, path, type(error).__name__)
            raise ConnectionError(f"{method} {path}: {type(error).__name__}") from error
        deadline.remaining(ConnectionError)
        if response.is_error:
            value = self._decode(response)
            if not isinstance(value, dict):
                raise ProtocolError("错误响应必须为对象")
            code, detail = value.get("error", ""), value.get("message", "")
            if not isinstance(code, str) or not isinstance(detail, str):
                raise ProtocolError("错误响应字段类型不合法")
            raise HTTPError(response.status_code, code, detail)
        return response

    @staticmethod
    def _decode(response):
        try:
            return response.json()
        except ValueError as error:
            raise ProtocolError(f"HTTP {response.status_code} 返回非 JSON") from error

    def _json(self, method, path, **options):
        return self._decode(self._response(method, path, **options))

    def close(self):
        if not self._closed:
            self._closed = True
            for device in list(self._devices):
                device.close()
            self._http.close()

    def __enter__(self):
        self._check_open()
        return self

    def __exit__(self, exc_type, exc_value, traceback):
        self.close()
