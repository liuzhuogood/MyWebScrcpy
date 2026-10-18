"""OCR consensus behavior: actual worker overlap, voting and fresh-frame budgets."""
from dataclasses import replace
import threading
import time
from unittest import TestCase
from unittest.mock import MagicMock, patch

import numpy as np

from mywebscrcpy.contracts import Deadline
from mywebscrcpy.device import Device
from mywebscrcpy.errors import FrameTimeout
from mywebscrcpy.models import Frame
from mywebscrcpy.ocr import OCRBox, OCRResult, perform_ocr


def response(text="确定", x=10, confidence=.8):
    return {"ocr_result": text, "ocr_boxes": [] if not text else [
        dict(text=text, confidence=confidence, x=x, y=10, w=30, h=10)]}


class ParallelTests(TestCase):
    def test_real_overlap_and_engine_votes_ignore_confidence(self):
        barrier = threading.Barrier(3)
        def upload(image, url, auth="", timeout=10):
            barrier.wait(timeout=.8)  # A serial implementation cannot pass.
            return response("错误" if "8032" in url else "确定",
                            confidence=.99 if "8032" in url else .1)
        with patch("mywebscrcpy.ocr.upload_image", side_effect=upload) as call:
            result, engine, errors = perform_ocr(np.zeros((100, 100, 3)), engine="parallel", timeout=1)
        self.assertEqual(engine, "parallel")
        self.assertFalse(errors)
        self.assertEqual(call.call_count, 3)
        self.assertEqual([b["text"] for b in result["ocr_boxes"]], ["确定"])
        self.assertEqual(result["samples"], 1)
        self.assertEqual(result["engines_used"], ["indraocr", "macocr", "paddleocr"])

    def test_partial_failure_and_explicit_configuration(self):
        def upload(image, url, auth="", timeout=10):
            if url == "bad":
                raise RuntimeError("offline")
            if url == "mac":
                self.assertEqual(auth, "user:pass")
            return response()
        with patch("mywebscrcpy.ocr.upload_image", side_effect=upload) as call:
            result, _, errors = perform_ocr(np.zeros((100, 100, 3)), engine="parallel",
                indraocr_url="bad", macocr_url="mac", macocr_auth="user:pass", paddleocr_url="paddle")
        self.assertEqual({c.args[1] for c in call.call_args_list}, {"bad", "mac", "paddle"})
        self.assertEqual(len(result["ocr_boxes"]), 1)
        self.assertTrue(any("indraocr" in e.lower() for e in errors))
        self.assertEqual(result["engines_used"], ["macocr", "paddleocr"])

    def test_all_failure_raises_and_empty_success_does_not(self):
        with patch("mywebscrcpy.ocr.upload_image", side_effect=RuntimeError("offline")):
            with self.assertRaisesRegex(RuntimeError, "OCR"):
                perform_ocr(np.zeros((100, 100, 3)), engine="parallel")
        with patch("mywebscrcpy.ocr.upload_image", return_value=response("")):
            result, _, errors = perform_ocr(np.zeros((100, 100, 3)), engine="parallel")
        self.assertEqual(result["ocr_boxes"], [])
        self.assertFalse(errors)

    def test_timeout_returns_completed_result_without_waiting_for_workers(self):
        release = threading.Event()
        barrier = threading.Barrier(3)
        def upload(image, url, auth="", timeout=10):
            barrier.wait(timeout=.5)
            if "8032" not in url:
                release.wait(.8)
            return response()
        try:
            with patch("mywebscrcpy.ocr.upload_image", side_effect=upload):
                started = time.monotonic()
                result, _, errors = perform_ocr(np.zeros((100, 100, 3)), engine="parallel", timeout=.1)
                self.assertLess(time.monotonic() - started, .35)
                self.assertEqual(len(result["ocr_boxes"]), 1)
                self.assertEqual(len(errors), 2)
                self.assertTrue(all("timeout" in e.lower() for e in errors))
        finally:
            release.set()

    def test_all_timeout_raises(self):
        release = threading.Event()
        try:
            with patch("mywebscrcpy.ocr.upload_image", side_effect=lambda *a, **k: release.wait(.8)):
                started = time.monotonic()
                with self.assertRaises((RuntimeError, TimeoutError)):
                    perform_ocr(np.zeros((100, 100, 3)), engine="parallel", timeout=.05)
                self.assertLess(time.monotonic() - started, .3)
        finally:
            release.set()

    def test_malformed_engine_is_partial_failure(self):
        def upload(image, url, auth="", timeout=10):
            return {} if "8000" in url else response()
        with patch("mywebscrcpy.ocr.upload_image", side_effect=upload):
            result, _, errors = perform_ocr(np.zeros((100, 100, 3)), engine="parallel")
        self.assertEqual(len(result["ocr_boxes"]), 1)
        self.assertEqual(len(errors), 1)

    def test_auto_stays_serial_and_explicit_engines_never_fallback(self):
        active, order = 0, []
        def upload(image, url, auth="", timeout=10):
            nonlocal active
            active += 1
            self.assertEqual(active, 1)
            order.append(url)
            active -= 1
            if len(order) < 3:
                raise RuntimeError("offline")
            return response()
        with patch("mywebscrcpy.ocr.upload_image", side_effect=upload):
            perform_ocr(None, indraocr_url="indra", macocr_url="mac", paddleocr_url="paddle")
        self.assertEqual(order, ["indra", "mac", "paddle"])
        for engine in ("indraocr", "macocr", "paddleocr"):
            with self.subTest(engine=engine), patch("mywebscrcpy.ocr.upload_image", side_effect=RuntimeError("offline")) as call:
                with self.assertRaises(RuntimeError):
                    perform_ocr(None, engine=engine)
                self.assertEqual(call.call_count, 1)


