#!/usr/bin/env python3
"""Call the Android location service after discovering its live HTML API docs."""

from __future__ import annotations

import argparse
from html.parser import HTMLParser
import json
import math
import re
import socket
import subprocess
import sys
import time
from typing import Any
from urllib.error import HTTPError, URLError
from urllib.parse import urljoin
from urllib.request import Request, urlopen


DEFAULT_PORT = 8888
ENDPOINT_RE = re.compile(r"\b(GET|POST)\s+(/[^\s<—]+)", re.I)


class TextExtractor(HTMLParser):
    def __init__(self) -> None:
        super().__init__()
        self.parts: list[str] = []

    def handle_data(self, data: str) -> None:
        if data.strip():
            self.parts.append(data.strip())


def fail(message: str, code: int = 2) -> None:
    print(json.dumps({"ok": False, "error": message}, ensure_ascii=False, indent=2), file=sys.stderr)
    raise SystemExit(code)


def run_adb(serial: str | None, *args: str) -> str:
    command = ["adb"] + (["-s", serial] if serial else []) + list(args)
    try:
        result = subprocess.run(command, text=True, capture_output=True, timeout=10, check=False)
    except FileNotFoundError:
        fail("未找到 adb；请安装 Android platform-tools 或显式传入 --base-url")
    except subprocess.TimeoutExpired:
        fail("adb 命令超时")
    if result.returncode != 0:
        fail(f"adb 失败: {result.stderr.strip() or result.stdout.strip()}")
    return result.stdout


def select_device(requested: str | None) -> str:
    devices = []
    for line in run_adb(None, "devices").splitlines()[1:]:
        columns = line.split()
        if len(columns) >= 2 and columns[1] == "device":
            devices.append(columns[0])
    if requested:
        if requested not in devices:
            fail(f"ADB 设备不在线或未授权: {requested}")
        return requested
    if not devices:
        fail("没有在线且已授权的 ADB 设备")
    if len(devices) > 1:
        fail("检测到多台 ADB 设备，请用 --serial 明确目标: " + ", ".join(devices))
    return devices[0]


def adb_ipv4_candidates(serial: str) -> list[str]:
    output = run_adb(serial, "shell", "ip", "-o", "-4", "addr", "show", "scope", "global")
    found: list[str] = []
    for value in re.findall(r"\binet\s+(\d{1,3}(?:\.\d{1,3}){3})/\d+", output):
        if value not in found:
            found.append(value)
    return found


def parse_docs(body: str) -> list[tuple[str, str]]:
    parser = TextExtractor()
    parser.feed(body)
    endpoints: list[tuple[str, str]] = []
    for method, path in ENDPOINT_RE.findall("\n".join(parser.parts)):
        item = (method.upper(), path.rstrip(".,;"))
        if item not in endpoints:
            endpoints.append(item)
    return endpoints


def probe(base_url: str, timeout: float) -> tuple[str, list[tuple[str, str]]]:
    normalized = base_url.rstrip("/") + "/"
    try:
        with urlopen(Request(normalized, headers={"Accept": "text/html"}), timeout=timeout) as response:
            status = response.status
            body = response.read().decode("utf-8", "replace")
    except (HTTPError, URLError, TimeoutError, socket.timeout) as exc:
        raise RuntimeError(str(exc)) from exc
    if status != 200:
        raise RuntimeError(f"根页面 HTTP {status}")
    endpoints = parse_docs(body)
    required = {("POST", "/api/location/set"), ("GET", "/api/location/current")}
    if not required.issubset(set(endpoints)):
        raise RuntimeError("根页面不是预期定位接口文档")
    return normalized.rstrip("/"), endpoints


def local_port_available(port: int) -> bool:
    with socket.socket(socket.AF_INET, socket.SOCK_STREAM) as sock:
        sock.setsockopt(socket.SOL_SOCKET, socket.SO_REUSEADDR, 1)
        try:
            sock.bind(("127.0.0.1", port))
        except OSError:
            return False
    return True


