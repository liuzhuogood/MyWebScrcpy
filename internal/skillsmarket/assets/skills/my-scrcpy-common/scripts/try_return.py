#!/usr/bin/env python3
"""通用操作：发送 KEYCODE_ESCAPE (111) 尝试返回并对比前后状态"""

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


def get_state(client: ApiClient, serial: str, task_id: str | None = None) -> dict:
    query = []
    if task_id:
        query.append(("task_id", task_id))
    res = client.request("GET", f"/api/device-state/{serial}", query=query)
    if res.status == 200:
        try:
            return json.loads(res.body.decode("utf-8"))
        except Exception:
            return {}
    return {}


def send_escape(client: ApiClient, serial: str) -> tuple[bool, str]:
    payload = {
        "serial": serial,
        "keycode": 111,  # KEYCODE_ESCAPE
    }
    body = json.dumps(payload).encode("utf-8")
    res = client.request("POST", "/api/v1/actions/send-key", body=body, headers={"Content-Type": "application/json"})
    if res.status != 200:
        return False, f"HTTP {res.status}: {res.body.decode('utf-8', errors='replace')}"
    try:
        data = json.loads(res.body.decode("utf-8"))
    except Exception as e:
        return False, f"解析按键响应失败: {e}"

    if data.get("status") != "executed" or data.get("executed") is not True:
        return False, f"按键未执行成功: status={data.get('status')}"
    return True, ""


def analyze_diff(pre: dict, post: dict) -> tuple[str, str]:
    """对比前后状态，返回 (status, summary)。status 可选: changed / unchanged / unknown"""
    if not pre or not post or not pre.get("observation_id") or not post.get("observation_id"):
        return "unknown", "前后状态获取不完整，无法精确判断"

    pre_page = pre.get("page_info") or {}
    post_page = post.get("page_info") or {}

    pre_pkg = pre_page.get("package_name", "")
    post_pkg = post_page.get("package_name", "")
    pre_act = pre_page.get("activity", "")
    post_act = post_page.get("activity", "")
    pre_win = pre_page.get("window_name", "")
    post_win = post_page.get("window_name", "")

    if pre_pkg != post_pkg:
        return "changed", f"应用包名变更: {pre_pkg} -> {post_pkg}"

    if pre_act != post_act:
        return "changed", f"页面 Activity 变更: {pre_act} -> {post_act}"

    if pre_win and post_win and pre_win != post_win:
        return "changed", f"窗口名称变更: {pre_win} -> {post_win}"

    # 严禁单靠视频字幕、弹幕、轮播时间戳等 OCR 文本波动误判返回成功
    return "unchanged", f"包名 ({pre_pkg})、Activity ({pre_act}) 与窗口标识均未变化，未确认退出当前页面"


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--base", "--base-url", dest="base", default=None, help="服务地址 (或由环境变量 MYWEBSCRCPY_BASE_URL 指定)")
    parser.add_argument("--serial", required=True, help="设备序列号")
    parser.add_argument("--task-id", help="关联任务ID")
    parser.add_argument("--wait-seconds", type=float, default=0.8, help="按键后等待页面渲染秒数 (默认 0.8)")
    parser.add_argument("--json", action="store_true", help="以 JSON 格式输出对比结果")
    args = parser.parse_args()

    try:
        base_url = resolve_base_url(args.base)
    except ValueError as e:
        if args.json:
            print(json.dumps({"error": str(e), "status": "action_failed"}, ensure_ascii=False))
        else:
            print(f"[错误] {e}", file=sys.stderr)
        return 1

    if math.isnan(args.wait_seconds) or math.isinf(args.wait_seconds) or args.wait_seconds < 0:
        msg = "wait-seconds 必须为有限非负数秒"
        if args.json:
            print(json.dumps({"error": msg, "status": "action_failed"}, ensure_ascii=False))
        else:
            print(f"[错误] {msg}", file=sys.stderr)
        return 1

    client = ApiClient(base_url)

    # 1. 动作前只读观测：前置状态失败则禁止发键
    pre_state = get_state(client, args.serial, args.task_id)
    if not pre_state or not pre_state.get("observation_id"):
        msg = "获取前置设备状态失败或观测无效，禁止发送按键"
        if args.json:
            print(json.dumps({"error": msg, "status": "pre_state_failed"}, ensure_ascii=False))
        else:
            print(f"[错误] {msg}", file=sys.stderr)
        return 1

    # 2. 发送 KEYCODE_ESCAPE 并校验执行契约
    action_ok, err_msg = send_escape(client, args.serial)
    if not action_ok:
        msg = f"发送 KEYCODE_ESCAPE 失败: {err_msg}"
        if args.json:
            print(json.dumps({"error": msg, "status": "action_failed"}, ensure_ascii=False))
        else:
            print(f"[错误] {msg}", file=sys.stderr)
        return 1

    time.sleep(args.wait_seconds)

    # 3. 动作后重新观测
    post_state = get_state(client, args.serial, args.task_id)

    # 4. 前后业务对比
    status, diff_summary = analyze_diff(pre_state, post_state)

    result = {
        "serial": args.serial,
        "action": "KEYCODE_ESCAPE",
        "result_status": status,
        "diff_summary": diff_summary,
        "pre_observation": {
            "observation_id": pre_state.get("observation_id"),
            "package_name": pre_state.get("page_info", {}).get("package_name") if pre_state.get("page_info") else "",
            "activity": pre_state.get("page_info", {}).get("activity") if pre_state.get("page_info") else "",
        },
        "post_observation": {
            "observation_id": post_state.get("observation_id") if post_state else "",
            "package_name": post_state.get("page_info", {}).get("package_name") if post_state and post_state.get("page_info") else "",
            "activity": post_state.get("page_info", {}).get("activity") if post_state and post_state.get("page_info") else "",
        },
    }

    if args.json:
        print(json.dumps(result, indent=2, ensure_ascii=False))
    else:
        print("=" * 50)
        print(f"尝试返回结果: {status} ({diff_summary})")
        print(f"前置页面: {result['pre_observation']['package_name']} / {result['pre_observation']['activity']}")
        print(f"后置页面: {result['post_observation']['package_name']} / {result['post_observation']['activity']}")
        print("=" * 50)

    # 严格区分返回退出码：changed=0，unchanged=2，unknown/error=1，不伪造成功
    if status == "changed":
        return 0
    elif status == "unchanged":
        return 2
    return 1


if __name__ == "__main__":
    sys.exit(main())