class FusionTests(TestCase):
    def fuse(self, rounds, **kwargs):
        from mywebscrcpy.ocr_consensus import fuse_boxes
        return fuse_boxes(rounds, **kwargs)

    def box(self, text="确定", x=.1, confidence=.8):
        return OCRBox(text, confidence, x, .1, .3, .1, round(x*100), 10, 30, 10)

    def test_jitter_dedup_one_vote_per_source_and_median(self):
        boxes = self.fuse([[self.box("错", .09, 1)] * 8,
                           [self.box("确定", .10, .1)], [self.box("确定", .11, .2)]])
        self.assertEqual(len(boxes), 1)
        self.assertEqual(boxes[0].text, "确定")
        self.assertAlmostEqual(boxes[0].x, .105)
        self.assertEqual(boxes[0].pixel_x, 10)

    def test_iou_threshold_and_spatially_distinct_identical_text(self):
        boxes = self.fuse([[self.box(x=.1), self.box(x=.7)], [self.box(x=.11)]])
        self.assertEqual(len(boxes), 2)
        # IoU of equal width boxes shifted by .17 is .13/.47 < .3.
        self.assertEqual(len(self.fuse([[self.box(x=.1)], [self.box(x=.27)]])), 2)

    def test_temporal_half_and_latest_round_requirement(self):
        keep = self.box("新", .1)
        old = self.box("旧", .7)
        boxes = self.fuse([[keep, old], [old], [keep, old], [keep]], temporal=True)
        self.assertEqual([b.text for b in boxes], ["新"])
        self.assertEqual(self.fuse([[keep], [], []], temporal=True), [])
        self.assertEqual(self.fuse([[], [], [keep]], temporal=True), [])
        self.assertEqual(len(self.fuse([[keep], [], [], [keep]], temporal=True)), 1)

    def test_text_tie_prefers_latest_and_round_gets_only_one_vote(self):
        a, b = self.box("a", confidence=1), self.box("b", confidence=.01)
        self.assertEqual(self.fuse([[a], [b]], temporal=True)[0].text, "b")
        first_round = self.fuse([[a], [a], [a]])
        self.assertEqual(self.fuse([first_round, [b], [b]], temporal=True)[0].text, "b")

    def test_old_majority_text_cannot_survive_page_change_at_same_position(self):
        old, new = self.box("旧页面"), self.box("新页面")
        self.assertEqual(self.fuse([[old], [old], [old], [new]], temporal=True)[0].text, "新页面")

    def test_coordinates_ignore_losing_transcripts_including_temporal_majority(self):
        boxes = self.fuse([[self.box("旧", .09)], [self.box("旧", .10)],
                           [self.box("新", .11)]], temporal=True)
        self.assertEqual(boxes[0].text, "新")
        self.assertAlmostEqual(boxes[0].x, .11)
        self.assertEqual(boxes[0].pixel_x, 11)


