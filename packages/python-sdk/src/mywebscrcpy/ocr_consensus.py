"""Spatial consensus with one vote per source; confidence never selects text."""
from collections import Counter
from concurrent.futures import wait
from dataclasses import replace
import logging
import math
from statistics import median
import time

from .contracts import Deadline
from .ocr import OCRBox
from .ocr_workers import OCRBusyError, submit_ocr

logger = logging.getLogger(__name__)
ENGINE_TIMEOUT_SECONDS = 5.0


def _iou(a, b):
    width = max(0, min(a.x + a.w, b.x + b.w) - max(a.x, b.x))
    height = max(0, min(a.y + a.h, b.y + b.h) - max(a.y, b.y))
    intersection = width * height
    union = a.w * a.h + b.w * b.h - intersection
    return intersection / union if union > 0 else 0


def _representative(votes, *, latest_text=None):
    counts = Counter(box.text for box in votes)
    # The last source is the most recent observation on a tied vote.
    candidates = [(index, box) for index, box in enumerate(votes)
                  if latest_text is None or box.text == latest_text]
    text = max(candidates, key=lambda item: (counts[item[1].text], item[0]))[1].text
    selected = [box for box in votes if box.text == text]
    geometry = {key: median(getattr(box, key) for box in selected)
                for key in ("x", "y", "w", "h", "pixel_x", "pixel_y", "pixel_w", "pixel_h")}
    for key in ("pixel_x", "pixel_y", "pixel_w", "pixel_h"):
        geometry[key] = round(geometry[key])
    return OCRBox(text, median(box.confidence for box in selected), **geometry)


def fuse_boxes(sources, *, temporal=False):
    """Group by IoU >= .3, deduplicate source votes, then take median geometry.

    Sources are engines within a round, or already fused rounds. Temporal
    presence includes empty rounds in its denominator and requires the last one.
    """
    groups = []
    for source, boxes in enumerate(sources):
        for box in boxes:
            overlaps = [_iou(_representative(list(group.values())), box) for group in groups]
            index = max(range(len(groups)), key=overlaps.__getitem__) if groups else None
            if index is None or overlaps[index] < .3:
                groups.append({source: box})
            elif source not in groups[index]:
                groups[index][source] = box
            # Duplicate detections from a single source cannot add votes or
            # shift the coordinate median by repeating the same observation.
    result = []
    for group in groups:
        if temporal and (len(sources) - 1 not in group or len(group) * 2 < len(sources)):
            continue
        # A region that persists across a page change does not authorize an
        # old transcript: the emitted text must itself occur in the last round.
        latest_text = group[len(sources) - 1].text if temporal else None
        result.append(_representative(list(group.values()), latest_text=latest_text))
    return result


def response_boxes(result, engine, shape):
    """Read the established ocr_result/ocr_boxes contract, without aliases.

    Indra uses pixels. macOCR/PaddleOCR retain the SDK's existing coordinate
    convention (a component > 1 denotes pixels). Parallel outputs are normalized.
    """
    if not isinstance(result["ocr_result"], str) or not isinstance(result["ocr_boxes"], list):
        raise ValueError("OCR 响应结构无效")
    height, width = shape[:2]
    boxes = []
    for raw in result["ocr_boxes"]:
        text = raw["text"]
        if not isinstance(text, str):
            raise ValueError("OCR text 必须为字符串")
        values = {key: raw[key] for key in ("x", "y", "w", "h", "confidence")}
        if any(type(value) not in (int, float) or not math.isfinite(value) or value < 0
               for value in values.values()):
            raise ValueError("OCR 文字框数值无效")
        if values["w"] <= 0 or values["h"] <= 0 or values["confidence"] > 1:
            raise ValueError("OCR 文字框尺寸或置信度无效")
        x, y, w, h = (values[key] for key in ("x", "y", "w", "h"))
        pixels = engine == "indraocr" or (engine != "parallel" and max(x, y, w, h) > 1)
        if pixels:
            px, py, pw, ph = x, y, w, h
            x, y, w, h = x / width, y / height, w / width, h / height
        else:
            px, py, pw, ph = x * width, y * height, w * width, h * height
        if text.strip():
            boxes.append(OCRBox(text.strip(), values["confidence"], x, y, w, h,
                                round(px), round(py), round(pw), round(ph)))
    return boxes


def parallel_ocr(image, configurations, timeout):
    """Three engine requests share six global slots and individual 5s limits."""
    from .ocr import _validate_indra, upload_image
    deadline = Deadline(timeout)
    futures, failures = {}, {}

    def recognize(engine, url, auth, expires):
        remaining = expires - time.monotonic()
        if remaining <= 0:
            raise TimeoutError("timeout")
        result = upload_image(image, url, auth, timeout=remaining)
        if engine == "indraocr":
            _validate_indra(result)
        if engine == "macocr" and result.get("success") is False:
            raise RuntimeError("macOCR 返回失败")
        boxes = response_boxes(result, engine, image.shape)
        return boxes, time.monotonic()

    for engine, url, auth in configurations:
        expires = min(deadline.end, time.monotonic() + ENGINE_TIMEOUT_SECONDS)
        try:
            futures[engine] = (submit_ocr(recognize, engine, url, auth, expires), expires)
        except OCRBusyError as error:
            failures[engine] = str(error)
            logger.warning("Parallel OCR engine rejected: engine=%s busy", engine)
    if futures:
        end = max(expires for _, expires in futures.values())
        wait([future for future, _ in futures.values()], timeout=max(0, end - time.monotonic()))
    successful, engines_used = [], []
    for engine, (future, expires) in futures.items():
        try:
            if not future.done():
                raise TimeoutError("timeout")
            boxes, completed = future.result()
            if completed > expires:
                raise TimeoutError("timeout")
            successful.append((completed, boxes))
            engines_used.append(engine)
        except Exception as error:
            failures[engine] = "timeout" if isinstance(error, TimeoutError) else str(error)
            logger.warning("Parallel OCR engine failed: engine=%s type=%s", engine, type(error).__name__)
    errors = [f"{engine}: {failures[engine]}" for engine, _, _ in configurations if engine in failures]
    if not successful:
        raise RuntimeError("OCR 服务均不可用: " + "；".join(errors))
    boxes = fuse_boxes([boxes for _, boxes in sorted(successful, key=lambda item: item[0])])
    result = {"samples": 1, "engines_used": engines_used,
              "ocr_result": "\n".join(box.text for box in boxes), "ocr_boxes": [
        dict(text=box.text, confidence=box.confidence, x=box.x, y=box.y, w=box.w, h=box.h)
        for box in boxes]}
    # Timed-out futures remain admitted until their actual task completion.
    # Workers only return private data and cannot publish late Device state.
    return result, "parallel", errors


def global_boxes(boxes, bbox):
    x, y, w, h = bbox
    return [replace(box, x=x + box.x*w, y=y + box.y*h, w=box.w*w, h=box.h*h) for box in boxes]
