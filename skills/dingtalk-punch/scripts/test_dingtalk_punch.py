#!/usr/bin/env python3
"""针对 dingtalk_punch.py 的单元测试。"""

from pathlib import Path
import sys
import unittest
from unittest.mock import MagicMock, patch

sys.path.insert(0, str(Path(__file__).resolve().parent))

from dingtalk_punch import (
    check_city_in_address,
    find_box_by_text,
    parse_punch_result,
    DingTalkPunchRunner,
)


class TestDingTalkPunch(unittest.TestCase):
    def test_find_box_by_text(self):
        sample_ocr = {
            "boxes": [
                {"text": "无关文字", "x": 0.01, "y": 0.02, "w": 0.1, "h": 0.05},
                {"text": "我的笔记", "x": 0.8, "y": 0.4, "w": 0.1, "h": 0.05},
            ]
        }
        coord = find_box_by_text(sample_ocr, "我的笔记")
        self.assertIsNotNone(coord)
        self.assertAlmostEqual(coord[0], 0.85)
        self.assertAlmostEqual(coord[1], 0.425)

        not_found = find_box_by_text(sample_ocr, "不存在的按钮")
        self.assertIsNone(not_found)

    def test_check_city_in_address(self):
        valid_data = {"code": 0, "data": {"address": "湖南省长沙市岳麓区麓谷街道"}}
        self.assertTrue(check_city_in_address(valid_data, "长沙"))
        self.assertFalse(check_city_in_address(valid_data, "武汉"))

        invalid_data = {"code": 0, "data": None}
        self.assertFalse(check_city_in_address(invalid_data, "长沙"))

    def test_parse_punch_result_success(self):
        sample_res = {
            "text": "考勤打卡 09:05 打卡成功 极速打卡成功",
            "boxes": [{"text": "打卡成功"}],
        }
        res = parse_punch_result(sample_res)
        self.assertEqual(res["status"], "succeeded")
        self.assertIn("打卡成功", res["matched_success"])
        self.assertEqual(res["punch_time"], "09:05")

    def test_parse_punch_result_failure(self):
        sample_res = {
            "text": "不在考勤范围内 打卡失败",
            "boxes": [],
        }
        res = parse_punch_result(sample_res)
        self.assertEqual(res["status"], "failed")
        self.assertIn("打卡失败", res["matched_failure"])

    def test_parse_punch_result_unknown(self):
        sample_res = {
            "text": "工作台 待办 日历",
            "boxes": [],
        }
        res = parse_punch_result(sample_res)
        self.assertEqual(res["status"], "unknown")

    @patch("dingtalk_punch.subprocess.run")
    def test_step2_3_city_mismatch_raises(self, mock_run):
        mock_run.side_effect = [
            MagicMock(returncode=0, stdout='{"ok": true}'),
            MagicMock(returncode=0, stdout='{"ok": true, "address": {"data": {"address": "湖北省武汉市江汉区"}}}'),
        ]
        runner = DingTalkPunchRunner(
            backend_url="http://127.0.0.1:8080",
            serial="10.0.0.104:5555",
            location_url="http://10.0.0.104:8888",
            lat=28.154,
            lng=112.937,
            expected_city="长沙",
        )
        with self.assertRaises(RuntimeError) as ctx:
            runner.step2_and_3_set_and_verify_location()
        self.assertIn("逆地理编码未包含目标城市", str(ctx.exception))


if __name__ == "__main__":
    unittest.main()