class SamplingTests(TestCase):
    def setUp(self):
        self.device = Device(MagicMock(), "phone")
        self.session = MagicMock(session_id="s", generation=1, connection_id="c")
        self.image = np.zeros((100, 100, 3), dtype=np.uint8)
        self.ids, self.starts = [], []
        def frame(deadline):
            deadline.remaining(FrameTimeout)
            self.ids.append(len(self.ids) + 1)
            return Frame(self.ids[-1], "s", "phone", 100, 100, 1,
                         time.monotonic(), self.image, 1, "c")
        self.session.frame.side_effect = frame
        self.session.current_frame.side_effect = frame

    def test_parameters_rejected_before_frame_or_upload(self):
        for value in (True, False, -1, 1.5, 5001, "100", None):
            with self.subTest(value=value), patch.object(self.device, "_session") as session:
                with self.assertRaises(ValueError):
                    self.device.ocr(stabilization_ms=value)
                session.assert_not_called()

    def test_metadata_defaults_keep_existing_result_construction(self):
        first = OCRResult("not_found", False, "", [], "auto")
        second = OCRResult("not_found", False, "", [], "auto")
        self.assertEqual(first.samples, 1)
        self.assertEqual(first.engines_used, [])
        first.engines_used.append("indraocr")
        self.assertEqual(second.engines_used, [])

    def test_zero_keeps_current_single_frame(self):
        with patch.object(self.device, "_session", return_value=self.session), patch("mywebscrcpy.ocr.upload_image", return_value=response()):
            result = self.device.ocr(stabilization_ms=0, engine="indraocr")
        self.assertEqual(result.frame_id, 1)
        self.assertEqual(result.samples, 1)
        self.assertEqual(result.engines_used, ["indraocr"])
        self.session.current_frame.assert_called_once()
        self.session.frame.assert_not_called()

    def test_fresh_serial_samples_minimum_interval_and_latest_constraint(self):
        def upload(*args, **kwargs):
            self.starts.append(time.monotonic())
            return response("旧" if len(self.starts) == 1 else "新")
        with patch.object(self.device, "_session", return_value=self.session), patch("mywebscrcpy.ocr.upload_image", side_effect=upload):
            result = self.device.ocr(stabilization_ms=125, engine="indraocr", timeout=1)
        self.assertGreaterEqual(len(self.starts), 2)
        self.assertLessEqual(len(self.starts), 3)
        self.assertTrue(all(b-a >= .048 for a, b in zip(self.starts, self.starts[1:])))
        self.assertEqual([b.text for b in result.boxes], ["新"])
        self.assertEqual(result.frame_id, self.ids[-1])
        self.assertEqual(result.samples, len(self.ids))
        self.assertEqual(result.engines_used, ["indraocr"])
        self.session.current_frame.assert_not_called()

    def test_window_does_not_start_new_round_after_frame_wait(self):
        acquire = self.session.frame.side_effect
        def slow_frame(deadline):
            time.sleep(.035)
            return acquire(deadline)
        self.session.frame.side_effect = slow_frame
        with patch.object(self.device, "_session", return_value=self.session), patch("mywebscrcpy.ocr.upload_image", return_value=response()) as upload:
            with self.assertRaises(FrameTimeout):
                self.device.ocr(stabilization_ms=20, timeout=.2)
        upload.assert_not_called()

    def test_started_round_can_finish_after_window_and_mark_uses_latest_frame(self):
        def upload(*args, **kwargs):
            time.sleep(.04)
            return response()
        with patch.object(self.device, "_session", return_value=self.session), patch("mywebscrcpy.ocr.upload_image", side_effect=upload) as call:
            result = self.device.ocr(stabilization_ms=20, engine="indraocr", mark=True, timeout=.2)
        self.assertEqual(call.call_count, 1)
        self.assertEqual(result.frame_id, 1)
        self.assertEqual(self.session.publish_current.call_args.args[0].frame_id, 1)

    def test_stale_replayed_or_reset_frame_never_accepted(self):
        for changes in ({"replayed": True}, {"generation": 0}, {"connection_id": "old"}):
            frame = Frame(1, "s", "phone", 100, 100, 1, time.monotonic(), self.image, 1, "c")
            self.session.frame.side_effect = None
            self.session.frame.return_value = replace(frame, **changes)
            with self.subTest(changes=changes), patch.object(self.device, "_session", return_value=self.session), patch("mywebscrcpy.ocr.upload_image") as upload:
                with self.assertRaises(FrameTimeout):
                    self.device.ocr(stabilization_ms=20, timeout=.1)
                upload.assert_not_called()

    def test_repeated_frame_is_rejected(self):
        acquire = self.session.frame.side_effect
        first = acquire(Deadline(1))
        def repeated(deadline):
            return replace(first, received_at=time.monotonic())
        self.session.frame.side_effect = repeated
        with patch.object(self.device, "_session", return_value=self.session), patch("mywebscrcpy.ocr.upload_image", return_value=response()):
            with self.assertRaises(FrameTimeout):
                self.device.ocr(stabilization_ms=120, engine="indraocr", timeout=.3)

    def test_first_frame_must_be_fresh(self):
        old = Frame(1, "s", "phone", 100, 100, 1, time.monotonic()-1, self.image, 1, "c")
        self.session.frame.side_effect = None
        self.session.frame.return_value = old
        with patch.object(self.device, "_session", return_value=self.session), patch("mywebscrcpy.ocr.upload_image") as upload:
            with self.assertRaises(FrameTimeout):
                self.device.ocr(stabilization_ms=20, engine="indraocr", timeout=.1)
        upload.assert_not_called()

    def test_maximum_window_sampling_bound_with_virtual_clock(self):
        now = [100.]
        def wait(delay):
            now[0] += delay
            return False
        acquire = self.session.frame.side_effect
        def fresh(deadline):
            now[0] += .000001
            return acquire(deadline)
        self.session.frame.side_effect = fresh
        with patch.object(self.device, "_session", return_value=self.session), \
                patch("mywebscrcpy.ocr_sampling.time.monotonic", side_effect=lambda: now[0]), \
                patch.object(self.device._stop, "wait", side_effect=wait), \
                patch("mywebscrcpy.ocr_sampling._recognize", return_value=(response(), "indraocr", [])) as recognize:
            result = self.device.ocr(stabilization_ms=5000, engine="indraocr", timeout=6)
        self.assertGreaterEqual(recognize.call_count, 99)
        self.assertLessEqual(recognize.call_count, 101)
        self.assertLessEqual(now[0], 105.001)
        self.assertEqual(result.boxes[0].text, "确定")

    def test_parallel_samples_never_overlap_rounds_or_exceed_three_workers(self):
        active, peak = 0, 0
        lock = threading.Lock()
        def upload(*args, **kwargs):
            nonlocal active, peak
            with lock:
                active += 1
                peak = max(peak, active)
            time.sleep(.01)
            with lock:
                active -= 1
            return response()
        with patch.object(self.device, "_session", return_value=self.session), patch("mywebscrcpy.ocr.upload_image", side_effect=upload) as call:
            result = self.device.ocr(stabilization_ms=125, engine="parallel", timeout=.5)
        self.assertGreaterEqual(call.call_count, 6)
        self.assertEqual(call.call_count % 3, 0)
        self.assertEqual(peak, 3)
        self.assertEqual(len(result.boxes), 1)

    def test_bbox_keywords_and_fallback_are_preserved(self):
        def upload(image, url, auth="", timeout=10):
            self.assertEqual(image.shape[:2], (50, 50))
            if "8000" in url:
                raise RuntimeError("offline")
            return response()
        with patch.object(self.device, "_session", return_value=self.session), patch("mywebscrcpy.ocr.upload_image", side_effect=upload):
            result = self.device.ocr(stabilization_ms=20, engine="parallel", bbox=[.2, .3, .5, .5], keywords="确定", timeout=.5)
        self.assertAlmostEqual(result.boxes[0].x, .3)
        self.assertAlmostEqual(result.boxes[0].y, .4)
        self.assertTrue(result.fallback_reason)
        self.assertEqual(result.engines_used, ["indraocr", "paddleocr"])

    def test_keywords_are_applied_after_parallel_and_temporal_fusion(self):
        def upload(image, url, auth="", timeout=10):
            return response("目标" if "8032" in url else "其他")
        with patch.object(self.device, "_session", return_value=self.session), patch("mywebscrcpy.ocr.upload_image", side_effect=upload):
            result = self.device.ocr(stabilization_ms=20, engine="parallel", keywords="目标", timeout=.5)
        self.assertEqual(result.boxes, [])
        self.assertFalse(result.matched)
        self.assertEqual(result.text, "其他")

    def test_fractional_and_clipped_roi_use_actual_pixel_crop(self):
        for bbox, expected in (([.204, .306, .333, .222], [.2, .31, .34, .22]),
                               ([.994, .996, .3, .2], [.99, .99, .01, .01])):
            with self.subTest(bbox=bbox):
                def upload(image, url, auth="", timeout=10):
                    h, w = image.shape[:2]
                    return {"ocr_result": "全框", "ocr_boxes": [
                        dict(text="全框", confidence=.8, x=0, y=0, w=w, h=h)]}
                with patch.object(self.device, "_session", return_value=self.session), \
                        patch("mywebscrcpy.ocr.upload_image", side_effect=upload):
                    result = self.device.ocr(stabilization_ms=20, engine="indraocr", bbox=bbox, timeout=.5)
                box = result.boxes[0]
                for actual, target in zip((box.x, box.y, box.w, box.h), expected):
                    self.assertAlmostEqual(actual, target)

    def test_auto_fallback_shares_remaining_total_budget(self):
        timeouts = []
        def upload(image, url, auth="", timeout=10):
            timeouts.append(timeout)
            if "8032" in url:
                time.sleep(.025)
                raise RuntimeError("offline")
            return response()
        with patch.object(self.device, "_session", return_value=self.session), patch("mywebscrcpy.ocr.upload_image", side_effect=upload):
            result = self.device.ocr(stabilization_ms=20, timeout=.2)
        self.assertEqual(result.engine, "macocr")
        self.assertEqual(len(timeouts), 2)
        self.assertGreater(timeouts[0] - timeouts[1], .02)

    def test_total_timeout_bounds_slow_engine_and_no_late_mark(self):
        release = threading.Event()
        try:
            with patch.object(self.device, "_session", return_value=self.session), patch("mywebscrcpy.ocr.upload_image", side_effect=lambda *a, **k: release.wait(.8)):
                started = time.monotonic()
                with self.assertRaises((FrameTimeout, RuntimeError, TimeoutError)):
                    self.device.ocr(stabilization_ms=500, engine="indraocr", mark=True, timeout=.06)
                self.assertLess(time.monotonic() - started, .3)
        finally:
            release.set()
        self.session.publish_current.assert_not_called()

    def test_reset_during_recognition_cannot_publish(self):
        def upload(*args, **kwargs):
            self.session.generation = 2
            return response()
        with patch.object(self.device, "_session", return_value=self.session), patch("mywebscrcpy.ocr.upload_image", side_effect=upload):
            with self.assertRaises(FrameTimeout):
                self.device.ocr(stabilization_ms=20, engine="indraocr", mark=True, timeout=.1)
        self.session.publish_current.assert_not_called()
