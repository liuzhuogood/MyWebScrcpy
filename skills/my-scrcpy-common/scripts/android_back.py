#!/usr/bin/env python3
"""通用操作：发送 KEYCODE_BACK (4) 原生返回键"""

import argparse
import json
import os
import sys
import time
from request_client import ApiClient


def resolve_base_url(arg_base: str | None) -> str:
    url = arg_base or os.environ.get("MYWEBSCRCPY_BASE_URL") or os.environ.get("MYWEBSCRCPY_API_URL")
    if not url:
        raise ValueError("请指定服务地址 (通过 --base-url 或环境变量 MYWEBSCRCPY_BASE_URL / MYWEBSCRCPY_API_URL)")
    return url.rstrip("/")


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--base", "--base-url", dest="base", default=None, help="服务地址 (或由环境变量 MYWEBSCRCPY_BASE_URL 指定)")
    parser.add_argument("--serial", required=True, help="设备序列号")
    parser.add_argument("--verify", action="store_true", help="发送后验证前台页面信息")
    parser.add_argument("--json", action="store_true", help="以 JSON 格式输出")
    args = parser.parse_args()

    try:
        base_url = resolve_base_url(args.base)
    except ValueError as e:
        if args.json:
            print(json.dumps({"error": str(e), "success": False}, ensure_ascii=False))
        else:
            print(f"[错误] {e}", file=sys.stderr)
        return 1

    client = ApiClient(base_url)

    payload = {
        "serial": args.serial,
        "keycode": 4,  # KEYCODE_BACK
    }
    body = json.dumps(payload).encode("utf-8")
    res = client.request("POST", "/api/v1/actions/send-key", body=body, headers={"Content-Type": "application/json"})

    if res.status != 200:
        err_msg = f"发送 KEYCODE_BACK 失败: HTTP {res.status}: {res.body.decode('utf-8', errors='replace')}"
        if args.json:
            print(json.dumps({"error": err_msg, "success": False}, ensure_ascii=False))
        else:
            print(f"[错误] {err_msg}", file=sys.stderr)
        return 1

    try:
        data = json.loads(res.body.decode("utf-8"))
    except Exception as e:
        err_msg = f"解析按键响应失败: {e}"
        if args.json:
            print(json.dumps({"error": err_msg, "success": False}, ensure_ascii=False))
        else:
            print(f"[错误] {err_msg}", file=sys.stderr)
        return 1

    if data.get("status") != "executed" or data.get("executed") is not True:
        err_msg = f"按键未执行成功: status={data.get('status')}"
        if args.json:
            print(json.dumps({"error": err_msg, "success": False}, ensure_ascii=False))
        else:
            print(f"[错误] {err_msg}", file=sys.stderr)
        return 1

    post_page = {}
    if args.verify:
        time.sleep(0.5)
        page_res = client.request("GET", "/api/ui/page-info", query=[("serial", args.serial)])
        if page_res.status == 200:
            try:
                post_page = json.loads(page_res.body.decode("utf-8"))
            except Exception:
                post_page = {}

    result = {
        "serial": args.serial,
        "action": "KEYCODE_BACK",
        "keycode": 4,
        "success": True,
        "post_page": {
            "package_name": post_page.get("package_name", ""),
            "activity": post_page.get("activity", ""),
        } if args.verify else None,
    }

    if args.json:
        print(json.dumps(result, indent=2, ensure_ascii=False))
    else:
        print(f"[成功] 已向设备 {args.serial} 发送 KEYCODE_BACK")
        if args.verify and post_page:
            print(f"当前前台页面: {post_page.get('package_name', '')} / {post_page.get('activity', '')}")

    return 0


if __name__ == "__main__":
    sys.exit(main())
