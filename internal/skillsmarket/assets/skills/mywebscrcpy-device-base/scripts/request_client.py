#!/usr/bin/env python3
"""Generic HTTP transport for the project API skill (Python 3.10+)."""

import argparse
from dataclasses import dataclass
import json
import math
import os
from pathlib import Path
import re
import ssl
import sys
from urllib.error import HTTPError, URLError
from urllib.parse import urlencode, urlsplit
from urllib.request import HTTPRedirectHandler, HTTPSHandler, ProxyHandler, Request, build_opener


@dataclass
class ApiResponse:
    status: int
    headers: dict[str, str]
    body: bytes


class NoRedirect(HTTPRedirectHandler):
    def redirect_request(self, req, fp, code, msg, headers, newurl):
        return None


def validate_header(name: str, value: str) -> None:
    if not re.fullmatch(r"[!#$%&'*+.^_`|~0-9A-Za-z-]+", name):
        raise ValueError("请求头名称不合法")
    if "\r" in value or "\n" in value:
        raise ValueError("请求头值不能包含换行")
    try:
        value.encode("latin-1")
    except UnicodeEncodeError:
        raise ValueError("请求头值必须可编码为 Latin-1") from None


class ApiClient:
    """One request method; no API paths, business models or retries."""

    def __init__(self, base_url: str, *, timeout: float = 30, insecure: bool = False):
        base = urlsplit(base_url)
        if (base.scheme not in {"http", "https"} or not base.hostname
                or base.username is not None or base.password is not None
                or base.query or base.fragment or base.path not in {"", "/"}):
            raise ValueError("服务地址必须是无凭据、查询参数或接口路径的 HTTP(S) 根地址")
        if not math.isfinite(timeout) or timeout <= 0:
            raise ValueError("超时必须为有限正数秒")
        self.base_url = base_url.rstrip("/")
        self.timeout = timeout
        self.auth_headers: dict[str, str] = {}
        token = os.environ.get("MYWEBSCRCPY_API_TOKEN", "")
        if token:
            name = os.environ.get("MYWEBSCRCPY_API_TOKEN_HEADER", "")
            if not name:
                raise ValueError("配置 Token 时必须指定 MYWEBSCRCPY_API_TOKEN_HEADER")
            prefix = os.environ.get("MYWEBSCRCPY_API_TOKEN_PREFIX", "")
            value = f"{prefix} {token}" if prefix else token
            validate_header(name, value)
            self.auth_headers[name] = value
        context = ssl.create_default_context()
        if insecure:
            context.check_hostname = False
            context.verify_mode = ssl.CERT_NONE
            print("[api] 本次请求已关闭 TLS 证书校验", file=sys.stderr)
        self.opener = build_opener(ProxyHandler({}), HTTPSHandler(context=context), NoRedirect())

    def request(self, method: str, path: str, *, query=(), body: bytes | None = None,
                headers: dict[str, str] | None = None) -> ApiResponse:
        parsed = urlsplit(path)
        if (not path.startswith("/") or path.startswith("//") or parsed.scheme
                or parsed.netloc or parsed.query or parsed.fragment):
            raise ValueError("接口路径必须以单个 / 开头，查询参数用 query 单独传入")
        method = method.upper()
        if not re.fullmatch(r"[A-Z]+", method):
            raise ValueError("HTTP 方法不合法")
        request_headers = dict(self.auth_headers)
        for name, value in (headers or {}).items():
            validate_header(name, value)
            if any(name.lower() == key.lower() for key in self.auth_headers):
                raise ValueError("单次请求不能覆盖集中配置的 Token 请求头")
            request_headers[name] = value
        url = self.base_url + path
        encoded = urlencode(query, doseq=True)
        if encoded:
            url += "?" + encoded
        request = Request(url, data=body, headers=request_headers, method=method)
        try:
            response = self.opener.open(request, timeout=self.timeout)
        except HTTPError as error:
            response = error
        with response:
            return ApiResponse(response.status, dict(response.headers.items()), response.read())


def key_value(value: str) -> tuple[str, str]:
    key, separator, content = value.partition("=")
    if not separator or not key:
        raise argparse.ArgumentTypeError("请使用 名称=值")
    return key, content


def read_body(filename: str) -> bytes:
    return sys.stdin.buffer.read() if filename == "-" else Path(filename).read_bytes()


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("path", help="从当前 OpenAPI 确认的接口路径")
    parser.add_argument("--base-url", default=os.environ.get("MYWEBSCRCPY_API_URL"))
    parser.add_argument("--method", default="GET")
    parser.add_argument("--query", type=key_value, action="append", default=[])
    parser.add_argument("--header", type=key_value, action="append", default=[])
    bodies = parser.add_mutually_exclusive_group()
    bodies.add_argument("--json-file", help="JSON 请求文件；- 表示标准输入")
    bodies.add_argument("--body-file", help="原始请求文件；- 表示标准输入")
    parser.add_argument("--content-type", help="原始请求体的媒体类型")
    parser.add_argument("--timeout", type=float, default=30)
    parser.add_argument("--insecure", action="store_true", help="仅在已获授权且网络可信时关闭 TLS 校验")
    parser.add_argument("--output", type=Path, help="将原始响应体保存到文件")
    options = parser.parse_args()
    if not options.base_url:
        parser.error("请配置 --base-url 或 MYWEBSCRCPY_API_URL")
    if options.body_file and not options.content_type:
        parser.error("--body-file 需要 --content-type")
    if options.json_file and options.content_type:
        parser.error("--json-file 固定使用 application/json，无需 --content-type")
    try:
        headers = dict(options.header)
        if options.json_file:
            body = read_body(options.json_file)
            json.loads(body)
            headers["Content-Type"] = "application/json"
        elif options.body_file:
            body = read_body(options.body_file)
            headers["Content-Type"] = options.content_type
        else:
            body = None
        client = ApiClient(options.base_url, timeout=options.timeout, insecure=options.insecure)
        response = client.request(options.method, options.path, query=options.query,
                                  body=body, headers=headers)
        if options.output:
            options.output.write_bytes(response.body)
        else:
            sys.stdout.buffer.write(response.body)
        if not 200 <= response.status < 300:
            print(f"[api] HTTP {response.status}；原始响应体已输出", file=sys.stderr)
            return 1
        return 0
    except (URLError, OSError, ValueError, UnicodeError) as error:
        # Avoid printing request headers or exceptions that could include a Token.
        print(f"[api] 请求失败：{type(error).__name__}", file=sys.stderr)
        if isinstance(error, ValueError) and not isinstance(error, json.JSONDecodeError):
            print(str(error), file=sys.stderr)
        return 1


if __name__ == "__main__":
    sys.exit(main())