def ensure_forward(serial: str, port: int) -> None:
    local, remote = f"tcp:{port}", f"tcp:{DEFAULT_PORT}"
    for line in run_adb(None, "forward", "--list").splitlines():
        columns = line.split()
        if len(columns) >= 3 and columns[1] == local:
            if columns[0] == serial and columns[2] == remote:
                return
            fail(f"本机 {local} 已映射到其他目标，拒绝覆盖: {line}")
    if not local_port_available(port):
        fail(f"本机 127.0.0.1:{port} 已被占用，无法建立 ADB 转发")
    run_adb(serial, "forward", local, remote)


def resolve_base_url(args: argparse.Namespace) -> tuple[str, str]:
    if args.base_url:
        try:
            base_url, _ = probe(args.base_url, args.timeout)
        except RuntimeError as exc:
            fail(f"无法访问定位服务 {args.base_url}: {exc}")
        return base_url, "explicit"

    serial = select_device(args.serial)
    errors = []
    if not args.adb_forward:
        for address in adb_ipv4_candidates(serial):
            candidate = f"http://{address}:{DEFAULT_PORT}"
            try:
                base_url, _ = probe(candidate, args.timeout)
                return base_url, f"adb-ip:{serial}"
            except RuntimeError as exc:
                errors.append(f"{candidate}: {exc}")

    ensure_forward(serial, args.forward_port)
    candidate = f"http://127.0.0.1:{args.forward_port}"
    try:
        base_url, _ = probe(candidate, args.timeout)
    except RuntimeError as exc:
        detail = "; ".join(errors) if errors else "ADB 未发现全局 IPv4"
        fail(f"手机 IP 直连失败（{detail}）；ADB 转发后仍无法读取文档: {exc}")
    return base_url, f"adb-forward:{serial}"


def api_request(base_url: str, method: str, path: str, timeout: float, payload: dict[str, Any] | None = None) -> dict[str, Any]:
    data = None if payload is None else json.dumps(payload, ensure_ascii=False).encode()
    headers = {"Accept": "application/json"}
    if data is not None:
        headers["Content-Type"] = "application/json"
    request = Request(urljoin(base_url + "/", path.lstrip("/")), data=data, headers=headers, method=method)
    try:
        with urlopen(request, timeout=timeout) as response:
            raw = response.read().decode("utf-8", "replace")
    except HTTPError as exc:
        fail(f"{method} {path} 返回 HTTP {exc.code}: {exc.read().decode('utf-8', 'replace')}")
    except (URLError, TimeoutError, socket.timeout) as exc:
        fail(f"{method} {path} 请求失败: {exc}")
    try:
        body = json.loads(raw)
    except json.JSONDecodeError:
        fail(f"{method} {path} 未返回 JSON: {raw[:300]}")
    if not isinstance(body, dict):
        fail(f"{method} {path} 响应不是 JSON 对象")
    return body


def require_endpoint(endpoints: list[tuple[str, str]], method: str, path: str) -> None:
    if (method, path) not in endpoints:
        fail(f"实时接口文档未声明 {method} {path}，拒绝调用")


def require_success(body: dict[str, Any], operation: str) -> dict[str, Any]:
    if body.get("code") != 0:
        fail(f"{operation} 失败: {json.dumps(body, ensure_ascii=False)}")
    data = body.get("data")
    if not isinstance(data, dict):
        fail(f"{operation} 成功响应缺少文档声明的 data 对象")
    return data


def get_current(base_url: str, endpoints: list[tuple[str, str]], timeout: float) -> dict[str, Any]:
    require_endpoint(endpoints, "GET", "/api/location/current")
    body = api_request(base_url, "GET", "/api/location/current", timeout)
    data = require_success(body, "获取当前位置")
    for field in ("lat", "lng"):
        if not isinstance(data.get(field), (int, float)):
            fail(f"当前位置响应缺少数值字段 {field}")
    return body


