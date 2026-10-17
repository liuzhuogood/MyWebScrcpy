#!/usr/bin/env python3
"""Run a bounded, evidence-backed Kuaishou video refresh session."""
import argparse
import hashlib
import json
import os
import random
import re
import subprocess
import sys
import time
import traceback
from pathlib import Path
from urllib.parse import urlencode

from mywebscrcpy import Client, TaskReporter

PACKAGE = "com.kuaishou.nebula"
HOME_ACTIVITY = "com.yxcorp.gifshow.HomeActivity"
# 本机实测：首页视频流底部导航为首页/朋友/去赚钱/我，顶部tab为探索/同城/商城/关注/发现（无“生活”）。
NAV_LABELS = ("首页", "朋友", "去赚钱", "我")
TOP_LABELS = ("探索", "同城", "商城", "关注", "发现")
POPUP_BUTTONS = ("立即打卡", "立即签到", "去看内容", "继续赚钱")
MODAL_MARKERS = ("立即打卡", "立即签到", "去看内容", "立即参与", "朋友推荐",
                 "今日签到可领", "今日打卡待完成", "翻倍任务开启", "瓜分百亿金币", "继续赚钱")
RED_PACKET = "遊好友得大额红包"
CHECK_SECONDS = 60


class PageTransition(RuntimeError):
    pass


