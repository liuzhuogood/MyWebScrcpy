#!/usr/bin/env python3
"""Indra OCR sidecar: /upload returns pixel boxes in the input image coordinate system."""
import argparse
import asyncio
from concurrent.futures import ThreadPoolExecutor
from contextlib import asynccontextmanager
import logging
import math
import os
import time

import cv2
from fastapi import FastAPI, File, HTTPException, UploadFile
import numpy as np
import uvicorn

LOG = logging.getLogger("indraocr-server")
MAX_BYTES = 16 * 1024 * 1024
MAX_PIXELS = 25_000_000


def recognize(engine, image):
    height, width = image.shape[:2]
    page = engine(cv2.cvtColor(image, cv2.COLOR_BGR2GRAY), turn=False)
    if page.rotation != 0:
        raise ValueError("Unexpected OCR rotation")
    boxes = []
    for line in page.lines:
        if not line.text.strip():
            continue
        x0, y0, x1, y1 = map(float, line.box)
        confidence = float(line.conf)
        if (not all(math.isfinite(v) for v in (x0, y0, x1, y1, confidence))
                or not 0 <= confidence <= 1):
            raise ValueError("Invalid OCR box or confidence")
        x0, x1 = max(0., min(width, x0)), max(0., min(width, x1))
        y0, y1 = max(0., min(height, y0)), max(0., min(height, y1))
        if x1 > x0 and y1 > y0:
            boxes.append(dict(text=line.text, confidence=confidence, x=x0, y=y0, w=x1-x0, h=y1-y0))
    return dict(ocr_result="\n".join(b["text"] for b in boxes), ocr_boxes=boxes,
                image_width=width, image_height=height)


def create_app(model_dir=None, threads=4, timeout=30., engine_factory=None):
    @asynccontextmanager
    async def lifespan(app):
        from indra_ocr import OrtOCR, model_dir as resolve_model_dir

        app.state.executor = ThreadPoolExecutor(max_workers=1, thread_name_prefix="indra-ocr")
        app.state.gate = asyncio.Lock()
        try:
            LOG.info("Loading Indra OCR threads=%d", threads)
            factory = engine_factory if engine_factory is not None else lambda: OrtOCR(resolve_model_dir(model_dir), threads=threads)
            app.state.engine = await asyncio.get_running_loop().run_in_executor(app.state.executor, factory)
            LOG.info("Indra OCR ready pid=%d", os.getpid())
            yield
        finally:
            app.state.executor.shutdown(wait=False, cancel_futures=True)

    app = FastAPI(title="Indra OCR", lifespan=lifespan)

    @app.get("/health")
    async def health():
        return dict(status="ok", ready=True, engine="indraocr", pid=os.getpid())

    @app.post("/upload")
    async def upload(file: UploadFile = File(...)):
        if app.state.gate.locked():
            raise HTTPException(503, "Indra OCR is busy")
        await app.state.gate.acquire()
        future = None
        started = time.monotonic()
        try:
            data = await file.read(MAX_BYTES + 1)
            if len(data) > MAX_BYTES:
                raise HTTPException(413, "Image exceeds 16 MiB")
            image = cv2.imdecode(np.frombuffer(data, dtype=np.uint8), cv2.IMREAD_COLOR) if data else None
            if image is None:
                raise HTTPException(400, "Invalid image")
            if image.shape[0] * image.shape[1] > MAX_PIXELS:
                raise HTTPException(413, "Image exceeds 25 megapixels")
            future = asyncio.get_running_loop().run_in_executor(app.state.executor, recognize, app.state.engine, image)
            result = await asyncio.wait_for(asyncio.shield(future), timeout)
            LOG.info("OCR completed boxes=%d elapsed_ms=%.1f", len(result["ocr_boxes"]), (time.monotonic()-started)*1000)
            return result
        except asyncio.TimeoutError as exc:
            LOG.warning("OCR timed out after %.1fs", timeout)
            raise HTTPException(504, "Indra OCR timed out") from exc
        except HTTPException:
            raise
        except Exception as exc:
            LOG.exception("OCR failed")
            raise HTTPException(502, "Indra OCR inference failed") from exc
        finally:
            # Cancellation does not stop native inference: retain capacity until it really exits.
            if future is not None and not future.done():
                def finished(result):
                    if not result.cancelled():
                        result.exception()
                    app.state.gate.release()
                future.add_done_callback(finished)
            else:
                app.state.gate.release()
            await file.close()

    return app


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--host", default="127.0.0.1")
    parser.add_argument("--port", type=int, default=8032)
    parser.add_argument("--threads", type=int, default=4)
    parser.add_argument("--model-dir")
    args = parser.parse_args()
    if args.threads < 1:
        parser.error("threads must be positive")
    logging.basicConfig(level=logging.INFO, format="%(asctime)s %(levelname)s %(message)s")
    uvicorn.run(create_app(args.model_dir, args.threads), host=args.host, port=args.port)


if __name__ == "__main__":
    main()