def coordinate_verification(args: argparse.Namespace, body: dict[str, Any]) -> dict[str, Any]:
    result: dict[str, Any] = {"read_back": True, "coordinate_checked": False}
    if args.command != "set" or args.lat is None:
        return result
    fields = {"wgs84": ("lat", "lng"), "gcj02": ("gcj02Lat", "gcj02Lng"), "bd09ll": ("bd09Lat", "bd09Lng")}
    coord_type = args.coord_type or "wgs84"
    lat_field, lng_field = fields[coord_type]
    actual_lat, actual_lng = body["data"].get(lat_field), body["data"].get(lng_field)
    if not isinstance(actual_lat, (int, float)) or not isinstance(actual_lng, (int, float)):
        fail(f"当前位置响应缺少 {coord_type} 验证字段 {lat_field}/{lng_field}")
    tolerance_m = args.jitter_meters if args.jitter else 2.0
    north_m = (actual_lat - args.lat) * 111_320
    east_m = (actual_lng - args.lng) * 111_320 * math.cos(math.radians((args.lat + actual_lat) / 2))
    distance_m = math.hypot(north_m, east_m)
    result.update({"coordinate_checked": True, "coordinate_system": coord_type, "distance_m": round(distance_m, 3), "tolerance_m": tolerance_m, "matched": distance_m <= tolerance_m})
    if not result["matched"]:
        fail(f"修改后已回读当前位置，但坐标偏差 {distance_m:.3f}m 超过 {tolerance_m:.3f}m")
    return result


def build_parser() -> argparse.ArgumentParser:
    parser = argparse.ArgumentParser(description="Android 8888 定位服务客户端（调用前实时校验接口文档）")
    parser.add_argument("--base-url", help="例如 http://10.0.0.104:8888；省略时通过 ADB 发现")
    parser.add_argument("--serial", help="ADB serial；多设备或需要端口转发时使用")
    parser.add_argument("--adb-forward", action="store_true", help="强制使用 ADB 将本机端口转发到手机 8888")
    parser.add_argument("--forward-port", type=int, default=8888)
    parser.add_argument("--timeout", type=float, default=5.0)
    sub = parser.add_subparsers(dest="command", required=True)
    sub.add_parser("docs")
    sub.add_parser("current")

    set_parser = sub.add_parser("set")
    target = set_parser.add_mutually_exclusive_group(required=True)
    target.add_argument("--place")
    target.add_argument("--lat", type=float)
    set_parser.add_argument("--lng", type=float)
    set_parser.add_argument("--name")
    set_parser.add_argument("--coord-type", choices=("wgs84", "gcj02", "bd09ll"))
    set_parser.add_argument("--jitter", action="store_true")
    set_parser.add_argument("--jitter-meters", type=float, default=10.0)

    geocode = sub.add_parser("geocode")
    geocode.add_argument("--address", required=True)
    geocode.add_argument("--city")

    direction = sub.add_parser("direction")
    direction.add_argument("--bearing", required=True)
    speed = direction.add_mutually_exclusive_group()
    speed.add_argument("--speed-mps", type=float)
    speed.add_argument("--speed-preset", choices=("walk", "run", "bike"))
    direction.add_argument("--jitter", action="store_true")
    direction.add_argument("--jitter-meters", type=float, default=10.0)

    route = sub.add_parser("route")
    destination = route.add_mutually_exclusive_group(required=True)
    destination.add_argument("--dest-place")
    destination.add_argument("--dest-lat", type=float)
    route.add_argument("--dest-lng", type=float)
    route.add_argument("--dest-name")
    route.add_argument("--coord-type", choices=("wgs84", "gcj02", "bd09ll"))
    route.add_argument("--mode", choices=("walking", "driving"))
    route_speed = route.add_mutually_exclusive_group()
    route_speed.add_argument("--speed-mps", type=float)
    route_speed.add_argument("--speed-preset", choices=("walk", "run", "bike"))
    route.add_argument("--jitter", action="store_true")
    route.add_argument("--jitter-meters", type=float, default=10.0)

    sub.add_parser("task")
    sub.add_parser("cancel")
    sub.add_parser("address")
    sub.add_parser("gyro")
    return parser


def compact(values: dict[str, Any]) -> dict[str, Any]:
    return {key: value for key, value in values.items() if value is not None}


