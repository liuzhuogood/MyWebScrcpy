from unittest import TestCase
from unittest.mock import MagicMock, patch
import numpy as np

from mywebscrcpy.device import Device
from mywebscrcpy.errors import FrameTimeout, MatchTimeout
from mywebscrcpy.ocr import OCRBox, OCRResult, crop_image, perform_ocr


class TestOCR(TestCase):
    def test_crop_image(self):
        img = np.zeros((100, 200, 3), dtype=np.uint8)
        cropped, pixel_bbox = crop_image(img, [0.1, 0.2, 0.5, 0.4])
        self.assertEqual(pixel_bbox, {"x": 20, "y": 20, "width": 100, "height": 40})
        self.assertEqual(cropped.shape, (40, 100, 3))

    def test_ocr_box(self):
        box = OCRBox("测试文字", 0.95, 0.1, 0.2, 0.3, 0.4)
        self.assertEqual(box.center, (0.25, 0.4))
        obj = box.to_object()
        self.assertEqual(obj["label"], "测试文字")
        self.assertEqual(obj["confidence"], 0.95)
        self.assertEqual(obj["kind"], "text")

    def test_perform_ocr_auto_fallback(self):
        img = np.zeros((50, 50, 3), dtype=np.uint8)
        mock_paddle_res = {
            "ocr_result": "识别文本",
            "ocr_boxes": [{"text": "识别文本", "confidence": 0.99, "x": 5, "y": 5, "w": 40, "h": 20}],
        }

        with patch("mywebscrcpy.ocr.upload_image") as mock_upload:
            def side_effect(image, url, auth="", timeout=10.0):
                if "8032" in url:
                    raise RuntimeError("Indra 离线")
                if "8000" in url:
                    raise RuntimeError("macocr 离线")
                return mock_paddle_res

            mock_upload.side_effect = side_effect
            result, engine_used, errors = perform_ocr(img, engine="auto")
            self.assertEqual(engine_used, "paddleocr")
            self.assertEqual(len(errors), 2)
            self.assertIn("Indra OCR", errors[0])
            self.assertIn("macOCR", errors[1])
            self.assertEqual(result["ocr_result"], "识别文本")

    def test_ocr_result_find(self):
        boxes = [
            OCRBox("确定按钮", 0.9, 0.1, 0.1, 0.2, 0.1),
            OCRBox("取消按钮", 0.8, 0.4, 0.1, 0.2, 0.1),
        ]
        res = OCRResult("matched", True, "确定按钮\n取消按钮", boxes, "paddleocr")
        found = res.find("确定")
        self.assertEqual(len(found), 1)
        self.assertEqual(found[0].text == "确定按钮", True)

    def test_wait_for_text_retries_and_returns_match(self):
        device = Device(MagicMock(), "device-1")
        missing = OCRResult("not_found", False, "", [], "paddleocr")
        matched = OCRResult("matched", True, "继续", [OCRBox("继续", .9, 0, 0, .1, .1)], "paddleocr")
        with patch.object(device, "ocr", side_effect=[missing, matched]) as recognize:
            result = device.wait_for_text("继续", timeout=1, interval=.01)
        self.assertIs(result, matched)
        self.assertEqual(recognize.call_count, 2)
        self.assertLessEqual(recognize.call_args.kwargs["timeout"], 1)

    def test_wait_for_text_raises_on_timeout(self):
        device = Device(MagicMock(), "device-1")
        missing = OCRResult("not_found", False, "其他", [], "paddleocr")
        with patch.object(device, "ocr", return_value=missing):
            with self.assertRaises(MatchTimeout):
                device.wait_for_text("继续", timeout=.03, interval=.01)

    def test_wait_for_text_translates_frame_timeout(self):
        device = Device(MagicMock(), "device-1")
        with patch.object(device, "ocr", side_effect=FrameTimeout("no frame")):
            with self.assertRaises(MatchTimeout):
                device.wait_for_text("继续", timeout=1)

    def test_wait_for_text_validates_inputs(self):
        device = Device(MagicMock(), "device-1")
        with self.assertRaises(ValueError):
            device.wait_for_text(" ")
        with self.assertRaises(ValueError):
            device.wait_for_text("继续", interval=0)

    def test_perform_ocr_respects_paddle_timeout(self):
        with patch("mywebscrcpy.ocr.upload_image", return_value={"ocr_result": ""}) as upload:
            perform_ocr(np.zeros((2, 2, 3), dtype=np.uint8), engine="paddleocr", timeout=1.25)
        self.assertEqual(upload.call_args.kwargs["timeout"], 1.25)

    def test_indra_auto_first_and_empty_result_is_success(self):
        response = {"ocr_result": "", "ocr_boxes": []}
        with patch("mywebscrcpy.ocr.upload_image", return_value=response) as upload:
            result, engine, errors = perform_ocr(None, indraocr_url="http://indra/upload")
        self.assertEqual((result, engine, errors), (response, "indraocr", []))
        self.assertEqual(upload.call_count, 1)
        self.assertEqual(upload.call_args.args[1], "http://indra/upload")

    def test_explicit_indra_does_not_fallback(self):
        with patch("mywebscrcpy.ocr.upload_image", side_effect=RuntimeError("offline")) as upload:
            with self.assertRaisesRegex(RuntimeError, "Indra OCR"):
                perform_ocr(None, engine="indraocr")
        self.assertEqual(upload.call_count, 1)
