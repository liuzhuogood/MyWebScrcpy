"""Admission limits must survive callers timing out, across both OCR paths."""
from concurrent.futures import ThreadPoolExecutor
import threading
import time
from unittest import TestCase
from unittest.mock import patch

import numpy as np

from mywebscrcpy.contracts import Deadline
from mywebscrcpy.errors import FrameTimeout
from mywebscrcpy.ocr import perform_ocr
from mywebscrcpy.ocr_sampling import _recognize


def response():
    return {"ocr_result": "", "ocr_boxes": []}


class CapacityTests(TestCase):
    def setUp(self):
        from mywebscrcpy.ocr_workers import OCRWorkerPool
        self.pool = OCRWorkerPool()
        self.release = threading.Event()
        self.image = np.zeros((10, 10, 3), dtype=np.uint8)
        self.addCleanup(self.pool.shutdown)
        self.addCleanup(self.release.set)
        patcher = patch("mywebscrcpy.ocr_workers._pool", self.pool)
        patcher.start()
        self.addCleanup(patcher.stop)

    def block(self):
        self.release.wait(2)
        return response()

    def test_timeouts_keep_all_six_slots_and_repeated_calls_cannot_queue(self):
        from mywebscrcpy.ocr_workers import OCRBusyError
        submitted = []
        original_submit = self.pool._executor.submit
        def submit(*args, **kwargs):
            future = original_submit(*args, **kwargs)
            submitted.append(future)
            return future
        admission = patch.object(self.pool._executor, "submit", side_effect=submit)
        admission.start()
        self.addCleanup(admission.stop)
        with patch("mywebscrcpy.ocr.upload_image", side_effect=lambda *a, **k: self.block()) as upload:
            with self.assertRaisesRegex(RuntimeError, "timeout"):
                perform_ocr(self.image, engine="parallel", timeout=.025)
            for _ in range(3):
                with self.assertRaises(FrameTimeout):
                    _recognize(self.image, Deadline(.025), {"engine": "indraocr"})
            self.assertEqual(upload.call_count, 6)
            started = time.monotonic()
            for _ in range(20):
                with self.assertRaises(OCRBusyError):
                    _recognize(self.image, Deadline(1), {"engine": "indraocr"})
                with self.assertRaisesRegex(RuntimeError, "busy"):
                    perform_ocr(self.image, engine="parallel", timeout=1)
            self.assertLess(time.monotonic()-started, .4)
            self.assertEqual(upload.call_count, 6)
            self.assertEqual(len(self.pool._executor._threads), 6)
            self.assertEqual(self.pool._executor._work_queue.qsize(), 0)
            self.release.set()
            for future in submitted:
                future.result(timeout=.5)
        # Re-admit six blocked tasks into the SAME pool, proving all capacity
        # returns after the abandoned calls finish, without replacing workers.
        hold = threading.Event()
        self.addCleanup(hold.set)
        restored = []
        end = time.monotonic() + .5
        while len(restored) < 6:
            try:
                restored.append(self.pool.submit(hold.wait, 1))
            except OCRBusyError:
                if time.monotonic() >= end:
                    raise
                time.sleep(.001)
        with self.assertRaises(OCRBusyError):
            self.pool.submit(lambda: True)
        hold.set()
        for future in restored:
            future.result(timeout=.5)
        with patch("mywebscrcpy.ocr.upload_image", return_value=response()):
            result, _, errors = perform_ocr(self.image, engine="parallel", timeout=.5)
        self.assertEqual(result["engines_used"], ["indraocr", "macocr", "paddleocr"])
        self.assertFalse(errors)

    def test_concurrent_submitters_cannot_exceed_capacity_and_completion_releases(self):
        from mywebscrcpy.ocr_workers import OCRBusyError
        futures, rejected = [], []
        lock = threading.Lock()
        def submit():
            try:
                future = self.pool.submit(self.block)
                with lock:
                    futures.append(future)
            except OCRBusyError:
                with lock:
                    rejected.append(True)
        with ThreadPoolExecutor(max_workers=20) as callers:
            list(callers.map(lambda _: submit(), range(20)))
        self.assertEqual(len(futures), 6)
        self.assertEqual(len(rejected), 14)
        self.release.set()
        for future in futures:
            future.result(timeout=.5)
        end = time.monotonic() + .5
        while True:
            try:
                self.assertTrue(self.pool.submit(lambda: True).result(timeout=.5))
                break
            except OCRBusyError:
                if time.monotonic() >= end:
                    raise
                time.sleep(.001)

    def test_submit_failure_restores_capacity(self):
        from mywebscrcpy.ocr_workers import OCRBusyError
        with patch.object(self.pool._executor, "submit", side_effect=RuntimeError("closed")):
            for _ in range(8):
                with self.assertRaisesRegex(RuntimeError, "closed"):
                    self.pool.submit(self.block)
        futures = [self.pool.submit(self.block) for _ in range(6)]
        with self.assertRaises(OCRBusyError):
            self.pool.submit(self.block)
        self.release.set()
        for future in futures:
            future.result(timeout=.5)

    def test_parallel_upload_timeout_is_capped_at_five_seconds_and_call_budget(self):
        for budget in (10, .2):
            with self.subTest(budget=budget), patch("mywebscrcpy.ocr.upload_image", return_value=response()) as upload:
                perform_ocr(self.image, engine="parallel", timeout=budget)
            self.assertEqual(upload.call_count, 3)
            for call in upload.call_args_list:
                self.assertGreater(call.kwargs["timeout"], 0)
                self.assertLessEqual(call.kwargs["timeout"], min(5, budget))

    def test_engine_deadline_returns_partial_even_if_upload_ignores_timeout(self):
        def upload(image, url, auth="", timeout=10):
            return response() if "8032" in url else self.block()
        with patch("mywebscrcpy.ocr_consensus.ENGINE_TIMEOUT_SECONDS", .04), \
                patch("mywebscrcpy.ocr.upload_image", side_effect=upload):
            started = time.monotonic()
            result, _, errors = perform_ocr(self.image, engine="parallel", timeout=1)
        self.assertLess(time.monotonic()-started, .25)
        self.assertEqual(result["engines_used"], ["indraocr"])
        self.assertEqual(len(errors), 2)
        self.assertTrue(all("timeout" in error for error in errors))
