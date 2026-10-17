#!/usr/bin/env python3
"""Claim the pending watch-video coins from Kuaishou's task center."""
import json
import os
import re
import subprocess
import time
from pathlib import Path
from urllib.parse import urlencode

from mywebscrcpy import Client, TaskReporter

from refresh_hour import PACKAGE, Runner


def curl(base, method, path, *, params=None, body=None, timeout=30):
    url = base.rstrip("/") + path
    if params:
        url += "?" + urlencode(params)
    cmd = ["curl", "--fail-with-body", "--silent", "--show-error", "--max-time", str(timeout), "-X", method, url]
    if body is not None:
        cmd += ["-H", "Content-Type: application/json", "--data-binary", json.dumps(body, ensure_ascii=False)]
    result = subprocess.run(cmd, capture_output=True, text=True, check=False)
    if result.returncode:
        raise RuntimeError(f"{method} {path} 请求失败：{result.stderr.strip()}")
    return json.loads(result.stdout)


def reward_button(ocr):
    boxes = ocr.get("boxes", [])
    buttons = [b for b in boxes if b["text"].strip() == "领金币"]
    if len(buttons) != 1:
        raise RuntimeError(f"当前画面的领金币按钮不唯一，数量={len(buttons)}")
    button = buttons[0]
    contexts = [b for b in boxes if ("待领取" in b["text"] or "继续看视频得金币" in b["text"])
                and b["x"] < button["x"] and abs((b["y"] + b["h"] / 2) - (button["y"] + button["h"] / 2)) <= .08]
    if not contexts:
        raise RuntimeError("领金币按钮附近未识别到观看视频待领取奖励，拒绝点击其他任务按钮")
    return button


def is_task_center(state):
    text = state["ocr"].get("text", "")
    if state["page"]["package_name"] == PACKAGE and "任务中心" in text:
        return True
    has_earnings_header = "任务中心" in text or "我的金币" in text or "金币：" in text or "金币:" in text
    has_task_list = any(label in text for label in ("连续签到", "连续打卡白拿手机", "日常任务"))
    return state["page"]["package_name"] == PACKAGE and has_earnings_header and has_task_list


def read_balance(rt, state):
    try:
        return rt.coin_total(state["ocr"])
    except RuntimeError:
        pass
    boxes = state["ocr"].get("boxes", [])
    combined = [re.fullmatch(r"金币[:：]\s*([\d,]+)", b["text"].strip()) for b in boxes]
    combined = [m for m in combined if m]
    if len(combined) == 1:
        return int(combined[0].group(1).replace(",", ""))
    # Confirmed collapsed task-center header: “金币: 7879”.
    labels = [b for b in boxes if b["text"].strip() in {"金币:", "金币："} and b["y"] < .12]
    if len(labels) != 1:
        raise RuntimeError("收益页未能唯一识别‘我的金币’或已确认的折叠金币栏")
    label = labels[0]
    amounts = [b for b in boxes if re.fullmatch(r"[\d,]+", b["text"].strip())
               and b["x"] > label["x"] and abs((b["y"] + b["h"] / 2) - (label["y"] + label["h"] / 2)) <= .035]
    if len(amounts) != 1:
        raise RuntimeError(f"折叠金币栏右侧余额不唯一，数量={len(amounts)}")
    return int(amounts[0]["text"].replace(",", ""))