class Runner:
    def __init__(self, base, serial, root, device, rng):
        self.base, self.serial, self.root, self.device, self.rng = base.rstrip("/"), serial, root, device, rng
        self.seq = 0
        self.recording_id = None
        self.last_screenshot_at = 0.0
        self.log_path = root / "events.jsonl"

    def log(self, message):
        item = {"at": time.strftime("%Y-%m-%dT%H:%M:%S%z"), "message": message}
        with self.log_path.open("a", encoding="utf-8") as f:
            f.write(json.dumps(item, ensure_ascii=False) + "\n")
        print(item["at"], message, flush=True)

    def curl(self, method, path, *, params=None, body=None, output=None, timeout=20):
        url = self.base + path
        if params:
            url += "?" + urlencode(params)
        cmd = ["curl", "--fail-with-body", "--silent", "--show-error", "--max-time", str(timeout), "-X", method, url]
        if body is not None:
            cmd += ["-H", "Content-Type: application/json", "--data-binary", json.dumps(body, ensure_ascii=False)]
        if output is not None:
            cmd += ["--output", str(output)]
        result = subprocess.run(cmd, capture_output=True, text=output is None, check=False)
        if result.returncode != 0:
            detail = result.stderr if output is None else "binary API request failed"
            raise RuntimeError(f"curl {method} {path} exited {result.returncode}: {detail}")
        if output is not None:
            return output
        return json.loads(result.stdout)

    def page(self):
        self.seq += 1
        value = self.curl("GET", "/api/ui/page-info", params={"serial": self.serial})
        (self.root / f"page-{self.seq:04}.json").write_text(json.dumps(value, ensure_ascii=False, indent=2))
        return value

    @staticmethod
    def identity(page):
        return tuple(page[k] for k in ("package_name", "activity", "component", "window_name", "display"))

    def state(self):
        before = self.page()
        value = self.curl("POST", "/api/vision/ocr-text-art", body={
            "serial": self.serial, "engine": "macocr", "columns": 100, "rows": 100,
        }, timeout=45)
        after = self.page()
        if self.identity(before) != self.identity(after):
            raise PageTransition("OCR识别期间前台页面发生变化，本次定位作废")
        if before["package_name"] != PACKAGE:
            raise RuntimeError("前台不是快手极速版，停止设备操作")
        self.seq += 1
        prefix = self.root / f"state-{self.seq:04}"
        prefix.with_suffix(".json").write_text(json.dumps({"before": before, "ocr": value, "after": after}, ensure_ascii=False, indent=2))
        return {"page": after, "ocr": value, "path": str(prefix.with_suffix(".json"))}

    def state_retry(self, tries=12, delay=2):
        for attempt in range(tries):
            try:
                return self.state()
            except PageTransition as error:
                if attempt == tries - 1:
                    raise
                current = self.page()
                if current["package_name"] != PACKAGE:
                    self.log(f"检测到快手退至后台（当前包：{current['package_name']}），唤醒并重新启动应用")
                    self.device.adb(["shell", "input", "keyevent", "KEYCODE_WAKEUP"], timeout=8)
                    self.device.adb(["shell", "monkey", "-p", PACKAGE, "-c",
                                     "android.intent.category.LAUNCHER", "1"], timeout=15)
                self.log(f"页面仍在变化，{delay}秒后重试采集（{error}）")
                time.sleep(delay)
        raise RuntimeError("页面持续变化，未能取得稳定状态")

    def screenshot(self, label):
        page = self.page()
        display = page["display"]
        target = self.root / label
        self.curl("POST", "/api/v1/vision/region-screenshot", body={
            "serial": self.serial, "x": 0, "y": 0,
            "width": display["width"], "height": display["height"],
        }, output=target, timeout=30)
        self.last_screenshot_at = time.monotonic()
        return target

    def screenshot_if_due(self, label):
        if time.monotonic() - self.last_screenshot_at >= 60:
            return self.screenshot(label)
        return None

    def xml_nodes(self):
        before = self.page()
        value = self.curl("GET", "/api/ui/xml", params={"serial": self.serial})
        after = self.page()
        if self.identity(before) != self.identity(after) or before["package_name"] != PACKAGE:
            raise RuntimeError("XML定位时页面变化或不在快手，停止操作")
        if value["serial"] != self.serial:
            raise RuntimeError("XML响应设备与目标设备不一致")
        self.seq += 1
        (self.root / f"xml-{self.seq:04}.json").write_text(json.dumps({"before": before, "xml": value, "after": after}, ensure_ascii=False, indent=2))
        import xml.etree.ElementTree as ET
        return value, list(ET.fromstring(value["xml"]).iter("node"))

    def click_accessibility(self, label):
        self.screenshot(f"before-click-{self.seq + 1:04}.png")
        value, nodes = self.xml_nodes()
        matches = [n for n in nodes if n.attrib["content-desc"] == label and n.attrib["package"] == PACKAGE
                   and n.attrib["enabled"] == "true" and n.attrib["clickable"] == "true"]
        if len(matches) != 1:
            raise RuntimeError(f"当前XML中可点击的{label}不是唯一节点")
        match = re.fullmatch(r"\[(\d+),(\d+)\]\[(\d+),(\d+)\]", matches[0].attrib["bounds"])
        if not match:
            raise RuntimeError(f"{label}控件bounds无效")
        x1, y1, x2, y2 = map(int, match.groups())
        width, height = value["display_size"]["width"], value["display_size"]["height"]
        if not (0 <= x1 < x2 <= width and 0 <= y1 < y2 <= height):
            raise RuntimeError(f"{label}控件bounds超出屏幕")
        point = ((x1 + x2) / 2 / width, (y1 + y2) / 2 / height)
        self.log(f"点击当前XML唯一控件：{label}")
        self.device.tap(point, timeout=8)

    def label_point(self, ocr, label, prefix=False):
        boxes = [b for b in ocr.get("boxes", []) if b["text"].strip() == label]
        if not boxes and prefix:
            boxes = [b for b in ocr.get("boxes", []) if b["text"].strip().startswith(label)]
        if len(boxes) > 1:
            raise RuntimeError(f"当前画面文字 {label} 不唯一，匹配数={len(boxes)}")
        if not boxes:
            return None
        box = boxes[0]
        return (box["x"] + box["w"] / 2, box["y"] + box["h"] / 2)

    def click_label(self, label, state, prefix=False):
        point = self.label_point(state["ocr"], label, prefix)
        if point is None:
            raise RuntimeError(f"当前画面未见可点击文字：{label}")
        self.log(f"点击当前画面唯一文字：{label} @ {point}")
        self.device.tap(point, timeout=8)

    @staticmethod
    def top_text(ocr):
        boxes = ocr.get("boxes", [])
        top = sorted((b for b in boxes if b["y"] + b["h"] <= 0.13), key=lambda b: b["x"])
        return " ".join(b["text"] for b in top)

    @staticmethod
    def bottom_text(ocr, state_page):
        boxes = ocr.get("boxes", [])
        bottom = sorted((b for b in boxes if b["y"] >= 0.85), key=lambda b: b["x"])
        return " ".join(b["text"] for b in bottom)

    def video_valid(self, state):
        page = state["page"]
        if page["package_name"] != PACKAGE or page["activity"] != HOME_ACTIVITY:
            return False
        text = self.bottom_text(state["ocr"], page)
        hits = [label for label in NAV_LABELS if label in text]
        if len(hits) >= 3:
            return True
        self.log(f"底部导航不完整{hits}，按技能规范用当前XML精确复核顶部tab")
        value, nodes = self.xml_nodes()
        width, height = value["display_size"]["width"], value["display_size"].get("height", 0)
        positions = {}
        for n in nodes:
            a = n.attrib
            label = a["content-desc"]
            if label not in {"关注", "发现"} or a["package"] != PACKAGE or a["enabled"] != "true":
                continue
            m = re.fullmatch(r"\[(\d+),(\d+)\]\[(\d+),(\d+)\]", a["bounds"])
            if not m:
                raise RuntimeError("视频页导航控件bounds无效")
            x1, y1, x2, y2 = map(int, m.groups())
            if 0 <= x1 < x2 <= width and 0 <= y1 < y2 <= 0.13 * height:
                if label in positions:
                    raise RuntimeError("视频页顶部导航不唯一")
                positions[label] = x1
        return set(positions) == {"关注", "发现"} and positions["关注"] < positions["发现"]

    @staticmethod
    def popup_button(ocr):
        text = ocr.get("text", "")
        candidates = ([ "去领取" ] if RED_PACKET in text else []) + list(POPUP_BUTTONS)
        return candidates

    def find_popup_button(self, ocr):
        for label in self.popup_button(ocr):
            try:
                if self.label_point(ocr, label) is not None:
                    return label
            except RuntimeError:
                raise
        return None

    def dismiss_popups(self, rounds=3):
        state = self.state_retry()
        for _ in range(rounds):
            label = self.find_popup_button(state["ocr"])
            if label is None:
                return state
            self.log(f"发现弹窗按钮 {label}，按用户操作习惯点击关掉")
            self.click_label(label, state)
            time.sleep(3)
            state = self.state_retry()
        return state

    @staticmethod
    def nav_visible(ocr):
        boxes = ocr.get("boxes", [])
        bottom = " ".join(b["text"] for b in boxes if b["y"] >= .85)
        return sum(1 for label in NAV_LABELS if label in bottom) >= 3

    @staticmethod
    def blocking_overlay(ocr):
        text = ocr.get("text", "")
        return any(marker in text for marker in MODAL_MARKERS)

    def tap_page_back(self, state):
        boxes = [b for b in state["ocr"].get("boxes", []) if "任务中心" in b["text"]]
        if not boxes:
            return False
        box = boxes[0]
        point = (box["x"] + 0.012, box["y"] + box["h"] / 2)
        self.log(f"点击任务中心左上角返回箭头 @ {point}")
        self.device.tap(point, timeout=8)
        return True

    def force_restart_app(self, timeout=45):
        self.log("任务中心页返回不可用；按接管记录原因，用 ADB am force-stop + monkey 冷启回首页")
        self.device.adb(["shell", "am", "force-stop", PACKAGE], timeout=10)
        time.sleep(2)
        self.device.adb(["shell", "monkey", "-p", PACKAGE, "-c", "android.intent.category.LAUNCHER", "1"], timeout=15)
        deadline = time.monotonic() + timeout
        while time.monotonic() < deadline:
            time.sleep(1)
            if self.page()["package_name"] != PACKAGE:
                continue
            state = self.state_retry()
            if state["page"]["activity"] == HOME_ACTIVITY:
                return state
        raise RuntimeError("冷启后仍未回到首页视频流")

    def back_to_home(self, tries=3):
        for _ in range(tries):
            if self.page()["package_name"] != PACKAGE:
                return self.relaunch_if_left()
            state = self.state_retry()
            label = self.find_popup_button(state["ocr"])
            if label is not None:
                self.log(f"返回前先按用户操作习惯点掉弹窗按钮 {label}")
                self.click_label(label, state)
                time.sleep(2)
                continue
            if "任务中心" in state["ocr"].get("text", "") and state["page"]["activity"] != HOME_ACTIVITY:
                if self.tap_page_back(state):
                    time.sleep(2)
                    continue
            if (state["page"]["activity"] == HOME_ACTIVITY and self.nav_visible(state["ocr"])
                    and not self.blocking_overlay(state["ocr"])):
                return state
            self.log("画面仍有弹层、广告落地页或不在首页视频流，按返回键关闭/回退")
            self.device.key(4, timeout=8)
            time.sleep(2)
        return self.force_restart_app()

    def relaunch_if_left(self, timeout=40):
        if self.page()["package_name"] == PACKAGE:
            return self.state_retry()
        self.log("已离开快手极速版，按规范用 monkey 重新启动")
        self.device.adb(["shell", "monkey", "-p", PACKAGE, "-c", "android.intent.category.LAUNCHER", "1"], timeout=15)
        deadline = time.monotonic() + timeout
        while time.monotonic() < deadline:
            time.sleep(1)
            if self.page()["package_name"] == PACKAGE:
                return self.state_retry()
        raise RuntimeError("重新启动后未回到快手极速版")

    def open_task_center(self, timeout=30, attempts=2):
        for attempt in range(1, attempts + 1):
            state = self.state_retry()
            if "任务中心" in state["ocr"].get("text", ""):
                return state
            if state["page"]["activity"] != HOME_ACTIVITY or self.blocking_overlay(state["ocr"]):
                state = self.back_to_home()
            try:
                self.click_label("去赚钱", state, prefix=True)
            except RuntimeError as error:
                self.log(f"点击去赚钱失败：{error}")
                self.back_to_home()
                continue
            end = time.monotonic() + timeout
            while time.monotonic() < end:
                time.sleep(1)
                try:
                    state = self.state()
                except PageTransition:
                    continue
                if "任务中心" in state["ocr"].get("text", ""):
                    return state
            self.log(f"第{attempt}次点击去赚钱后未进入任务中心，先回首页再试")
            self.back_to_home()
        raise TimeoutError("点击去赚钱后未进入任务中心")

    def earnings(self, ocr):
        text = ocr.get("text", "")
        if "任务中心" not in text:
            return False
        try:
            self.coin_total(ocr)
        except RuntimeError as error:
            self.log(f"任务中心已确认但金币余额暂不可读：{error}")
            return False
        return True

    def coin_total(self, ocr):
        boxes = ocr.get("boxes", [])
        labels = [b for b in boxes if b["text"].strip().startswith("我的金") and b["x"] < .4 and b["y"] < .2]
        if len(labels) != 1:
            raise RuntimeError(f"收益页顶部‘我的金币’余额标签不唯一，匹配数={len(labels)}")
        label = labels[0]
        amounts = [b for b in boxes if re.fullmatch(r"[\d,]+", b["text"].strip())
                   and b["x"] < .4 and label["y"] < b["y"] < .2
                   and abs(b["x"] - label["x"]) <= .06]
        if len(amounts) != 1:
            raise RuntimeError(f"‘我的金币’标签下方余额不唯一，匹配数={len(amounts)}")
        return int(amounts[0]["text"].replace(",", ""))

    def read_coins(self, label, timeout=120):
        """进入任务中心，读到“我的金币”余额并留存截图；弹窗只在读不到余额时才点掉。"""
        end = time.monotonic() + timeout
        state = self.open_task_center()
        while time.monotonic() < end:
            text = state["ocr"].get("text", "")
            if "任务中心" not in text:
                self.log("已离开任务中心，按返回键回首页后重新进入")
                self.back_to_home()
                state = self.open_task_center()
                continue
            try:
                coins = self.coin_total(state["ocr"])
            except RuntimeError as error:
                self.log(f"任务中心金币暂不可读：{error}")
                label_button = self.find_popup_button(state["ocr"])
                if label_button:
                    self.log(f"按用户操作习惯点掉弹窗按钮 {label_button} 后复读余额")
                    self.click_label(label_button, state)
                    time.sleep(3)
                else:
                    time.sleep(2)
                state = self.state_retry()
                continue
            # 收益页金币数字有入场动画，单次读数可能是过渡值；必须两次一致才采用
            time.sleep(3)
            try:
                verify = self.coin_total(self.state_retry()["ocr"])
            except (RuntimeError, PageTransition) as error:
                self.log(f"金币读数复核失败：{error}")
                state = self.state_retry()
                continue
            if verify != coins:
                self.log(f"金币读数未稳定（{coins} 与 {verify} 不一致），稍后重读")
                state = self.state_retry()
                continue
            image = self.screenshot(label)
            observed = self.state_retry()
            item = {"coins": coins, "page": observed["page"], "ocr": observed["ocr"],
                    "screenshot": str(image), "observed_at": time.time()}
            (self.root / f"{Path(label).stem}.json").write_text(json.dumps(item, ensure_ascii=False, indent=2))
            self.log(f"收益页金币读数：{coins}；截图：{image}")
            return coins, item
        raise TimeoutError("未能在任务中心读到金币余额")

    def start_recording(self, max_duration_ms):
        active = self.curl("GET", "/api/recordings", params={"serial": self.serial})["recordings"]
        if any(item["status"] == "recording" for item in active):
            raise RuntimeError("设备已有进行中的录像，按规范停止以免干扰用户录像")
        value = self.curl("POST", "/api/recordings", params={"serial": self.serial}, body={
            "max_duration_ms": max_duration_ms, "record_audio": False,
        })
        if value["status"] != "recording" or value["serial"] != self.serial:
            raise RuntimeError("录像服务未确认本设备已开始录像")
        self.recording_id = value["recording_id"]
        (self.root / "recording-start.json").write_text(json.dumps(value, ensure_ascii=False, indent=2))
        self.log(f"录像已启动，recording_id={self.recording_id}")

    def stop_and_save_recording(self):
        if not self.recording_id:
            return
        rid = self.recording_id
        self.curl("POST", f"/api/recordings/{rid}/stop", params={"serial": self.serial})
        end = time.monotonic() + 180
        while True:
            value = self.curl("GET", f"/api/recordings/{rid}", params={"serial": self.serial})
            if value["status"] in {"completed", "stopped"}:
                break
            if value["status"] == "failed":
                raise RuntimeError("录像失败：" + value["failure_reason"])
            if time.monotonic() >= end:
                raise TimeoutError("等待录像停止超过180秒")
            time.sleep(1)
        (self.root / "recording-final.json").write_text(json.dumps(value, ensure_ascii=False, indent=2))
        video = self.root / "refresh-session.mp4"
        self.curl("GET", f"/api/recordings/{rid}/download", params={"serial": self.serial}, output=video, timeout=1800)
        size = video.stat().st_size
        if size != value["bytes_written"]:
            raise RuntimeError("录像文件大小与服务端记录不一致")
        digest = hashlib.sha256()
        with video.open("rb") as f:
            for chunk in iter(lambda: f.read(4 * 1024 * 1024), b""):
                digest.update(chunk)
        import av
        with av.open(str(video)) as container:
            stream = container.streams.video[0]
            first = next(container.decode(stream))
            if first.width <= 0 or first.height <= 0:
                raise RuntimeError("录像首帧解码失败")
            duration = float(stream.duration * stream.time_base) if stream.duration is not None else None
        result = {"path": str(video), "bytes": size, "sha256": digest.hexdigest(),
                  "width": first.width, "height": first.height, "duration_seconds": duration}
        (self.root / "recording-verified.json").write_text(json.dumps(result, ensure_ascii=False, indent=2))
        self.log(f"录像下载完成并通过MP4首帧解码，大小={size} bytes")