def main() -> None:
    args = build_parser().parse_args()
    if not 1 <= args.forward_port <= 65535 or args.timeout <= 0:
        fail("端口必须在 1~65535，超时必须大于 0")
    if args.command == "set" and ((args.lat is None) != (args.lng is None)):
        fail("坐标定位必须同时提供 --lat 与 --lng")
    if args.command == "route" and ((args.dest_lat is None) != (args.dest_lng is None)):
        fail("路线坐标必须同时提供 --dest-lat 与 --dest-lng")
    if args.command == "set" and args.lat is not None and not (-90 <= args.lat <= 90 and -180 <= args.lng <= 180):
        fail("坐标范围必须为纬度 -90~90、经度 -180~180")
    if args.command == "route" and args.dest_lat is not None and not (-90 <= args.dest_lat <= 90 and -180 <= args.dest_lng <= 180):
        fail("终点坐标范围必须为纬度 -90~90、经度 -180~180")

    base_url, connection = resolve_base_url(args)
    try:
        _, endpoints = probe(base_url, args.timeout)
    except RuntimeError as exc:
        fail(f"读取实时接口文档失败: {exc}")

    if args.command == "docs":
        output = {"ok": True, "base_url": base_url, "connection": connection, "endpoints": [{"method": m, "path": p} for m, p in endpoints]}
    elif args.command == "current":
        output = {"ok": True, "base_url": base_url, "connection": connection, "current": get_current(base_url, endpoints, args.timeout)}
    else:
        simple = {"geocode": ("POST", "/api/geocode"), "task": ("GET", "/api/task"), "address": ("GET", "/api/location/address"), "gyro": ("GET", "/api/sensor/gyro")}
        if args.command in simple:
            method, path = simple[args.command]
            require_endpoint(endpoints, method, path)
            payload = compact({"address": args.address, "city": args.city}) if args.command == "geocode" else None
            response = api_request(base_url, method, path, args.timeout, payload)
            require_success(response, args.command)
            output = {"ok": True, "base_url": base_url, "connection": connection, "response": response}
        else:
            if args.command == "set":
                method, path = "POST", "/api/location/set"
                payload = compact({"lat": args.lat, "lng": args.lng, "place": args.place, "name": args.name, "coordType": args.coord_type, "jitter": True if args.jitter else None, "jitterMeters": args.jitter_meters if args.jitter else None})
            elif args.command == "direction":
                method, path = "POST", "/api/direction"
                payload = compact({"bearing": args.bearing, "speedMps": args.speed_mps, "speedPreset": args.speed_preset, "jitter": True if args.jitter else None, "jitterMeters": args.jitter_meters if args.jitter else None})
            elif args.command == "route":
                method, path = "POST", "/api/route"
                payload = compact({"destLat": args.dest_lat, "destLng": args.dest_lng, "destPlace": args.dest_place, "destName": args.dest_name, "coordType": args.coord_type, "mode": args.mode, "speedMps": args.speed_mps, "speedPreset": args.speed_preset, "jitter": True if args.jitter else None, "jitterMeters": args.jitter_meters if args.jitter else None})
            else:
                method, path, payload = "POST", "/api/task/cancel", None
            require_endpoint(endpoints, method, path)
            mutation = api_request(base_url, method, path, args.timeout, payload)
            require_success(mutation, args.command)
            if args.command in ("direction", "route"):
                time.sleep(1)
            current_body = get_current(base_url, endpoints, args.timeout)
            verification: dict[str, Any] = {"current": current_body, "result": coordinate_verification(args, current_body)}
            if args.command in ("direction", "route", "cancel") and ("GET", "/api/task") in endpoints:
                task_body = api_request(base_url, "GET", "/api/task", args.timeout)
                require_success(task_body, "获取当前任务")
                verification["task"] = task_body
            output = {"ok": True, "base_url": base_url, "connection": connection, "request": {"method": method, "path": path, "payload": payload}, "mutation": mutation, "verification": verification}
    print(json.dumps(output, ensure_ascii=False, indent=2))


if __name__ == "__main__":
    main()