def main():
    base, serial = os.environ["MYWEBSCRCPY_API_URL"], os.environ["MYWEBSCRCPY_SERIAL"]
    root = Path(os.environ["MYWEBSCRCPY_RUN_DIR"]).resolve()
    run = TaskReporter(base, name="快手领取观看视频金币", device_serial=serial, source="codex")
    result, failure = None, None
    with run, run.capture_output(), Client(base) as client:
        try:
            schema = curl(base, "GET", "/api/openapi.json")
            for path, method in (("/api/ui/page-info", "get"), ("/api/vision/ocr-text-art", "post"),
                                 ("/api/v1/vision/region-screenshot", "post")):
                if method not in schema["paths"].get(path, {}):
                    raise RuntimeError(f"当前OpenAPI未提供必需接口：{method.upper()} {path}")
            if not any(d.serial == serial and d.state == "device" for d in client.devices.list()):
                raise RuntimeError("目标设备当前未处于device状态")
            with client.device(serial) as device:
                rt = Runner(base, serial, root, device, __import__("random").Random())
                state = rt.state_retry()
                if state["page"]["package_name"] != PACKAGE:
                    raise RuntimeError("当前前台不是快手极速版")
                if not is_task_center(state):
                    state = rt.open_task_center()
                if not is_task_center(state):
                    raise RuntimeError("未确认进入任务中心")
                before_coins = None
                try:
                    before_coins = read_balance(rt, state)
                except RuntimeError as error:
                    rt.log(f"点击前暂时无法读取余额：{error}")
                already_received = ("任务完成奖励" in state["ocr"].get("text", "")
                                    and "到账可能存在延迟" in state["ocr"].get("text", "")
                                    and "待领取" not in state["ocr"].get("text", ""))
                screenshot = rt.screenshot("before-claim.png")
                (root / "before-claim-ocr.json").write_text(
                    json.dumps(state["ocr"], ensure_ascii=False, indent=2), encoding="utf-8")
                if already_received:
                    rt.log("当前页面显示观看奖励已领取、金币到账可能延迟；不重复点击，进入结果复核")
                else:
                    try:
                        button = reward_button(state["ocr"])
                    except RuntimeError:
                        rt.log("观看视频奖励卡片当前不可见，向上滚动任务中心列表后重新识别")
                        device.swipe((.5, .35), (.5, .82), duration_ms=550, timeout=8)
                        time.sleep(2)
                        state = rt.state_retry()
                        if not is_task_center(state):
                            raise RuntimeError("滚动后页面已离开任务中心")
                        button = reward_button(state["ocr"])
                    state = rt.state_retry()
                    if not is_task_center(state):
                        raise RuntimeError("点击前复核时已离开任务中心")
                    button = reward_button(state["ocr"])
                    point = (button["x"] + button["w"] / 2, button["y"] + button["h"] / 2)
                    rt.log(f"确认观看视频待领取卡片内唯一按钮，点击领金币：{point}")
                    device.tap(point, timeout=8)
                deadline = time.monotonic() + 30
                after, after_coins = None, None
                while time.monotonic() < deadline:
                    time.sleep(3)
                    after = rt.state_retry()
                    receipt = ("任务完成奖励" in after["ocr"].get("text", "")
                               and "到账可能存在延迟" in after["ocr"].get("text", ""))
                    if not is_task_center(after) and not receipt:
                        raise RuntimeError("点击后未留在任务中心，无法确认领取结果")
                    try:
                        after_coins = read_balance(rt, after)
                    except RuntimeError:
                        continue
                    if before_coins is not None and after_coins > before_coins:
                        break
                after_image = rt.screenshot("after-claim.png")
                success_text = any(phrase in after["ocr"].get("text", "")
                                   for phrase in ("领取成功", "已领取", "任务完成奖励"))
                remaining_reward = any("待领取" in b["text"] or "继续看视频得金币" in b["text"]
                                       for b in after["ocr"].get("boxes", []))
                claimed = (before_coins is not None and after_coins is not None and after_coins > before_coins) or success_text or already_received
                if not claimed:
                    raise RuntimeError("已点击一次，但30秒内未能确认余额增加或明确的领取成功状态；不重复点击")
                result = {"coins_before": before_coins, "coins_after": after_coins,
                          "balance_increased": before_coins is not None and after_coins is not None and after_coins > before_coins,
                          "pending_reward_visible_after": remaining_reward,
                          "before_screenshot": str(screenshot), "after_screenshot": str(after_image)}
                (root / "claim-result.json").write_text(json.dumps(result, ensure_ascii=False, indent=2), encoding="utf-8")
                rt.log("已验证观看视频奖励领取完成：" + json.dumps(result, ensure_ascii=False))
        except BaseException as error:
            failure = error
            import traceback
            traceback.print_exc()
        status = "succeeded" if failure is None else "failed"
        summary = "已点击观看视频奖励卡片的领金币，并验证余额或待领取状态变化。" if not failure else "领取失败，已保存截图、OCR和错误现场。"
        print(json.dumps({"status": status, "result": result, "error": str(failure) if failure else None,
                          "run_id": run.run_id, "run_dir": str(root)}, ensure_ascii=False), flush=True)
        run.finish(status, exit_code=0 if failure is None else 1, summary=summary,
                   report_markdown="领取结果：" + json.dumps(result or {"error": str(failure)}, ensure_ascii=False)[:160])
    return 0 if failure is None else 1


if __name__ == "__main__":
    raise SystemExit(main())
