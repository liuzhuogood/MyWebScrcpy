#!/usr/bin/env python3
"""通用操作：看短视频并执行拟人化随机上滑"""

import argparse
import json
import math
import os
import random
import sys
import time
from request_client import ApiClient


def resolve_base_url(arg_base: str | None) -> str:
    url = arg_base or os.environ.get("MYWEBSCRCPY_BASE_URL") or os.environ.get("MYWEBSCRCPY_API_URL")
    if not url:
        raise ValueError("请指定服务地址 (通过 --base-url 或环境变量 MYWEBSCRCPY_BASE_URL / MYWEBSCRCPY_API_URL)")
    return url.rstrip("/")


def get_current_page(client: ApiClient, serial: str) -> dict:
    res = client.request("GET", "/api/ui/page-info", query=[("serial", serial)])
    if res.status == 200:
        try:
            return json.loads(res.body.decode("utf-8"))
        except Exception:
            return {}
    return {}


def swipe_up(client: ApiClient, serial: str, humanize: bool = True) -> tuple[bool, str]:
    x1 = round(0.50 + random.uniform(-0.06, 0.06), 4)
    y1 = round(0.78 + random.uniform(-0.04, 0.04), 4)
    x2 = round(0.50 + random.uniform(-0.06, 0.06), 4)
    y2 = round(0.22 + random.uniform(-0.04, 0.04), 4)
    duration_ms = random.randint(350, 600)

    payload = {
        "serial": serial,
        "x1": x1,
        "y1": y1,
        "x2": x2,
        "y2": y2,
        "duration_ms": duration_ms,
        "humanize": humanize,
    }
    body = json.dumps(payload).encode("utf-8")
    res = client.request("POST", "/api/v1/actions/swipe", body=body, headers={"Content-Type": "application/json"})
    if res.status != 200:
        return False, f"HTTP {res.status}: {res.body.decode('utf-8', errors='replace')}"
    try:
        data = json.loads(res.body.decode("utf-8"))
    except Exception as e:
        return False, f"解析动作响应失败: {e}"

    if data.get("status") != "executed" or data.get("executed") is not True:
        return False, f"滑动未执行成功: status={data.get('status')}, error_code={data.get('error_code')}"
    return True, ""


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--base", "--base-url", dest="base", default=None, help="服务地址 (或由环境变量 MYWEBSCRCPY_BASE_URL 指定)")
    parser.add_argument("--serial", required=True, help="设备序列号")
    parser.add_argument("--min-stay", type=float, default=5.0, help="单视频最少停留秒数 (默认 5.0)")
    parser.add_argument("--max-stay", type=float, default=15.0, help="单视频最多停留秒数 (默认 15.0)")
    parser.add_argument("--total-duration", type=float, default=60.0, help="总执行时限秒数 (默认 60.0)")
    parser.add_argument("--count", type=int, default=5, help="最多观看并滑动视频数 (默认 5)")
    parser.add_argument("--expected-package", help="预期前台应用包名，若离开则立即停止")
    parser.add_argument("--json", action="store_true", help="以 JSON 格式输出结果")
    args = parser.parse_args()

    try:
        base_url = resolve_base_url(args.base)
    except ValueError as e:
        if args.json:
            print(json.dumps({"error": str(e), "status": "invalid_parameters"}, ensure_ascii=False))
        else:
            print(f"[错误] {e}", file=sys.stderr)
        return 1

    for val, name in [(args.min_stay, "min_stay"), (args.max_stay, "max_stay"), (args.total_duration, "total_duration")]:
        if math.isnan(val) or math.isinf(val):
            msg = f"参数 {name} 不能为 NaN 或 Inf"
            if args.json:
                print(json.dumps({"error": msg, "status": "invalid_parameters"}, ensure_ascii=False))
            else:
                print(f"[错误] {msg}", file=sys.stderr)
            return 1

    if args.min_stay < 0 or args.max_stay < args.min_stay or args.total_duration <= 0 or args.count <= 0:
        msg = "参数不合法: min_stay 必须 >= 0，max_stay 必须 >= min_stay，total_duration 与 count 必须 > 0"
        if args.json:
            print(json.dumps({"error": msg, "status": "invalid_parameters"}, ensure_ascii=False))
        else:
            print(f"[错误] {msg}", file=sys.stderr)
        return 1

    client = ApiClient(base_url)

    # 1. 验证可信前台页面
    initial_page = get_current_page(client, args.serial)
    current_pkg = initial_page.get("package_name", "")
    if not current_pkg:
        msg = "无法获取设备当前前台页面信息，页面状态不可信，中止操作"
        if args.json:
            print(json.dumps({"error": msg, "watched_count": 0, "status": "aborted"}, ensure_ascii=False))
        else:
            print(f"[错误] {msg}", file=sys.stderr)
        return 1

    if args.expected_package and current_pkg != args.expected_package:
        msg = f"当前前台包名 ({current_pkg}) 与预期包名 ({args.expected_package}) 不符，中止看视频"
        if args.json:
            print(json.dumps({"error": msg, "watched_count": 0, "status": "aborted"}, ensure_ascii=False))
        else:
            print(f"[错误] {msg}", file=sys.stderr)
        return 1

    start_time = time.monotonic()
    deadline = start_time + args.total_duration
    watched_count = 0
    records = []
    stopped_reason = "completed"
    all_success = True

    for i in range(args.count):
        now = time.monotonic()
        remaining = deadline - now
        # 剩余预算不足以满足最小停留时间时，不再执行下一次滑动，避免超时
        if remaining < args.min_stay:
            stopped_reason = "total_duration_exceeded"
            break

        stay_time = random.uniform(args.min_stay, min(args.max_stay, max(args.min_stay, remaining)))
        if not args.json:
            print(f"[{i+1}/{args.count}] 视频停留 {stay_time:.1f} 秒...")
        time.sleep(stay_time)

        if time.monotonic() >= deadline:
            stopped_reason = "total_duration_exceeded"
            break

        # 检查前台状态
        cur_page = get_current_page(client, args.serial)
        cur_pkg = cur_page.get("package_name", "")
        if not cur_pkg:
            stopped_reason = "page_lost"
            all_success = False
            break
        if args.expected_package and cur_pkg != args.expected_package:
            stopped_reason = f"package_changed_to_{cur_pkg}"
            if not args.json:
                print(f"[警告] 检测到前台已切换至 {cur_pkg}，交回外部决策")
            all_success = False
            break

        # 执行上滑并严格核对动作执行契约
        swipe_ok, err_msg = swipe_up(client, args.serial, humanize=True)
        if not swipe_ok:
            stopped_reason = f"swipe_action_failed: {err_msg}"
            all_success = False
            break

        watched_count += 1
        record = {
            "index": i + 1,
            "stay_seconds": round(stay_time, 2),
            "swipe_success": True,
            "package_name": cur_pkg,
            "activity": cur_page.get("activity", ""),
        }
        records.append(record)

    total_elapsed = round(time.monotonic() - start_time, 2)
    final_page = get_current_page(client, args.serial)

    result = {
        "serial": args.serial,
        "watched_count": watched_count,
        "total_elapsed_seconds": total_elapsed,
        "stopped_reason": stopped_reason,
        "final_page": {
            "package_name": final_page.get("package_name", ""),
            "activity": final_page.get("activity", ""),
        },
        "records": records,
    }

    if args.json:
        print(json.dumps(result, indent=2, ensure_ascii=False))
    else:
        print("=" * 50)
        print(f"看视频完成：共播放并上滑 {watched_count} 个视频，耗时 {total_elapsed} 秒")
        print(f"结束原因: {stopped_reason}")
        print(f"当前页面: {final_page.get('package_name', '')} / {final_page.get('activity', '')}")
        print("=" * 50)

    return 0 if (all_success and stopped_reason in ("completed", "total_duration_exceeded")) else 1


if __name__ == "__main__":
    sys.exit(main())