def main():
    parser = argparse.ArgumentParser(description="快手极速版长时视频刷新")
    parser.add_argument("--seconds", type=int, default=3600)
    parser.add_argument("--seed", type=int, default=None)
    args = parser.parse_args()
    if not 1 <= args.seconds <= 14400:
        parser.error("刷新时长必须在1到14400秒之间")
    base = os.environ["MYWEBSCRCPY_API_URL"]
    serial = os.environ["MYWEBSCRCPY_SERIAL"]
    root = Path(os.environ["MYWEBSCRCPY_RUN_DIR"]).resolve()
    rng = random.Random(args.seed)
    outcome = None
    failure = None
    cancelled = False
    run = TaskReporter(base, name=f"快手极速版视频刷新{args.seconds}秒", device_serial=serial, source="codex")
    with run, run.capture_output(), Client(base) as client:
        try:
            schema = Runner(base, serial, root, None, rng).curl("GET", "/api/openapi.json")
            for path, method in (("/api/ui/page-info", "get"), ("/api/ui/xml", "get"),
                                 ("/api/vision/ocr-text-art", "post"),
                                 ("/api/v1/vision/region-screenshot", "post"),
                                 ("/api/recordings", "post")):
                if method not in schema["paths"].get(path, {}):
                    raise RuntimeError(f"运行中OpenAPI未提供必需接口：{method.upper()} {path}")
            devices = client.devices.list()
            if not any(d.serial == serial and d.state == "device" for d in devices):
                raise RuntimeError("指定设备当前未处于device状态")
            with client.device(serial) as device:
                rt = Runner(base, serial, root, device, rng)
                rt.start_recording((args.seconds + 300) * 1000)
                rt.log("通过SDK唤醒设备、请求解除锁屏，并核验快手安装后启动")
                device.adb(["shell", "input", "keyevent", "KEYCODE_WAKEUP"], timeout=8)
                device.adb(["shell", "wm", "dismiss-keyguard"], timeout=8)
                installed = device.adb(["shell", "pm", "path", PACKAGE], timeout=10)
                if not installed.stdout.startswith("package:"):
                    raise RuntimeError("设备未安装快手极速版")
                device.adb(["shell", "monkey", "-p", PACKAGE, "-c", "android.intent.category.LAUNCHER", "1"], timeout=15)
                deadline = time.monotonic() + 40
                state = None
                while time.monotonic() < deadline:
                    page = rt.page()
                    if page["package_name"] == PACKAGE:
                        try:
                            state = rt.state()
                            break
                        except PageTransition:
                            pass
                    time.sleep(1)
                if state is None:
                    raise TimeoutError("启动后40秒未进入快手极速版")

                # 基线：先去“去赚钱”统计金币数量（读不到不中断整点观看，仅记录）
                try:
                    coins, baseline = rt.read_coins("baseline-earnings.png")
                    print("BASELINE_SCREENSHOT=" + baseline["screenshot"], flush=True)
                    print("BASELINE_COINS=" + str(coins), flush=True)
                except (TimeoutError, RuntimeError) as error:
                    coins, baseline = None, None
                    rt.log(f"金币基线未能读取：{error}")
                    print("BASELINE_COINS=unreadable", flush=True)
                input("已读取金币基线。检查截图后按 Enter 开始视频刷新计时：")

                rt.log("返回首页视频流")
                state = rt.back_to_home()
                deadline = time.monotonic() + 40
                while time.monotonic() < deadline and not rt.video_valid(state):
                    time.sleep(1)
                    try:
                        state = rt.state()
                    except PageTransition:
                        continue
                if not rt.video_valid(state):
                    rt.log("返回首页后未确认视频流，冷启快手再确认")
                    state = rt.force_restart_app()
                if not rt.video_valid(state):
                    raise TimeoutError("返回首页后40秒内未确认首页视频流")

                started = time.monotonic()
                stop_at = started + args.seconds
                last_swipe = started
                next_swipe = started + rng.uniform(15, 27)
                next_check = started + CHECK_SECONDS
                swipe_count = 0
                recoveries = 0
                popups = 0
                last_checkpoint = 0
                rt.log(f"已确认视频页，开始计时刷新{args.seconds}秒；上滑间隔15～30秒，每{CHECK_SECONDS}秒核验一次页面")
                while True:
                    now = time.monotonic()
                    if now >= stop_at:
                        break
                    if now >= next_swipe:
                        point1 = (rng.uniform(.30, .65), rng.uniform(.65, .82))
                        point2 = (rng.uniform(.30, .65), rng.uniform(.25, .43))
                        duration = rng.randint(350, 700)
                        rt.log(f"视频上滑{swipe_count + 1} 起点={point1} 终点={point2} 时长={duration}ms")
                        device.swipe(point1, point2, duration_ms=duration, timeout=8)
                        last_swipe = time.monotonic()
                        swipe_count += 1
                        next_swipe = last_swipe + rng.uniform(15, 27)
                        continue
                    if now >= next_check:
                        device.adb(["shell", "input", "keyevent", "KEYCODE_WAKEUP"], timeout=8)
                        rt.screenshot_if_due(f"video-{int(now - started) // 60:04}min.png")
                        try:
                            state = rt.state()
                        except PageTransition:
                            rt.log("核验时页面仍在变化，稍后重试")
                            next_check = time.monotonic() + 3
                            continue
                        label = rt.find_popup_button(state["ocr"])
                        if label:
                            popups += 1
                            if popups > 3:
                                raise RuntimeError("连续3次未能点掉弹窗，停止计时")
                            rt.log(f"核验发现弹窗按钮 {label}（第{popups}次），按用户操作习惯点掉")
                            rt.click_label(label, state)
                            time.sleep(2)
                            next_check = time.monotonic()
                            continue
                        if rt.video_valid(state) and not rt.blocking_overlay(state["ocr"]):
                            recoveries = 0
                            popups = 0
                            next_check = time.monotonic() + CHECK_SECONDS
                        else:
                            recoveries += 1
                            rt.log(f"视频页特征缺失或存在弹层（第{recoveries}次），按用户操作习惯清理并回首页")
                            try:
                                rt.back_to_home()
                            except (RuntimeError, TimeoutError) as error:
                                rt.log(f"回首页失败：{error}")
                            if recoveries > 3:
                                raise RuntimeError("连续3次未能恢复首页视频流，停止计时")
                            next_check = time.monotonic()
                        continue
                    time.sleep(min(1, max(0, min(next_swipe, next_check, stop_at) - now)))
                    elapsed = min(args.seconds, int(time.monotonic() - started))
                    if elapsed >= last_checkpoint + 60:
                        last_checkpoint = elapsed - elapsed % 60
                        rt.log(f"刷新进度 {elapsed}/{args.seconds} 秒；累计上滑{swipe_count}次")
                elapsed = time.monotonic() - started
                if elapsed < args.seconds:
                    raise RuntimeError("视频刷新有效计时不足指定时长")

                try:
                    final_coins, final = rt.read_coins("final-earnings.png")
                    final_screenshot = final["screenshot"]
                except (TimeoutError, RuntimeError) as error:
                    final_coins, final_screenshot = None, None
                    rt.log(f"结束金币未能读取：{error}")
                    try:
                        final_screenshot = str(rt.screenshot("final-earnings.png"))
                    except Exception:
                        traceback.print_exc()
                outcome = {"seconds": round(elapsed, 1), "swipes": swipe_count, "coins_before": coins,
                           "coins_after": final_coins, "final_screenshot": final_screenshot,
                           "swipe_interval": "15-27s", "check_interval": CHECK_SECONDS}
                (root / "result.json").write_text(json.dumps({"status": "succeeded", **outcome}, ensure_ascii=False, indent=2))
                rt.log(f"完成视频刷新；有效时长{elapsed:.1f}秒，上滑{swipe_count}次，金币{coins}→{final_coins}")
        except KeyboardInterrupt as error:
            failure, cancelled = error, True
            traceback.print_exc()
        except BaseException as error:
            failure = error
            traceback.print_exc()
        finally:
            if "rt" in locals():
                if failure is not None:
                    try:
                        rt.screenshot("failure-screen.png")
                    except Exception:
                        traceback.print_exc()
                try:
                    rt.stop_and_save_recording()
                except Exception:
                    traceback.print_exc()
                    if failure is None:
                        failure = RuntimeError("停止、下载或验证录像失败")
            status = "cancelled" if cancelled else ("failed" if failure is not None else "succeeded")
            summary = "完成视频刷新并返回收益页，金币前后数值已采集。" if status == "succeeded" else "视频刷新未完成，已保留执行日志与现场证据。"
            if failure is not None:
                (root / "failure.txt").write_text("".join(traceback.format_exception(type(failure), failure, failure.__traceback__)), encoding="utf-8")
            print(json.dumps({"status": status, "outcome": outcome, "error": str(failure) if failure else None,
                              "run_id": run.run_id, "run_dir": str(root)}, ensure_ascii=False), flush=True)
            run.finish(status, exit_code=0 if status == "succeeded" else 1, summary=summary,
                        report_markdown="结果：" + json.dumps(outcome or {"error": str(failure)}, ensure_ascii=False)[:160])
    return 0 if failure is None else 1


if __name__ == "__main__":
    raise SystemExit(main())
