# Generated from rules/05-runtime.md; do not edit.
import json
import math
import re
import time
import xml.etree.ElementTree as ET
from datetime import datetime

PACKAGE = "com.kuaishou.nebula"

class PageChanged(RuntimeError):
    pass

def remaining(end, cap=10):
    value = end - time.monotonic()
    if value <= 0:
        raise TimeoutError("步骤总期限已到")
    return min(cap, value)

def unique(boxes, text, contains=False):
    found = [b for b in boxes if (text in b["text"] if contains else b["text"] == text)]
    if len(found) > 1:
        raise RuntimeError("文字定位不唯一: " + text)
    return found[0] if found else None

def center(box):
    x, y, w, h = (box[k] for k in ("x", "y", "w", "h"))
    if not all(math.isfinite(v) for v in (x, y, w, h)):
        raise RuntimeError("文字框坐标不是有限数值")
    if not (0 <= x < x+w <= 1 and 0 <= y < y+h <= 1):
        raise RuntimeError("文字框越界")
    return x+w/2, y+h/2

class Runtime:
    def __init__(self, device, http, serial, root, engine="macocr", total_timeout=480):
        self.device, self.http, self.serial, self.root = device, http, serial, root
        self.seq, self.last, self.recording = 0, None, None
        self.last_image_saved_at = 0.0
        self.engine = engine
        if not 480 <= total_timeout <= 14700:
            raise ValueError("自动化总时限必须在480到14700秒之间")
        self.until = time.monotonic() + total_timeout

    def log(self, message):
        print(time.strftime("%H:%M:%S"), message, flush=True)

    def request(self, method, path, end, **kwargs):
        response = self.http.request(method, path, timeout=remaining(min(end, self.until)), **kwargs)
        response.raise_for_status()
        return response

    def page(self, end):
        value = self.request("GET", "/api/ui/page-info", end, params={"serial": self.serial}).json()
        if value["package_name"] != PACKAGE:
            raise RuntimeError("前台不是快手极速版，停止设备操作")
        return value

    def read(self, end, region=None):
        end = min(end, self.until)
        before = self.page(end)
        size = before["display"]
        image = self.request("POST", "/api/v1/vision/region-screenshot", end,
                             json={"serial": self.serial, "x": 0, "y": 0,
                                   "width": size["width"], "height": size["height"]}).content
        self.seq += 1
        if self.seq > 5000:
            raise RuntimeError("已达5000次画面识别容量上限")
        prefix = self.root / f"{self.seq:03}"
        image_path = None
        if time.monotonic() - self.last_image_saved_at >= 60:
            image_path = prefix.with_suffix(".png")
            image_path.write_bytes(image)
            self.last_image_saved_at = time.monotonic()
        upload, bbox = image, [0, 0, 1, 1]
        if region is not None:
            import cv2
            import numpy as np
            x, y, w, h = region
            if not (0 <= x < x+w <= 1 and 0 <= y < y+h <= 1):
                raise RuntimeError("OCR裁剪区域不合法")
            pixels = cv2.imdecode(np.frombuffer(image, dtype=np.uint8), cv2.IMREAD_COLOR)
            ih, iw = pixels.shape[:2]
            x1, y1, x2, y2 = round(x*iw), round(y*ih), round((x+w)*iw), round((y+h)*ih)
            ok, encoded = cv2.imencode(".png", pixels[y1:y2, x1:x2])
            if not ok:
                raise RuntimeError("OCR裁剪编码失败")
            upload, bbox = encoded.tobytes(), [x1/iw, y1/ih, (x2-x1)/iw, (y2-y1)/ih]
        ocr = self.request("POST", "/api/vision/ocr", end,
                           data={"engine": self.engine, "bbox": json.dumps(bbox)},
                           files={"file": ("screen.png", upload, "image/png")}).json()
        after = self.page(end)
        value = {"before": before, "after": after, "ocr": ocr,
                 "deadline": end, "observed_at": time.monotonic(),
                 "image": str(image_path) if image_path else None}
        prefix.with_suffix(".json").write_text(json.dumps(value, ensure_ascii=False, indent=2))
        for key in ("package_name", "activity", "component", "window_name", "display"):
            if before[key] != after[key]:
                raise PageChanged("识别期间页面发生变化，丢弃本次定位")
        remaining(end)
        self.last = value
        return value

    def tap(self, value, box, end):
        if value is not self.last or time.monotonic()-value["observed_at"] > 2:
            raise RuntimeError("定位结果已经失效")
        self.log("点击 " + box["text"])
        self.device.tap(center(box), timeout=remaining(min(end, self.until), 5))
        self.last = None

    def video_xml(self, end):
        self.log("OCR不确定，使用当前XML精确核验三项顶部导航")
        before = self.page(end)
        response = self.request("GET", "/api/ui/xml", end, params={"serial": self.serial}).json()
        after = self.page(end)
        for key in ("package_name", "activity", "component", "window_name", "display"):
            if before[key] != after[key]:
                raise PageChanged("视频XML采集期间页面变化")
        def stamp(v):
            return datetime.fromisoformat(v.replace("Z", "+00:00"))
        if response["serial"] != self.serial or not stamp(before["fetched_at"]) <= stamp(response["captured_at"]) <= stamp(after["fetched_at"]):
            raise RuntimeError("视频XML设备或采集时间不匹配")
        (self.root/f"video-xml-{self.seq:03}.json").write_text(
            json.dumps({"before": before, "response": response, "after": after}, ensure_ascii=False))
        width, height = response["display_size"]["width"], response["display_size"]["height"]
        positions = {}
        for node in ET.fromstring(response["xml"]).iter("node"):
            a = node.attrib
            label = a["content-desc"]
            if label not in {"生活", "关注", "发现"} or a["package"] != PACKAGE or a["enabled"] != "true":
                continue
            match = re.fullmatch(r"\[(\d+),(\d+)\]\[(\d+),(\d+)\]", a["bounds"])
            if match is None:
                raise RuntimeError("视频导航bounds不合法")
            x1, y1, x2, y2 = map(int, match.groups())
            if 0 <= x1 < x2 <= width and 0 <= y1 < y2 <= .13*height:
                if label in positions:
                    raise RuntimeError("顶部导航不唯一")
                positions[label] = x1
        remaining(end)
        return set(positions) == {"生活", "关注", "发现"} and positions["生活"] < positions["关注"] < positions["发现"]

    def navigation(self, label, end):
        self.log("通过当前控件进入 " + label)
        before = self.page(end)
        response = self.request("GET", "/api/ui/xml", end, params={"serial": self.serial}).json()
        after = self.page(end)
        for key in ("package_name", "activity", "component", "window_name", "display"):
            if before[key] != after[key]:
                raise PageChanged("导航定位期间页面变化")
        def stamp(v):
            return datetime.fromisoformat(v.replace("Z", "+00:00"))
        if not stamp(before["fetched_at"]) <= stamp(response["captured_at"]) <= stamp(after["fetched_at"]):
            raise RuntimeError("XML不在本次采集时间窗口内")
        if response["serial"] != self.serial:
            raise RuntimeError("XML设备不匹配")
        (self.root/f"navigation-{self.seq:03}-{label}.json").write_text(
            json.dumps({"before": before, "response": response, "after": after}, ensure_ascii=False))
        nodes = [n for n in ET.fromstring(response["xml"]).iter("node")
                 if n.attrib["content-desc"] == label and n.attrib["package"] == PACKAGE
                 and n.attrib["enabled"] == "true" and n.attrib["clickable"] == "true"]
        if len(nodes) != 1:
            raise RuntimeError("导航控件不唯一: " + label)
        match = re.fullmatch(r"\[(\d+),(\d+)\]\[(\d+),(\d+)\]", nodes[0].attrib["bounds"])
        if match is None:
            raise RuntimeError("导航bounds不合法")
        x1, y1, x2, y2 = map(int, match.groups())
        width, height = response["display_size"]["width"], response["display_size"]["height"]
        if not (0 <= x1 < x2 <= width and 0 <= y1 < y2 <= height):
            raise RuntimeError("导航控件越界")
        self.device.tap(((x1+x2)/2/width, (y1+y2)/2/height), timeout=remaining(end, 5))
        self.last = None

    def prepare(self, schema):
        required = {"/api/ui/page-info": "get", "/api/ui/xml": "get", "/api/vision/ocr": "post",
                    "/api/v1/vision/region-screenshot": "post", "/api/recordings": "post",
                    "/api/adb/commands": "post"}
        for path, method in required.items():
            if method not in schema["paths"][path]:
                raise RuntimeError("服务缺少必要能力: " + path)
        self.log("通过当前SDK ADB唤醒并请求解除锁屏")
        self.device.adb(["shell", "input", "keyevent", "KEYCODE_WAKEUP"], timeout=5)
        self.device.adb(["shell", "wm", "dismiss-keyguard"], timeout=5)
        self.log("核对快手极速版安装状态并启动")
        installed = self.device.adb(["shell", "pm", "path", PACKAGE], timeout=10)
        if not installed.stdout.startswith("package:"):
            raise RuntimeError("快手极速版未安装")
        self.device.adb(["shell", "monkey", "-p", PACKAGE, "-c",
                         "android.intent.category.LAUNCHER", "1"], timeout=15)

    def start_recording(self, max_duration_ms=600000):
        result = self.request("POST", "/api/recordings", self.until,
                              params={"serial": self.serial},
                              json={"max_duration_ms": max_duration_ms, "record_audio": False}).json()
        self.recording = result["recording_id"]
        (self.root/"recording-id.txt").write_text(self.recording)

    def stop_recording(self):
        if self.recording is None:
            return
        end = time.monotonic()+180
        self.until = end
        path = "/api/recordings/" + self.recording
        self.request("POST", path+"/stop", end, params={"serial": self.serial})
        while True:
            value = self.request("GET", path, end, params={"serial": self.serial}).json()
            if value["status"] in {"completed", "stopped"}:
                break
            if value["status"] == "failed":
                raise RuntimeError("录像失败: " + value["failure_reason"])
            time.sleep(min(.5, remaining(end)))
        import shutil
        if value["bytes_written"] > max(0, shutil.disk_usage(self.root).free - 1024*1024*1024):
            raise RuntimeError("当前磁盘剩余空间不足以安全保存完整录像")
        download_end = time.monotonic() + min(14400, max(180, value["bytes_written"]/(5*1024*1024)+120))
        self.until = download_end
        video = self.root/"session.mp4"
        with self.http.stream("GET", path+"/download", params={"serial": self.serial},
                              timeout=remaining(download_end)) as response, video.open("wb") as stream:
            response.raise_for_status()
            count = 0
            for chunk in response.iter_bytes():
                remaining(download_end)
                count += len(chunk)
                if count > value["bytes_written"]:
                    raise RuntimeError("录像下载超过服务端记录大小")
                stream.write(chunk)
        if count != value["bytes_written"]:
            raise RuntimeError("下载录像大小与服务端记录不一致")
        import av
        with av.open(str(video)) as container:
            first = next(container.decode(video=0))
            if first.width <= 0 or first.height <= 0:
                raise RuntimeError("录像无法解码")
        self.log("录像保存并通过解码检查")
