#!/usr/bin/env python3
"""观察设备状态或获取 AI 视觉决策建议"""

import argparse
import json
import os
import sys
from request_client import ApiClient


def resolve_base_url(arg_base: str | None) -> str:
    url = arg_base or os.environ.get("MYWEBSCRCPY_BASE_URL") or os.environ.get("MYWEBSCRCPY_API_URL")
    if not url:
        raise ValueError("请指定服务地址 (通过 --base-url 或环境变量 MYWEBSCRCPY_BASE_URL / MYWEBSCRCPY_API_URL)")
    return url.rstrip("/")


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--base", "--base-url", dest="base", default=None, help="服务地址 (或由环境变量 MYWEBSCRCPY_BASE_URL 指定)")
    parser.add_argument("--serial", required=True, help="设备序列号")
    parser.add_argument("--template-ids", help="指定模板ID（逗号分隔）")
    parser.add_argument("--decision", action="store_true", help="调用 AI 视觉决策模型")
    parser.add_argument("--goal", help="本轮决策目标（仅在 --decision 时生效）")
    parser.add_argument("--json", action="store_true", help="输出完整原始 JSON")
    args = parser.parse_args()

    try:
        base_url = resolve_base_url(args.base)
    except ValueError as e:
        if args.json:
            print(json.dumps({"error": str(e)}, ensure_ascii=False))
        else:
            print(f"[错误] {e}", file=sys.stderr)
        return 1

    client = ApiClient(base_url)

    if not args.decision:
        query = []
        if args.template_ids:
            query.append(("template_ids", args.template_ids))
        res = client.request("GET", f"/api/device-state/{args.serial}", query=query)
    else:
        body_dict = {}
        if args.template_ids:
            body_dict["template_ids"] = args.template_ids
        if args.goal:
            body_dict["goal"] = args.goal
        body_bytes = json.dumps(body_dict).encode("utf-8")
        res = client.request("POST", f"/api/device-state/{args.serial}/decision", body=body_bytes, headers={"Content-Type": "application/json"})

    if res.status != 200:
        err_msg = f"HTTP {res.status}: {res.body.decode('utf-8', errors='replace')}"
        if args.json:
            print(json.dumps({"error": err_msg}, ensure_ascii=False))
        else:
            print(f"[错误] 请求失败: {err_msg}", file=sys.stderr)
        return 1

    try:
        data = json.loads(res.body.decode("utf-8"))
    except Exception as e:
        err_msg = f"解析响应 JSON 失败: {e}"
        if args.json:
            print(json.dumps({"error": err_msg}, ensure_ascii=False))
        else:
            print(f"[错误] {err_msg}", file=sys.stderr)
        return 1

    if args.json:
        print(json.dumps(data, indent=2, ensure_ascii=False))
        return 0

    if args.decision:
        state = data.get("device_state", {})
        dec = data.get("decision", {})
    else:
        state = data
        dec = None

    print("=" * 50)
    print(f"观测标识: {state.get('observation_id')}")
    print(f"设备序列号: {state.get('serial')}")
    page_info = state.get("page_info") or {}
    print(f"前台应用: {page_info.get('package_name', '')} / {page_info.get('activity', '')}")
    print(f"观测稳定性: {'稳定' if state.get('context_stable') else '不稳定'}")

    ocr_data = state.get("ocr", {})
    if ocr_data.get("boxes"):
        print(f"OCR 提取文字 ({len(ocr_data['boxes'])} 项): {ocr_data.get('text', '')[:100]}...")

    matches = state.get("template_matches", [])
    if matches:
        print(f"模板匹配 ({len(matches)} 项):")
        for m in matches:
            print(f"  - [{m.get('source')}] {m.get('template_name')} (置信度: {m.get('confidence', 0):.2f}) 坐标: [{m.get('pixel_x')}, {m.get('pixel_y')}]")

    if dec is not None:
        print("-" * 50)
        print("【AI 视觉决策结果】")
        if dec.get("available"):
            print(f"画面摘要: {dec.get('page_summary')}")
            print(f"可见控件: {', '.join(dec.get('visible_controls_and_dialogs', []))}")
            print(f"进展评估: {dec.get('task_progress_assessment')}")
            print(f"下一步建议: {dec.get('next_step_suggestion')}")
            print(f"判断依据: {dec.get('rationale')}")
            if dec.get("uncertainties"):
                print(f"不确定项: {', '.join(dec.get('uncertainties'))}")
        else:
            print(f"决策不可用: {dec.get('error')}")
    print("=" * 50)
    return 0


if __name__ == "__main__":
    sys.exit(main())
