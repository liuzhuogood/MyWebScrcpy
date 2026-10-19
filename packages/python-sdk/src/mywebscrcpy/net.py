"""SDK 的代理通道配置。

SDK 默认不使用任何代理：既有系统代理与 `HTTP_PROXY`/`HTTPS_PROXY` 环境变量都不会被采用，
这与历史行为一致。需要代理时显式传 `proxy=`，或设置环境变量 `MYWEBSCRCPY_PROXY`：

- `None`（默认）：不使用代理，各通道显式屏蔽环境代理。
- `"env"`：交给运行环境决定（HTTPX 读取环境变量，urllib 使用默认 ProxyHandler）。
- 代理地址（如 `http://127.0.0.1:7890`）：HTTPX、Vision WebSocket 与 OCR 上传都经该代理。

典型用途：macOS 按代码签名授予本地网络访问权，adhoc 签名的 Python 直连内网会得到
`OSError: [Errno 65]`；此时把内网流量交给本机已获授权的代理进程即可恢复。
"""
from __future__ import annotations

import os
import urllib.request
from urllib.parse import unquote, urlsplit

ENV_VAR = "MYWEBSCRCPY_PROXY"
ENV_MODE = "env"


def resolve_proxy(explicit: str | None = None) -> str | None:
    """把显式配置或环境变量归一为 None、`env` 或代理地址。"""
    value = explicit if explicit is not None else os.environ.get(ENV_VAR)
    if value is None:
        return None
    value = value.strip()
    if not value:
        return None
    if value.casefold() == ENV_MODE:
        return ENV_MODE
    parts = urlsplit(value)
    if parts.scheme not in {"http", "https"} or not parts.hostname or parts.port is None:
        raise ValueError("代理地址必须是带端口的 http(s) 地址，例如 http://127.0.0.1:7890")
    return value


def httpx_options(proxy: str | None) -> dict:
    """HTTPX 客户端参数：不使用代理时显式屏蔽环境代理。"""
    if proxy is None:
        return {"trust_env": False}
    if proxy == ENV_MODE:
        return {"trust_env": True}
    return {"proxy": proxy, "trust_env": False}


def websocket_options(proxy: str | None) -> dict:
    """websocket-client 的连接参数；wss 由该库自行建立 CONNECT 隧道。"""
    if proxy is None:
        return {"http_no_proxy": ["*"]}
    if proxy == ENV_MODE:
        return {}
    parts = urlsplit(proxy)
    options: dict = {"http_proxy_host": parts.hostname, "http_proxy_port": parts.port}
    if parts.username is not None:
        options["http_proxy_auth"] = (unquote(parts.username), unquote(parts.password or ""))
    return options


def urllib_opener(proxy: str | None) -> urllib.request.OpenerDirector:
    """urllib 请求器：不使用代理时显式屏蔽环境代理。"""
    if proxy is None:
        return urllib.request.build_opener(urllib.request.ProxyHandler({}))
    if proxy == ENV_MODE:
        return urllib.request.build_opener()
    return urllib.request.build_opener(urllib.request.ProxyHandler({"http": proxy, "https": proxy}))
