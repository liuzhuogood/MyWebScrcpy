#!/usr/bin/env python3
"""通用操作：停止并重启目标应用，验证实际前台状态"""

import argparse
import json
import math
import os
import sys
import time
from request_client import ApiClient


def resolve_base_url(arg_base: str | None) -> str:
    url = arg_base or os.environ.get("MYWEBSCRCPY_BASE_URL") or os.environ.get("MYWEBSCRCPY_API_URL")
    if not url:
        raise ValueError("请指定服务地址 (通过 --base-url 或环境变量 MYWEBSCRCPY_BASE_URL / MYWEBSCRCPY_API_URL)")
    return url.rstrip("/")


def format_component(package: str, activity: str) -> str:
    if "/" in activity:
        return activity
    return f"{package}/{activity}"


def get_current_page(client: ApiClient, serial: str) -> dict:
    res = client.request("GET", "/api/ui/page-info", query=[("serial", serial)])
    if res.status == 200:
        try:
            return json.loads(res.body.decode("utf-8"))
        except Exception:
            return {}
    return {}


def force_stop_app(client: ApiClient, serial: str, package: str) -> tuple[bool, str]:
    body = json.dumps({"args": ["shell", "am", "force-stop", package]}).encode("utf-8")
    res = client.request("POST", "/api/adb/commands", query=[("serial", serial)], body=body, headers={"Content-Type": "application/json"})
    if res.status != 200:
        return False, f"HTTP {res.status}: {res.body.decode('utf-8', errors='replace')}"
    try:
        data = json.loads(res.body.decode("utf-8"))
    except Exception as e:
        return False, f"解析停止响应失败: {e}"
    if data.get("status") != "completed":
        return False, f"停止命令未完成: status={data.get('status')} (error={data.get('error')})"
    exit_code = data.get("exit_code")
    if exit_code is None or exit_code != 0:
        return False, f"停止命令退出码异常: {exit_code} (stderr={data.get('stderr')})"
    return True, ""


def launch_app(client: ApiClient, serial: str, package: str, activity: str | None = None) -> tuple[bool, str]:
    if activity:
        comp = format_component(package, activity)
        args = ["shell", "am", "start", "-n", comp]
    else:
        args = ["shell", "monkey", "-p", package, "-c", "android.intent.category.LAUNCHER", "1"]

    body = json.dumps({"args": args}).encode("utf-8")
    res = client.request("POST", "/api/adb/commands", query=[("serial", serial)], body=body, headers={"Content-Type": "application/json"})
    if res.status != 200:
        return False, f"HTTP {res.status}: {res.body.decode('utf-8', errors='replace')}"
    try:
        data = json.loads(res.body.decode("utf-8"))
    except Exception as e:
        return False, f"解析启动响应失败: {e}"

    if data.get("status") != "completed":
        return False, f"启动命令未完成: status={data.get('status')} (error={data.get('error')})"
    exit_code = data.get("exit_code")
    if exit_code is None or exit_code != 0:
        return False, f"启动命令退出码异常: {exit_code} (stderr={data.get('stderr')})"
    return True, ""


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--base", "--base-url", dest="base", default=None, help="服务地址 (或由环境变量 MYWEBSCRCPY_BASE_URL 指定)")
    parser.add_argument("--serial", required=True, help="设备序列号")
    parser.add_argument("--package", required=True, help="目标应用包名")
    parser.add_argument("--activity", help="可选目标 Activity")
    parser.add_argument("--timeout", type=float, default=15.0, help="重启验证超时秒数 (默认 15.0)")
    parser.add_argument("--json", action="store_true", help="以 JSON 格式输出")
    args = parser.parse_args()

    try:
        base_url = resolve_base_url(args.base)
    except ValueError as e:
        if args.json:
            print(json.dumps({"success": False, "error": str(e)}, ensure_ascii=False))
        else:
            print(f"[错误] {e}", file=sys.stderr)
        return 1

    if math.isnan(args.timeout) or math.isinf(args.timeout) or args.timeout <= 0:
        msg = "超时必须为有限正数秒"
        if args.json:
            print(json.dumps({"success": False, "error": msg}, ensure_ascii=False))
        else:
            print(f"[错误] {msg}", file=sys.stderr)
        return 1

    client = ApiClient(base_url)

    # 1. 强制停止
    stop_ok, stop_err = force_stop_app(client, args.serial, args.package)
    if not stop_ok:
        msg = f"发送停止指令失败 (package={args.package}): {stop_err}"
        if args.json:
            print(json.dumps({"success": False, "error": msg}, ensure_ascii=False))
        else:
            print(f"[错误] {msg}", file=sys.stderr)
        return 1

    time.sleep(0.6)

    # 2. 重新启动
    launch_ok, launch_err = launch_app(client, args.serial, args.package, args.activity)
    if not launch_ok:
        msg = f"发送重新启动指令失败 (package={args.package}): {launch_err}"
        if args.json:
            print(json.dumps({"success": False, "error": msg}, ensure_ascii=False))
        else:
            print(f"[错误] {msg}", file=sys.stderr)
        return 1

    # 3. 闭环轮询验证前台包名
    start_time = time.monotonic()
    deadline = start_time + args.timeout
    restarted = False
    current_page = {}
    while time.monotonic() < deadline:
        time.sleep(0.5)
        current_page = get_current_page(client, args.serial)
        if current_page.get("package_name") == args.package:
            restarted = True
            break

    elapsed = round(time.monotonic() - start_time, 2)
    result = {
        "serial": args.serial,
        "target_package": args.package,
        "target_activity": args.activity,
        "success": restarted,
        "elapsed_seconds": elapsed,
        "current_page": {
            "package_name": current_page.get("package_name", ""),
            "activity": current_page.get("activity", ""),
        },
    }

    if args.json:
        print(json.dumps(result, indent=2, ensure_ascii=False))
    else:
        if restarted:
            print(f"[成功] 应用 {args.package} 已重启并在前台运行 (耗时 {elapsed}s)")
            print(f"当前页面: {current_page.get('activity', '')}")
        else:
            print(f"[失败] 重启超时 ({args.timeout}s)，前台为: {current_page.get('package_name', '')} / {current_page.get('activity', '')}", file=sys.stderr)

    return 0 if restarted else 1


if __name__ == "__main__":
    sys.exit(main())
