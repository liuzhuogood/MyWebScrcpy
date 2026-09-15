#!/usr/bin/env python3
"""MyWebScrcpy 内嵌模板匹配进程。

Go 通过 subprocess 启动本脚本，stdin 输入 scrcpy H264 帧（支持 CABAC），
PyAV 解码成 BGR 帧，cv2 模板匹配 data/templates 下的启用模板，
结果以 JSON 行输出到 stdout，Go 据此更新 worker 状态并广播。

stdin 协议（Go -> Python），每条消息：
    [4B 大端体长 N][N 字节体]
    体 = [4B serialLen][serial 字节][1B kind][8B frameID][payload 字节]

stdout 协议（Python -> Go），每行一条 JSON：
    {"device_id": serial, "frame_id": n,
     "matches": [{"name":..., "score":..., "x":..., "y":..., "w":..., "h":...}]}
    x/y/w/h 均为归一化坐标 (0..1)，与 MyWebScrcpy MatchResult 一致。
"""
from __future__ import annotations

import argparse
import io
import json
import os
import struct
import sys
import time
from pathlib import Path
from typing import Any

import cv2
import numpy as np
import av

META_NAMES = ("meta.json", "target.json")
IMAGE_EXT = (".png", ".jpg", ".jpeg", ".webp", ".bmp")
RELOAD_CHECK_INTERVAL = 1.0


# ---------------------------------------------------------------- H264 decode
def avcc_description_to_annexb(data: bytes) -> bytes:
    if len(data) < 7 or data[0] != 1:
        return annexb_nals(data)
    pos, count = 6, data[5] & 31
    out = bytearray()
    for group in range(2):
        for _ in range(count):
            if pos + 2 > len(data):
                return data
            size = int.from_bytes(data[pos:pos + 2], "big")
            pos += 2
            if pos + size > len(data):
                return data
            out += b"\x00\x00\x00\x01" + data[pos:pos + size]
            pos += size
        if group == 0:
            if pos >= len(data):
                break
            count = data[pos]
            pos += 1
    return bytes(out) or data


def annexb_nals(data: bytes) -> bytes:
    if data.startswith((b"\x00\x00\x00\x01", b"\x00\x00\x01")):
        return data
    out = bytearray()
    pos = 0
    while pos + 4 <= len(data):
        size = int.from_bytes(data[pos:pos + 4], "big")
        pos += 4
        if size <= 0 or pos + size > len(data):
            return data
        out += b"\x00\x00\x00\x01" + data[pos:pos + size]
        pos += size
    return bytes(out) if pos == len(data) and out else data


class FrameDecoder:
    """按 serial 维护独立的 H264 解码器。"""

    def __init__(self) -> None:
        self.codec: av.CodecContext | None = None

    def decode(self, kind: int, payload: bytes) -> list[np.ndarray]:
        if kind == 0:
            self.codec = av.CodecContext.create("h264", "r")
            self.codec.extradata = avcc_description_to_annexb(payload)
            return []
        if kind not in (1, 2) or self.codec is None:
            return []
        try:
            frames = self.codec.decode(av.Packet(annexb_nals(payload)))
        except av.error.InvalidDataError:
            return []
        out = []
        for f in frames:
            if f is None:
                continue
            img = f.to_ndarray(format="bgr24")
            if img.size == 0:
                continue
            out.append(img)
        return out


# ------------------------------------------------------------------ templates
class Template:
    __slots__ = ("name", "image", "threshold", "method", "grayscale")

    def __init__(self, name: str, image: np.ndarray, threshold: float,
                 method: int, grayscale: bool) -> None:
        self.name = name
        self.image = image
        self.threshold = threshold
        self.method = method
        self.grayscale = grayscale


METHODS = {
    "TM_CCOEFF_NORMED": cv2.TM_CCOEFF_NORMED,
    "TM_CCORR_NORMED": cv2.TM_CCORR_NORMED,
    "TM_SQDIFF_NORMED": cv2.TM_SQDIFF_NORMED,
}


