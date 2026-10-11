from __future__ import annotations

import json
import logging
import ssl
import urllib.error
import urllib.request
from dataclasses import dataclass, field
from pathlib import Path
from typing import Dict, List, Optional

import cv2
import numpy as np

logger = logging.getLogger("template_matcher")


@dataclass
class TemplateItem:
    id: str
    name: str
    serial: str
    threshold: float = 0.8
    grayscale: bool = True
    scales: List[float] = field(default_factory=lambda: [1.0])
    image: Optional[np.ndarray] = None
    gray_image: Optional[np.ndarray] = None
    width: int = 0
    height: int = 0
    updated_at: str = ""


class TemplateLoader:
    """模板加载器：默认通过 HTTP API 动态获取模板列表与图片；支持本地目录离线加载兜底"""

    def __init__(
        self,
        base_url: str = "http://127.0.0.1:8080",
        serial: str = "",
        templates_root: Optional[str | Path] = None,
        from_disk: bool = False,
        insecure: bool = False,
    ):
        self.base_url = base_url.rstrip("/")
        self.serial = serial
        self.templates_root = Path(templates_root) if templates_root else None
        self.from_disk = from_disk
        self.insecure = insecure

        # 内存缓存，避免每次轮询都重复下载未变更的模板图片
        self._cache: Dict[str, TemplateItem] = {}
        self.templates: List[TemplateItem] = []

    def _get_http_url(self) -> str:
        url = self.base_url
        if url.startswith("ws://"):
            return "http://" + url[5:]
        if url.startswith("wss://"):
            return "https://" + url[6:]
        if not url.startswith("http://") and not url.startswith("https://"):
            return "http://" + url
        return url

    def load_all(self, global_only: bool = False) -> List[TemplateItem]:
        """统一加载入口：优先走 HTTP 接口，失败或指定时走本地磁盘"""
        if self.from_disk:
            return self._load_from_disk(global_only=global_only)

        try:
            items = self._load_from_api(global_only=global_only)
            self.templates = items
            return items
        except Exception as e:
            logger.warning("通过 API 获取模板失败 (%s)，尝试回退至本地磁盘...", e)
            if self.templates_root and self.templates_root.exists():
                return self._load_from_disk(global_only=global_only)
            return self.templates

    def _load_from_api(self, global_only: bool = False) -> List[TemplateItem]:
        http_base = self._get_http_url()
        serial_param = "global" if global_only else (self.serial or "all")
        list_url = f"{http_base}/api/templates?serial={urllib.parse.quote(serial_param)}"

        ctx = None
        if self.insecure:
            ctx = ssl.create_default_context()
            ctx.check_hostname = False
            ctx.verify_mode = ssl.CERT_NONE

        req = urllib.request.Request(list_url, headers={"User-Agent": "MyWebScrcpy-TemplateMatcher/1.0"})
        with urllib.request.urlopen(req, context=ctx, timeout=10) as resp:
            data = json.loads(resp.read().decode("utf-8"))

        raw_list = data.get("templates", [])
        new_items: List[TemplateItem] = []

        for item_data in raw_list:
            if not item_data.get("enabled", True):
                continue

            tmpl_id = item_data.get("id")
            if not tmpl_id:
                continue

            name = item_data.get("name") or tmpl_id
            threshold = float(item_data.get("threshold", 0.8))
            grayscale = bool(item_data.get("grayscale", True))
            scales = item_data.get("scales", [1.0])
            if not isinstance(scales, list) or not scales:
                scales = [1.0]
            scales = [float(s) for s in scales if float(s) > 0]
            updated_at = str(item_data.get("updated_at", ""))

            # 检查本地缓存是否存在且图片未更新
            cached = self._cache.get(tmpl_id)
            if cached and cached.updated_at == updated_at and cached.image is not None:
                cached.name = name
                cached.threshold = threshold
                cached.grayscale = grayscale
                cached.scales = scales
                new_items.append(cached)
                continue

            # 从接口下载图片二进制
            img_url = f"{http_base}/api/templates/{urllib.parse.quote(tmpl_id)}/image"
            try:
                img_req = urllib.request.Request(img_url, headers={"User-Agent": "MyWebScrcpy-TemplateMatcher/1.0"})
                with urllib.request.urlopen(img_req, context=ctx, timeout=10) as img_resp:
                    img_bytes = img_resp.read()

                img_np = np.frombuffer(img_bytes, dtype=np.uint8)
                img_bgr = cv2.imdecode(img_np, cv2.IMREAD_COLOR)
                if img_bgr is None:
                    continue

                h, w = img_bgr.shape[:2]
                gray = cv2.cvtColor(img_bgr, cv2.COLOR_BGR2GRAY)

                loaded = TemplateItem(
                    id=tmpl_id,
                    name=name,
                    serial=item_data.get("serial", ""),
                    threshold=threshold,
                    grayscale=grayscale,
                    scales=scales,
                    image=img_bgr,
                    gray_image=gray,
                    width=w,
                    height=h,
                    updated_at=updated_at,
                )
                self._cache[tmpl_id] = loaded
                new_items.append(loaded)
            except Exception as exc:
                logger.warning("下载模板图片失败 [id=%s]: %s", tmpl_id, exc)

        logger.info("从 API 成功同步 %d 个模板 (总数: %d)", len(new_items), len(raw_list))
        return new_items

    def _load_from_disk(self, global_only: bool = False) -> List[TemplateItem]:
        """本地目录离线扫描"""
        items: List[TemplateItem] = []
        if not self.templates_root or not self.templates_root.exists():
            logger.warning("模板目录不存在: %s", self.templates_root)
            self.templates = []
            return items

        dirs_to_scan = [self.templates_root / "global"]
        if not global_only and self.serial and self.serial != "global":
            safe_serial = self.serial.replace(":", "_")
            dirs_to_scan.append(self.templates_root / safe_serial)
            if safe_serial != self.serial:
                dirs_to_scan.append(self.templates_root / self.serial)

        seen_tmpl_ids = set()
        for scope_dir in dirs_to_scan:
            if not scope_dir.is_dir():
                continue
            for tmpl_dir in scope_dir.iterdir():
                if not tmpl_dir.is_dir() or tmpl_dir.name in seen_tmpl_ids:
                    continue
                item = self._load_template_item_disk(tmpl_dir)
                if item:
                    seen_tmpl_ids.add(tmpl_dir.name)
                    items.append(item)

        self.templates = items
        logger.info("从本地磁盘成功加载 %d 个模板", len(items))
        return items

    def _load_template_item_disk(self, tmpl_dir: Path) -> Optional[TemplateItem]:
        img_path = tmpl_dir / "template.png"
        if not img_path.is_file():
            pngs = list(tmpl_dir.glob("*.png"))
            if not pngs:
                return None
            img_path = pngs[0]

        img_bgr = cv2.imread(str(img_path))
        if img_bgr is None:
            return None

        h, w = img_bgr.shape[:2]
        gray = cv2.cvtColor(img_bgr, cv2.COLOR_BGR2GRAY)

        meta_path = tmpl_dir / "meta.json"
        target_path = tmpl_dir / "target.json"
        cfg = {}
        if meta_path.is_file():
            try:
                cfg = json.loads(meta_path.read_text(encoding="utf-8"))
            except Exception:
                pass
        elif target_path.is_file():
            try:
                cfg = json.loads(target_path.read_text(encoding="utf-8"))
            except Exception:
                pass

        enabled = cfg.get("enabled", True)
        if not enabled:
            return None

        name = cfg.get("name") or tmpl_dir.name
        threshold = float(cfg.get("threshold", 0.8))
        grayscale = bool(cfg.get("grayscale", True))
        scales = cfg.get("scales", [1.0])
        if not isinstance(scales, list) or not scales:
            scales = [1.0]
        scales = [float(s) for s in scales if float(s) > 0]

        return TemplateItem(
            id=tmpl_dir.name,
            name=name,
            serial=cfg.get("serial", ""),
            threshold=threshold,
            grayscale=grayscale,
            scales=scales,
            image=img_bgr,
            gray_image=gray,
            width=w,
            height=h,
        )
