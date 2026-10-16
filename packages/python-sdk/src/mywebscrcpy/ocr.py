"""OCR text detection for MyWebScrcpy Python SDK."""
from __future__ import annotations

import base64
from dataclasses import dataclass, field
import json
import logging
import math
import os
import time
from typing import Any
import urllib.error
import urllib.parse
import urllib.request
import uuid

from .dependencies import vision_dependencies

logger = logging.getLogger(__name__)

DEFAULT_INDRAOCR_URL = "http://127.0.0.1:8032/upload"
DEFAULT_MACOCR_URL = "http://10.0.0.11:8000/upload"
DEFAULT_PADDLEOCR_URL = "http://127.0.0.1:8031/upload"


@dataclass(frozen=True)
class OCRBox:
    text: str
    confidence: float
    x: float
    y: float
    w: float
    h: float
    pixel_x: int = 0
    pixel_y: int = 0
    pixel_w: int = 0
    pixel_h: int = 0

    @property
    def center(self) -> tuple[float, float]:
        return (self.x + self.w / 2.0, self.y + self.h / 2.0)

    def to_object(self) -> dict[str, Any]:
        return {
            "label": self.text,
            "confidence": self.confidence,
            "x": self.x,
            "y": self.y,
            "w": self.w,
            "h": self.h,
            "kind": "text",
        }


@dataclass(frozen=True)
class OCRResult:
    status: str
    matched: bool
    text: str
    boxes: list[OCRBox]
    engine: str
    frame_id: int = 0
    elapsed_ms: int = 0
    fallback_reason: list[str] = field(default_factory=list)

    def find(self, keyword: str) -> list[OCRBox]:
        """按关键词匹配文字框。"""
        target = keyword.casefold()
        return [b for b in self.boxes if target in b.text.casefold()]


def _encode_png(image: Any) -> bytes:
    _, cv2, _ = vision_dependencies()
    ok, encoded = cv2.imencode(".png", image)
    if not ok:
        raise RuntimeError("无法编码图像为 PNG")
    return encoded.tobytes()


def upload_image(image: Any, url: str, auth: str = "", timeout: float = 10.0) -> dict[str, Any]:
    """通过 multipart 上传图像到支持 /upload 契约的 OCR 服务。"""
    encoded_png = _encode_png(image)
    boundary = f"----mywebscrcpy-ocr-{uuid.uuid4().hex}"
    body = (
        f"--{boundary}\r\n"
        'Content-Disposition: form-data; name="file"; filename="ocr.png"\r\n'
        "Content-Type: image/png\r\n\r\n"
    ).encode("utf-8") + encoded_png + f"\r\n--{boundary}--\r\n".encode("utf-8")

    parts = urllib.parse.urlsplit(url)
    url_auth = auth
    if parts.username is not None and not url_auth:
        url_auth = f"{urllib.parse.unquote(parts.username)}:{urllib.parse.unquote(parts.password or '')}"
        host = parts.hostname or ""
        if parts.port is not None:
            host = f"{host}:{parts.port}"
        url = urllib.parse.urlunsplit((parts.scheme, host, parts.path, parts.query, parts.fragment))

    req = urllib.request.Request(
        url,
        data=body,
        headers={
            "Accept": "application/json",
            "Content-Type": f"multipart/form-data; boundary={boundary}",
        },
        method="POST",
    )
    if url_auth:
        token = base64.b64encode(url_auth.encode("utf-8")).decode("utf-8")
        req.add_header("Authorization", f"Basic {token}")

    opener = urllib.request.build_opener(urllib.request.ProxyHandler({}))
    with opener.open(req, timeout=timeout) as resp:
        return json.loads(resp.read().decode("utf-8"))


def perform_ocr(
    image: Any,
    *,
    engine: str = "auto",
    macocr_url: str | None = None,
    macocr_auth: str | None = None,
    paddleocr_url: str | None = None,
    indraocr_url: str | None = None,
    timeout: float = 10.0,
) -> tuple[dict[str, Any], str, list[str]]:
    """调用底层 OCR 引擎识别图像并返回结果、所用引擎与降级原因。"""
    if engine not in {"auto", "indraocr", "macocr", "paddleocr"}:
        raise ValueError("engine 必须是 auto、indraocr、macocr 或 paddleocr")

    mac_url = macocr_url or os.getenv("MACOCR_URL", DEFAULT_MACOCR_URL)
    mac_auth = macocr_auth or os.getenv("MACOCR_AUTH", "")
    paddle_url = paddleocr_url or os.getenv("PADDLEOCR_URL", DEFAULT_PADDLEOCR_URL)

    indra_url = indraocr_url if indraocr_url is not None else os.getenv("INDRAOCR_URL", DEFAULT_INDRAOCR_URL)
    errors: list[str] = []
    result: dict[str, Any] | None = None
    engine_used = engine

    if engine in {"auto", "indraocr"}:
        try:
            res = upload_image(image, indra_url, "", timeout=timeout)
            _validate_indra(res)
            result, engine_used = res, "indraocr"
        except Exception as exc:
            errors.append(f"Indra OCR: {exc}")
            if engine == "indraocr":
                raise RuntimeError(f"Indra OCR 不可用: {exc}") from exc
            logger.warning("Indra OCR unavailable; trying next engine: %s", type(exc).__name__)

    if result is None and engine in {"auto", "macocr"}:
        try:
            res = upload_image(image, mac_url, mac_auth, timeout=timeout)
            if res.get("success") is False:
                raise RuntimeError(str(res.get("message") or "macOCR 返回失败"))
            result = res
            engine_used = "macocr"
        except Exception as exc:
            errors.append(f"macOCR: {exc}")
            if engine == "macocr":
                raise RuntimeError(f"macOCR 不可用: {exc}") from exc

    if result is None:
        try:
            result = upload_image(image, paddle_url, "", timeout=timeout)
            engine_used = "paddleocr"
        except Exception as exc:
            errors.append(f"PaddleOCR: {exc}")
            raise RuntimeError(f"OCR 服务均不可用: {'；'.join(errors)}") from exc

    return result, engine_used, errors


def _validate_indra(result):
    if not isinstance(result["ocr_result"], str) or not isinstance(result["ocr_boxes"], list):
        raise ValueError("Indra OCR 响应结构无效")
    for box in result["ocr_boxes"]:
        if not isinstance(box["text"], str):
            raise ValueError("Indra OCR 文字框缺少 text")
        for key in ("x", "y", "w", "h", "confidence"):
            value = box[key]
            if type(value) not in (int, float) or not math.isfinite(value) or value < 0:
                raise ValueError(f"Indra OCR 文字框 {key} 无效")
            if key in ("w", "h") and value <= 0 or key == "confidence" and value > 1:
                raise ValueError(f"Indra OCR 文字框 {key} 无效")


def crop_image(image: Any, bbox: list[float] | tuple[float, float, float, float]) -> tuple[Any, dict[str, int]]:
    """按归一化坐标 [x, y, w, h] 裁剪图像，返回 (裁剪图像, 像素边界)。"""
    h, w = image.shape[:2]
    x, y, bw, bh = bbox
    left = max(0, min(w - 1, round(x * w)))
    top = max(0, min(h - 1, round(y * h)))
    right = min(w, max(left + 1, round((x + bw) * w)))
    bottom = min(h, max(top + 1, round((y + bh) * h)))
    cropped = image[top:bottom, left:right]
    pixel_bbox = {"x": left, "y": top, "width": right - left, "height": bottom - top}
    return cropped, pixel_bbox