class TemplateStore:
    """从 data/templates 目录加载模板，轮询热更新。"""

    def __init__(self, root: Path) -> None:
        self.root = Path(root)
        self._templates: dict[str, list[Template]] = {}
        # serial -> {tmpl_dir_name: (signature, [Template, ...])}
        self._cache: dict[str, dict[str, tuple[float, list[Template]]]] = {}
        self._last_check = 0.0
        self._scan(force=True)
        self._last_check = time.time()

    def _dir_sig(self, tmpl_dir: Path) -> float:
        """模板子目录签名：目录及文件最大 mtime；不存在返回负数。"""
        try:
            best = tmpl_dir.stat().st_mtime
        except OSError:
            return -1.0
        try:
            for p in tmpl_dir.iterdir():
                if not p.is_file():
                    continue
                try:
                    m = p.stat().st_mtime
                except OSError:
                    continue
                if m > best:
                    best = m
        except OSError:
            pass
        return best

    def _scan(self, force: bool = False) -> None:
        try:
            entries = [e for e in self.root.iterdir() if e.is_dir()]
        except OSError:
            return
        seen_serials: set[str] = set()
        for entry in entries:
            serial = entry.name
            seen_serials.add(serial)
            try:
                tmpl_dirs = [p for p in entry.iterdir() if p.is_dir()]
            except OSError:
                continue
            cache = self._cache.setdefault(serial, {})
            seen_tmpls: set[str] = set()
            for tmpl_dir in tmpl_dirs:
                name = tmpl_dir.name
                seen_tmpls.add(name)
                sig = self._dir_sig(tmpl_dir)
                if sig < 0:
                    cache.pop(name, None)
                    continue
                old = cache.get(name)
                if not force and old is not None and old[0] == sig:
                    continue
                cache[name] = (sig, self._load_one(tmpl_dir))
            for name in list(cache):
                if name not in seen_tmpls:
                    del cache[name]
            self._templates[serial] = [
                t for _, lst in cache.items() for t in lst
            ]
        for serial in list(self._cache):
            if serial not in seen_serials:
                del self._cache[serial]
                self._templates.pop(serial, None)

    def _load_one(self, tmpl_dir: Path) -> list[Template]:
        templates: list[Template] = []
        meta: dict[str, Any] = {}
        for name in META_NAMES:
            f = tmpl_dir / name
            if f.is_file():
                try:
                    meta = json.loads(f.read_text(encoding="utf-8"))
                    break
                except (OSError, ValueError):
                    meta = {}
        if not meta.get("enabled", True):
            return templates
        name = str(meta.get("name") or tmpl_dir.name)
        threshold = float(meta.get("threshold", 0.8))
        method = METHODS.get(str(meta.get("method", "TM_CCOEFF_NORMED")),
                             cv2.TM_CCOEFF_NORMED)
        grayscale = bool(meta.get("grayscale", True))
        img_path = None
        try:
            candidates = sorted(tmpl_dir.iterdir())
        except OSError:
            return templates
        for p in candidates:
            if p.is_file() and p.suffix.lower() in IMAGE_EXT:
                img_path = p
                break
        if img_path is None:
            return templates
        img = cv2.imread(str(img_path), cv2.IMREAD_UNCHANGED)
        if img is None or img.size == 0:
            return templates
        templates.append(Template(name, img, threshold, method, grayscale))
        return templates

    def _load_templates_dir(self, directory: Path) -> list[Template]:
        templates: list[Template] = []
        try:
            tmpl_dirs = [p for p in directory.iterdir() if p.is_dir()]
        except OSError:
            return templates
        for tmpl_dir in tmpl_dirs:
            templates.extend(self._load_one(tmpl_dir))
        return templates

    def templates_for(self, serial: str) -> list[Template]:
        self.maybe_reload()
        # 设备模板 + 全局模板。目录名把 serial 的冒号替换为下划线
        # (10.0.0.30:5555 -> 10.0.0.30_5555)，两种形式都要尝试。
        result: list[Template] = []
        keys = {serial, serial.replace(":", "_"), "global"}
        for key in keys:
            result.extend(self._templates.get(key, []))
        return result

    def maybe_reload(self) -> None:
        now = time.time()
        if now - self._last_check < RELOAD_CHECK_INTERVAL:
            return
        self._last_check = now
        self._scan()

    def reload_if_changed(self) -> None:
        self.maybe_reload()

    def reload(self) -> None:
        self._scan(force=True)
        self._last_check = time.time()


