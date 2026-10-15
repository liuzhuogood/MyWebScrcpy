#!/usr/bin/env python3
"""PaddleOCR sidecar: 提供与 macOCR /upload 相同契约的本地文字识别 HTTP 服务。

线程模型: paddle CPU 推理在本机的非主线程中会死锁，因此 uvicorn 在后台线程接收请求，
图片经队列交给主线程串行识别后回传。首次启动会在 ~/.paddlex 下载 PP-OCR 模型。
"""

from __future__ import annotations

import argparse
from collections import OrderedDict
from copy import deepcopy
import hashlib
import logging
import os
import queue
import signal
import threading
from typing import Any

import cv2
import numpy as np
import uvicorn
from fastapi import FastAPI, File, HTTPException, UploadFile

_ocr: Any = None
_model_ready: bool = False
_model_error: str | None = None
_work_queue: queue.Queue = queue.Queue()
_result_cache: OrderedDict = OrderedDict()
OCR_CACHE_SIZE = 128
logger = logging.getLogger(__name__)


def get_ocr() -> Any:
    """主线程内懒加载 PaddleOCR; 首次调用会下载模型。"""
    global _ocr
    if _ocr is None:
        from paddleocr import PaddleOCR

        profile = os.environ.get("MYWEBSCRCPY_OCR_PROFILE", "default")
        model_options = {"lang": "ch"}
        if profile == "cpu":
            model_options = {
                "text_detection_model_name": "PP-OCRv6_medium_det",
                "text_recognition_model_name": "PP-OCRv6_small_rec",
            }
        logger.info("PaddleOCR initializing profile=%s cpu_threads=2 mkldnn=False", profile)
        _ocr = PaddleOCR(
            **model_options,
            device="cpu",
            enable_mkldnn=False,
            cpu_threads=2,
            use_doc_orientation_classify=False,
            use_doc_unwarping=False,
            use_textline_orientation=False,
        )
    return _ocr


def run_ocr(image: np.ndarray) -> dict[str, Any]:
    """识别 BGR 图像，返回与 macOCR 兼容的像素坐标文字框。"""
    height, width = image.shape[:2]
    pages = get_ocr().predict(image)
    if isinstance(pages, dict):
        pages = [pages]
    boxes: list[dict[str, Any]] = []
    texts: list[str] = []
    for page in pages or []:
        raw_texts = page.get("rec_texts")
        page_texts = [str(t) for t in raw_texts] if raw_texts is not None else []

        raw_scores = page.get("rec_scores")
        page_scores = [float(s) for s in raw_scores] if raw_scores is not None else []

        raw_polys = page.get("rec_polys")
        if raw_polys is None:
            polys: list[Any] = []
        elif isinstance(raw_polys, np.ndarray):
            polys = [raw_polys[i] for i in range(len(raw_polys))]
        else:
            polys = list(raw_polys)

        texts.extend(page_texts)
        for index, text in enumerate(page_texts):
            if index < len(polys):
                points = np.asarray(polys[index], dtype=float).reshape(-1, 2)
                x = float(points[:, 0].min())
                y = float(points[:, 1].min())
                w = float(points[:, 0].max()) - x
                h = float(points[:, 1].max()) - y
            else:
                x = y = w = h = 0.0
            confidence = page_scores[index] if index < len(page_scores) else 0.0
            boxes.append({
                "text": text,
                "confidence": confidence,
                "x": x,
                "y": y,
                "w": w,
                "h": h,
            })
    return {
        "ocr_result": "\n".join(texts),
        "ocr_boxes": boxes,
        "image_width": width,
        "image_height": height,
    }


def run_cached_ocr(image: np.ndarray) -> dict[str, Any]:
    """主线程复用完全相同的像素结果; 变化的文字始终重新识别。"""
    key = (image.shape, image.dtype.str, hashlib.sha256(image.tobytes()).digest())
    if key in _result_cache:
        _result_cache.move_to_end(key)
        logger.debug("OCR identical-image cache hit shape=%s", image.shape)
        return deepcopy(_result_cache[key])
    result = run_ocr(image)
    _result_cache[key] = deepcopy(result)
    if len(_result_cache) > OCR_CACHE_SIZE:
        _result_cache.popitem(last=False)
    return result


def main_thread_worker(stop_event: threading.Event) -> None:
    """在主线程初始化模型并串行处理所有识别请求。"""
    global _model_ready, _model_error
    try:
        get_ocr()
        _model_ready = True
        logger.info("PaddleOCR 模型加载完成，服务就绪")
    except Exception as exc:
        _model_error = str(exc)
        logger.error("PaddleOCR 模型加载失败: %s", exc)
        return

    while True:
        try:
            image, reply = _work_queue.get(timeout=0.5)
        except queue.Empty:
            if stop_event.is_set():
                return
            continue
        try:
            reply.put(("ok", run_cached_ocr(image)))
        except Exception as exc:
            reply.put(("error", exc))


def submit_ocr(image: np.ndarray, timeout: float = 180) -> dict[str, Any]:
    """把图片交给主线程识别并等待结果。"""
    reply: queue.Queue = queue.Queue(maxsize=1)
    _work_queue.put((image, reply))
    try:
        status, payload = reply.get(timeout=timeout)
    except queue.Empty as exc:
        raise RuntimeError(f"PaddleOCR 处理超时（{timeout} 秒）") from exc
    if status == "error":
        raise payload
    return payload


app = FastAPI(title="PaddleOCR Sidecar", version="1.0.0")


@app.get("/health")
def health() -> dict[str, Any]:
    if not _model_ready:
        if _model_error:
            raise HTTPException(status_code=503, detail=f"Model initialization failed: {_model_error}")
        detail = "Model is initializing"
        raise HTTPException(status_code=503, detail=detail)
    return {"status": "ok", "ready": True}


@app.post("/upload")
def upload(file: UploadFile = File(...)) -> dict[str, Any]:
    """接收 multipart 上传的 PNG，返回 macOCR 兼容的识别结果。"""
    data = file.file.read()
    image = cv2.imdecode(np.frombuffer(data, dtype=np.uint8), cv2.IMREAD_COLOR)
    if image is None:
        raise HTTPException(status_code=400, detail="无法解析上传的图片")
    try:
        return submit_ocr(image)
    except Exception as exc:
        raise HTTPException(status_code=502, detail=f"PaddleOCR 识别失败: {exc}") from exc


def main() -> None:
    parser = argparse.ArgumentParser(description="PaddleOCR sidecar 服务")
    parser.add_argument("--host", default="0.0.0.0")
    parser.add_argument("--port", type=int, default=8031)
    args = parser.parse_args()
    config = uvicorn.Config(app, host=args.host, port=args.port, log_level="info")
    server = uvicorn.Server(config)
    stop_event = threading.Event()

    def request_shutdown(signum: int, frame: Any) -> None:
        server.should_exit = True
        stop_event.set()

    signal.signal(signal.SIGTERM, request_shutdown)
    signal.signal(signal.SIGINT, request_shutdown)

    server_thread = threading.Thread(target=server.run, name="uvicorn", daemon=True)
    server_thread.start()
    main_thread_worker(stop_event)
    server_thread.join(timeout=10)


if __name__ == "__main__":
    main()
