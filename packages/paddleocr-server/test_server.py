"""PaddleOCR sidecar 接口契约测试 (使用 Mock 测试替身，验证输出契约与数据结构读取，非真实模型运行)。"""
from __future__ import annotations

import unittest
from unittest.mock import Mock
import numpy as np

import server


class TestPaddleOCRSidecarContract(unittest.TestCase):
    def setUp(self):
        self.mock_ocr = Mock()
        server._ocr = self.mock_ocr
        server._result_cache.clear()

    def test_run_ocr_with_list_polys(self):
        self.mock_ocr.predict.return_value = [{
            "rec_texts": ["开始游戏", "设置"],
            "rec_scores": [0.98, 0.92],
            "rec_polys": [
                [[10, 10], [50, 10], [50, 25], [10, 25]],
                [[60, 10], [90, 10], [90, 25], [60, 25]],
            ],
        }]
        img = np.zeros((50, 100, 3), dtype=np.uint8)
        res = server.run_ocr(img)

        self.assertEqual(res["image_width"], 100)
        self.assertEqual(res["image_height"], 50)
        self.assertEqual(res["ocr_result"], "开始游戏\n设置")
        self.assertEqual(len(res["ocr_boxes"]), 2)
        box0 = res["ocr_boxes"][0]
        self.assertEqual(box0["text"], "开始游戏")
        self.assertAlmostEqual(box0["x"], 10.0)
        self.assertAlmostEqual(box0["y"], 10.0)
        self.assertAlmostEqual(box0["w"], 40.0)
        self.assertAlmostEqual(box0["h"], 15.0)

    def test_run_ocr_with_ndarray_polys(self):
        """验证 ndarray 格式坐标与分数不会触发布尔歧义异常。"""
        polys = np.array([
            [[10, 10], [50, 10], [50, 25], [10, 25]],
            [[60, 10], [90, 10], [90, 25], [60, 25]],
        ], dtype=float)
        scores = np.array([0.98, 0.92], dtype=float)

        self.mock_ocr.predict.return_value = [{
            "rec_texts": ["开始游戏", "设置"],
            "rec_scores": scores,
            "rec_polys": polys,
        }]
        img = np.zeros((50, 100, 3), dtype=np.uint8)
        res = server.run_ocr(img)

        self.assertEqual(len(res["ocr_boxes"]), 2)
        self.assertEqual(res["ocr_boxes"][0]["text"], "开始游戏")
        self.assertAlmostEqual(res["ocr_boxes"][0]["confidence"], 0.98)

    def test_run_ocr_empty_result(self):
        self.mock_ocr.predict.return_value = [{
            "rec_texts": [],
            "rec_scores": [],
            "rec_polys": [],
        }]
        img = np.zeros((50, 100, 3), dtype=np.uint8)
        res = server.run_ocr(img)

        self.assertEqual(res["ocr_result"], "")
        self.assertEqual(res["ocr_boxes"], [])


    def test_health_not_ready(self):
        server._model_ready = False
        server._model_error = None
        from fastapi import HTTPException
        with self.assertRaises(HTTPException) as cm:
            server.health()
        self.assertEqual(cm.exception.status_code, 503)
        self.assertIn("initializing", cm.exception.detail)

    def test_health_ready(self):
        server._model_ready = True
        server._model_error = None
        res = server.health()
        self.assertEqual(res, {"status": "ok", "ready": True})

    def test_health_error(self):
        server._model_ready = False
        server._model_error = "CUDA out of memory"
        from fastapi import HTTPException
        with self.assertRaises(HTTPException) as cm:
            server.health()
        self.assertEqual(cm.exception.status_code, 503)
        self.assertIn("CUDA out of memory", cm.exception.detail)


if __name__ == "__main__":
    unittest.main()