# ------------------------------------------------------------------ matching
def _match_templates(frame: np.ndarray, templates: list[Template],
                     max_results: int = 8) -> list[dict[str, Any]]:
    fh, fw = frame.shape[:2]
    out: list[dict[str, Any]] = []
    for t in templates:
        if t.image.ndim == 3 and t.image.shape[2] == 4:
            img = t.image[:, :, :3]
        else:
            img = t.image
        if t.grayscale:
            src = cv2.cvtColor(frame, cv2.COLOR_BGR2GRAY)
            tpl = cv2.cvtColor(img, cv2.COLOR_BGR2GRAY) if img.ndim == 3 else img
        else:
            src = frame
            tpl = img
        if tpl.ndim == 3 and src.ndim == 2:
            src = cv2.cvtColor(frame, cv2.COLOR_BGR2GRAY)
            tpl = cv2.cvtColor(img, cv2.COLOR_BGR2GRAY)
        tw, th = tpl.shape[1], tpl.shape[0]
        if tw > fw or th > fh:
            continue
        try:
            res = cv2.matchTemplate(src, tpl, t.method)
        except cv2.error:
            continue
        if t.method == cv2.TM_SQDIFF_NORMED:
            scores = 1.0 - res
        else:
            scores = res
        # 贪心提取超阈值峰值 + 邻域抑制（参考 opencv_match）
        remaining = scores.copy()
        while len(out) < max_results:
            _min_val, max_val, _min_loc, max_loc = cv2.minMaxLoc(remaining)
            if max_val < t.threshold:
                break
            x, y = max_loc
            left = max(0, x - tw + 1)
            top = max(0, y - th + 1)
            right = min(remaining.shape[1], x + tw)
            bottom = min(remaining.shape[0], y + th)
            remaining[top:bottom, left:right] = -np.inf
            out.append({
                "name": t.name,
                "score": round(float(max_val), 4),
                "x": round(x / fw, 6),
                "y": round(y / fh, 6),
                "w": round(tw / fw, 6),
                "h": round(th / fh, 6),
            })
    return out


# ------------------------------------------------------------------- stdio io
def read_frame(stream: io.BufferedReader) -> tuple[str, int, int, bytes] | None:
    """读一条 Go 消息，返回 (serial, kind, frame_id, payload) 或 None(EOF)。"""
    header = stream.read(4)
    if not header:
        return None
    if len(header) < 4:
        return None
    n = struct.unpack(">I", header)[0]
    body = stream.read(n)
    if len(body) < n:
        return None
    serial_len = struct.unpack(">I", body[0:4])[0]
    serial = body[4:4 + serial_len].decode("utf-8", errors="replace")
    offset = 4 + serial_len
    kind = body[offset]
    frame_id = struct.unpack(">Q", body[offset + 1:offset + 9])[0]
    payload = body[offset + 9:]
    return serial, kind, frame_id, payload


def emit(device_id: str, frame_id: int, matches: list[dict[str, Any]]) -> None:
    sys.stdout.write(json.dumps({
        "device_id": device_id,
        "frame_id": frame_id,
        "matches": matches,
    }) + "\n")
    sys.stdout.flush()


def main() -> int:
    parser = argparse.ArgumentParser(description="MyWebScrcpy template matcher")
    parser.add_argument("--templates-dir", required=True)
    args = parser.parse_args()

    store = TemplateStore(Path(args.templates_dir))
    decoders: dict[str, FrameDecoder] = {}
    stdin = sys.stdin.buffer

    # 首行握手：Go 会先发一条 {serial:"",kind:254,payload:b"ready"} 确认就绪
    while True:
        item = read_frame(stdin)
        if item is None:
            return 0
        serial, kind, frame_id, payload = item
        store.reload_if_changed()
        if kind == 254:  # control: ready
            emit(serial, 0, [])
            continue
        dec = decoders.get(serial)
        if dec is None:
            dec = FrameDecoder()
            decoders[serial] = dec
        frames = dec.decode(kind, payload)
        for frame in frames:
            templates = store.templates_for(serial)
            matches = _match_templates(frame, templates)
            emit(serial, frame_id, matches)
    return 0


if __name__ == "__main__":
    sys.exit(main())
