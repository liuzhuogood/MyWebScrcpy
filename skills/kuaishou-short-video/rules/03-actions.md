# 动作实现源

以下代码块生成 `scripts/lib/flow.py`，规则为唯一手写源。设备实现见[设备与证据](05-runtime.md)，入口见[入口实现](06-entrypoint.md)，验证见[离线验证](07-dryrun.md)。

```python path=scripts/lib/flow.py
import queue
import random
import threading
import time
import httpx
from .runtime import PageChanged, unique

PHONE = "连续打卡白拿手机"
INVITE = "遊好友得大额红包"

def video_page(value):
    top = sorted((b for b in value["ocr"]["boxes"] if b["y"]+b["h"] <= .13), key=lambda b: b["x"])
    text = " ".join(b["text"] for b in top)
    return all(word in text for word in ("生活", "关注", "发现"))

def paired_video(boxes):
    title = unique(boxes, "连续签到", contains=True)
    if title is None:
        return None
    choices = [b for b in boxes if b["text"] == "去看视频"
               and b["x"] >= title["x"]+title["w"]
               and min(b["y"]+b["h"], title["y"]+title["h"]) > max(b["y"], title["y"])]
    if len(choices) > 1:
        raise RuntimeError("连续签到任务旁的去看视频不唯一")
    return choices[0] if choices else None

def random_swipe(rng):
    return ((rng.uniform(.30, .65), rng.uniform(.65, .82)),
            (rng.uniform(.30, .65), rng.uniform(.25, .43)), rng.randint(350, 700))

def earnings(value):
    text = value["ocr"]["text"]
    return "日常任务" in text and (PHONE in text or "连续签到" in text)

class Flow:
    def __init__(self, runtime, *, clock=time.monotonic, sleep=time.sleep, rng=None):
        self.rt, self.clock, self.sleep = runtime, clock, sleep
        self.rng = random.Random() if rng is None else rng

    def wait(self, predicate, seconds=15):
        end = min(self.clock()+seconds, self.rt.until)
        while self.clock() < end:
            try:
                value = self.rt.read(end)
                if predicate(value):
                    return value
            except PageChanged:
                self.rt.log("页面变化，重新采集")
            self.sleep(min(.5, max(0, end-self.clock())))
        raise TimeoutError("等待页面条件超时")

    def guardian(self, seconds=30):
        self.rt.log("启动异步红包守护")
        end = self.clock()+seconds
        stop, errors = threading.Event(), queue.Queue(maxsize=1)
        def worker():
            pending = False
            try:
                while self.clock() < end and not stop.is_set():
                    try:
                        value = self.rt.read(end)
                    except PageChanged:
                        continue
                    except (TimeoutError, httpx.TimeoutException):
                        if self.clock() >= end:
                            break
                        raise
                    seen = INVITE in value["ocr"]["text"]
                    if seen and not pending:
                        button = unique(value["ocr"]["boxes"], "去领取")
                        if button is None:
                            raise RuntimeError("红包出现但去领取按钮缺失")
                        if stop.is_set() or self.clock() >= end:
                            break
                        self.rt.tap(value, button, end)
                        pending = True
                    if not seen:
                        pending = False
                    stop.wait(min(1, max(0, end-self.clock())))
                if pending:
                    raise RuntimeError("红包点击后未确认弹窗消失")
            except BaseException as error:
                errors.put_nowait(error)
        thread = threading.Thread(target=worker, name="kuaishou-redpacket")
        thread.start()
        try:
            thread.join(timeout=seconds+2)
        finally:
            stop.set()
            thread.join(timeout=11)
        if thread.is_alive():
            raise RuntimeError("守护线程未退出，禁止进入下一页")
        if not errors.empty():
            raise errors.get_nowait()
        self.rt.log("红包守护结束，线程已回收")

    def go_earn(self):
        self.rt.navigation("去赚钱", self.clock()+20)
        return self.wait(earnings, 20)

    def find_phone(self):
        for index in range(5):
            value = self.wait(earnings, 15)
            button = unique(value["ocr"]["boxes"], PHONE)
            if button is not None:
                self.rt.tap(value, button, self.clock()+5)
                return
            if index < 4:
                self.rt.log("在任务列表中向下查找手机任务")
                self.rt.device.swipe((.5, .80), (.5, .40), duration_ms=500, timeout=5)
        raise RuntimeError("有界查找后未找到手机任务，不能推断已完成")

    def phone_checkin(self):
        self.find_phone()
        failure, result = None, None
        try:
            value = self.wait(lambda v: "去签到" in v["ocr"]["text"] or "已签到" in v["ocr"]["text"], 20)
            boxes = value["ocr"]["boxes"]
            go, done = unique(boxes, "去签到"), unique(boxes, "已签到")
            if go is not None and done is not None:
                raise RuntimeError("去签到与已签到同时存在，需要视觉核对归属")
            if go is None and done is not None:
                result = "本日已签到"
            elif go is not None:
                self.rt.tap(value, go, self.clock()+5)
                self.wait(lambda v: "打卡成功" in v["ocr"]["text"], 30)
                result = "本次打卡成功"
            else:
                raise RuntimeError("签到文字存在但无唯一可操作文字框")
        except BaseException as error:
            failure = error
        try:
            self.rt.log("返回手机任务列表")
            self.rt.device.key(4, timeout=5)
            self.wait(earnings, 15)
        except BaseException:
            if failure is not None:
                self.rt.log("返回也失败，保留原始签到错误")
            else:
                raise
        if failure is not None:
            raise failure
        return result

    def is_video(self, value):
        if video_page(value):
            return True
        return self.rt.video_xml(min(self.clock()+10, value["deadline"]))

    def watch(self, seconds=120):
        self.wait(self.is_video, 15)
        start = self.clock()
        end = start+seconds
        last_swipe = start
        # 目标等待15～27秒，给当前页核验预留最多3秒；实际动作间隔仍须<=30秒。
        next_swipe = start+self.rng.uniform(15, 27)
        count = 0
        while self.clock() < end:
            wake = min(end, next_swipe, self.clock()+5)
            self.sleep(max(0, wake-self.clock()))
            value = self.rt.read(min(self.clock()+10, last_swipe+30, self.rt.until))
            if not self.is_video(value):
                raise RuntimeError("观看中视频页特征缺失，停止计时与滑动")
            if self.clock() >= end:
                break
            if self.clock() >= next_swipe:
                start_point, end_point, duration = random_swipe(self.rng)
                self.rt.log(f"观看视频上滑 {count+1}，起点={start_point}，终点={end_point}，时长={duration}ms")
                sent_at = self.clock()
                action_budget = min(5, last_swipe+30-sent_at)
                if action_budget <= 0:
                    raise TimeoutError("识别耗时超出30秒滑动间隔，停止观看")
                self.rt.device.swipe(start_point, end_point, duration_ms=duration, timeout=action_budget)
                last_swipe = sent_at
                count += 1
                after = self.rt.read(min(self.clock()+10, self.rt.until))
                if not self.is_video(after):
                    raise RuntimeError("滑动后视频页特征缺失")
                next_swipe = last_swipe+self.rng.uniform(15, 27)
        elapsed = self.clock()-start
        self.rt.log(f"观看模式完成，已确认时长{elapsed:.1f}秒，上滑{count}次")
        self.go_earn()
        return {"seconds": round(elapsed, 1), "swipes": count, "returned": True}

    def tasks(self, seconds):
        self.wait(lambda v: True, 15)
        self.guardian()
        self.go_earn()
        phone = self.phone_checkin()
        value = self.wait(earnings, 15)
        title = unique(value["ocr"]["boxes"], "连续签到", contains=True)
        if title is not None:
            y = max(0, title["y"]-.02)
            value = self.rt.read(self.clock()+15, region=(0, y, 1, min(.12, 1-y)))
            if unique(value["ocr"]["boxes"], "连续签到", contains=True) is None:
                raise RuntimeError("任务行变化，停止使用旧标题位置")
        button = paired_video(value["ocr"]["boxes"])
        if button is None:
            title = unique(value["ocr"]["boxes"], "连续签到", contains=True)
            state = "当前可见任务不要求看视频" if title is not None else "当前视窗未见连续签到，未断言不存在"
            return {"phone": phone, "continuous": state}
        self.rt.tap(value, button, self.clock()+5)
        result = self.watch(seconds)
        result.update(phone=phone, continuous="已观看并返回，平台签到结果另行核对")
        return result
```
