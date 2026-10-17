# 离线验证源

虚拟时钟验证计时、隔离设备动作，短期限真实线程验证回收。

```python path=scripts/test_flow.py
import random
import threading
import unittest
import tempfile
from pathlib import Path
from unittest.mock import Mock
from types import SimpleNamespace
from lib.flow import Flow, INVITE, paired_video, random_swipe, video_page
from lib.runtime import Runtime, center, unique

def box(text, x=.1, y=.4, w=.3, h=.03):
    return dict(text=text, x=x, y=y, w=w, h=h)
def screen(*boxes):
    return {"deadline": 1e12, "ocr": {"boxes": list(boxes), "text": "\n".join(b["text"] for b in boxes)}}
VIDEO = screen(box("生活 关注 发现", y=.04, w=.7))

class Fake:
    def __init__(self, values):
        self.values, self.actions, self.until = list(values), [], 1e12
        self.device = SimpleNamespace(swipe=self.swipe, key=lambda *a, **k: self.actions.append("back"))
    def read(self, end):
        value = self.values.pop(0) if len(self.values)>1 else self.values[0]
        if isinstance(value, Exception):
            raise value
        return value
    def tap(self, value, target, end):
        self.actions.append(target["text"])
    def video_xml(self, end):
        return False
    def navigation(self, label, end):
        self.actions.append(label)
    def swipe(self, start, end, **kwargs):
        self.actions.append((start, end, kwargs))
    def log(self, message):
        pass

class Clock:
    def __init__(self):
        self.now=0
    def __call__(self):
        return self.now
    def sleep(self, duration):
        self.now += duration

class Checks(unittest.TestCase):
    def test_merged_navigation(self):
        self.assertTrue(video_page(VIDEO))
        self.assertFalse(video_page(screen(box("生活 关注", y=.04))))
        self.assertFalse(video_page(screen(box("生活 关注 发现", y=.5))))
    def test_paired_task_only(self):
        title=box("连续签到14天得10.2元", w=.5)
        unrelated=box("去看视频", x=.8, y=.2, w=.15)
        target=box("去看视频", x=.8, y=.4, w=.15)
        self.assertIsNone(paired_video([title, unrelated]))
        self.assertIs(paired_video([title, unrelated, target]), target)
    def test_ambiguous_and_invalid_bounds(self):
        with self.assertRaises(RuntimeError):
            unique([box("去领取"),box("去领取")],"去领取")
        with self.assertRaises(RuntimeError):
            center(box("越界",x=.9,w=.3))
    def test_random_ranges(self):
        rng=random.Random(5)
        actions=[random_swipe(rng) for _ in range(200)]
        self.assertGreater(len(set(actions)),190)
        for start,end,ms in actions:
            self.assertTrue(.30<=start[0]<=.65 and .65<=start[1]<=.82)
            self.assertTrue(.30<=end[0]<=.65 and .25<=end[1]<=.43)
            self.assertTrue(350<=ms<=700 and .22<=start[1]-end[1]<=.57)
    def test_watch_two_minutes_and_return(self):
        clock=Clock(); runtime=Fake([VIDEO])
        flow=Flow(runtime,clock=clock,sleep=clock.sleep,rng=random.Random(4))
        flow.go_earn=lambda:runtime.actions.append("去赚钱")
        result=flow.watch(120)
        self.assertEqual(result["seconds"],120)
        self.assertEqual(runtime.actions[-1],"去赚钱")
        self.assertTrue(3<=result["swipes"]<=7)
    def test_video_page_lost(self):
        clock=Clock(); runtime=Fake([VIDEO,screen(box("其他页面"))])
        with self.assertRaises(RuntimeError):
            Flow(runtime,clock=clock,sleep=clock.sleep).watch(120)
        self.assertEqual(runtime.actions,[])
    def phone_flow(self,values):
        clock=Clock(); runtime=Fake(values)
        flow=Flow(runtime,clock=clock,sleep=clock.sleep)
        flow.find_phone=lambda:None
        return flow,runtime
    def test_phone_success(self):
        task=screen(box("日常任务"),box("连续签到"))
        flow,rt=self.phone_flow([screen(box("去签到")),screen(box("打卡成功")),task])
        self.assertEqual(flow.phone_checkin(),"本次打卡成功")
        self.assertEqual(rt.actions,["去签到","back"])
    def test_phone_already_signed(self):
        task=screen(box("日常任务"),box("连续签到"))
        flow,rt=self.phone_flow([screen(box("已签到")),task])
        self.assertEqual(flow.phone_checkin(),"本日已签到")
        self.assertEqual(rt.actions,["back"])
    def test_phone_no_success_is_failure_and_returns(self):
        flow,rt=self.phone_flow([screen(box("去签到")),screen(box("已签到"))])
        with self.assertRaises(TimeoutError):
            flow.phone_checkin()
        self.assertEqual(rt.actions,["去签到","back"])
    def test_guard_absent_and_reclaimed(self):
        rt=Fake([VIDEO]); Flow(rt).guardian(.02)
        self.assertEqual(rt.actions,[])
        self.assertFalse(any(t.name=="kuaishou-redpacket" for t in threading.enumerate()))
    def test_guard_click_once_requires_disappearance(self):
        rt=Fake([screen(box(INVITE),box("去领取"))])
        with self.assertRaises(RuntimeError):
            Flow(rt).guardian(.02)
        self.assertEqual(rt.actions,["去领取"])
    def test_guard_missing_button_and_transport(self):
        for value in [screen(box(INVITE)),RuntimeError("截图接口失败")]:
            rt=Fake([value])
            with self.assertRaises(RuntimeError):
                Flow(rt).guardian(.02)
            self.assertEqual(rt.actions,[])
    def test_guard_disappearance_allows_completion(self):
        rt=Fake([screen(box(INVITE),box("去领取")),VIDEO])
        Flow(rt).guardian(1.03)
        self.assertEqual(rt.actions,["去领取"])
    def xml_runtime(self, root, labels, captured="2026-10-17T00:00:01Z"):
        rt=Runtime(None,None,"test-serial",root)
        page={"package_name":"com.kuaishou.nebula","activity":"Home","component":"Home",
              "window_name":"Home","display":{"width":1080,"height":2400}}
        rt.page=Mock(side_effect=[dict(page,fetched_at="2026-10-17T00:00:00Z"),
                                  dict(page,fetched_at="2026-10-17T00:00:02Z")])
        nodes="".join(f'<node content-desc="{label}" package="com.kuaishou.nebula" enabled="true" bounds="[{x},68][{x+88},173]" />'
                      for label,x in labels)
        value={"serial":"test-serial","captured_at":captured,"display_size":{"width":1080,"height":2400},
               "xml":"<hierarchy>"+nodes+"</hierarchy>"}
        response=Mock();response.json.return_value=value;rt.request=Mock(return_value=response)
        return rt
    def test_xml_exact_navigation(self):
        with tempfile.TemporaryDirectory() as root:
            rt=self.xml_runtime(Path(root),[("生活",558),("关注",696),("发现",834)])
            self.assertTrue(rt.video_xml(__import__('time').monotonic()+5))
    def test_xml_missing_label_and_stale_capture_rejected(self):
        with tempfile.TemporaryDirectory() as root:
            rt=self.xml_runtime(Path(root),[("关注",696),("发现",834)])
            self.assertFalse(rt.video_xml(__import__('time').monotonic()+5))
            rt=self.xml_runtime(Path(root),[("生活",558),("关注",696),("发现",834)],captured="2026-10-16T00:00:01Z")
            with self.assertRaises(RuntimeError):
                rt.video_xml(__import__('time').monotonic()+5)
    def test_guard_ambiguous_button(self):
        rt=Fake([screen(box(INVITE),box("去领取"),box("去领取"))])
        with self.assertRaises(RuntimeError):
            Flow(rt).guardian(.02)
        self.assertEqual(rt.actions,[])

if __name__=="__main__":
    unittest.main(verbosity=2)
```
