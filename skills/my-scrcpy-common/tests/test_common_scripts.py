#!/usr/bin/env python3
"""通用操作技能脚本的有效行为与关键异常自测套件"""

import json
import math
import os
import sys
import unittest
from unittest.mock import MagicMock, patch

# 将 scripts 加入 sys.path
SCRIPT_DIR = os.path.abspath(os.path.join(os.path.dirname(__file__), "..", "scripts"))
if SCRIPT_DIR not in sys.path:
    sys.path.insert(0, SCRIPT_DIR)

from request_client import ApiResponse
import open_app
import restart_app
import watch_video
import try_return
import android_back
import observe


class TestCommonScripts(unittest.TestCase):
    def setUp(self):
        self.mock_client = MagicMock()

    # --- 1. 服务地址解析测试 ---
    def test_resolve_base_url_explicit(self):
        url = open_app.resolve_base_url("http://127.0.0.1:15272/")
        self.assertEqual(url, "http://127.0.0.1:15272")

    def test_resolve_base_url_from_env(self):
        with patch.dict(os.environ, {"MYWEBSCRCPY_BASE_URL": "http://127.0.0.1:9090"}):
            url = open_app.resolve_base_url(None)
            self.assertEqual(url, "http://127.0.0.1:9090")

    def test_resolve_base_url_missing_raises(self):
        with patch.dict(os.environ, {}, clear=True):
            with self.assertRaises(ValueError):
                open_app.resolve_base_url(None)

    # --- 2. open_app 行为与异常测试 ---
    def test_format_component(self):
        self.assertEqual(open_app.format_component("com.kuaishou.nebula", "com.kuaishou.nebula.MainActivity"),
                         "com.kuaishou.nebula/com.kuaishou.nebula.MainActivity")
        self.assertEqual(open_app.format_component("com.kuaishou.nebula", ".MainActivity"),
                         "com.kuaishou.nebula/.MainActivity")
        self.assertEqual(open_app.format_component("com.kuaishou.nebula", "com.kuaishou.nebula/com.kuaishou.nebula.MainActivity"),
                         "com.kuaishou.nebula/com.kuaishou.nebula.MainActivity")

    def test_launch_app_success(self):
        self.mock_client.request.return_value = ApiResponse(
            status=200, headers={}, body=json.dumps({"status": "completed", "exit_code": 0}).encode("utf-8")
        )
        ok, err = open_app.launch_app(self.mock_client, "serial-1", "com.test.app", "MainActivity")
        self.assertTrue(ok)
        self.assertEqual(err, "")

    def test_launch_app_adb_failed(self):
        # 退出码非0
        self.mock_client.request.return_value = ApiResponse(
            status=200, headers={}, body=json.dumps({"status": "completed", "exit_code": 1, "stderr": "Activity not found"}).encode("utf-8")
        )
        ok, err = open_app.launch_app(self.mock_client, "serial-1", "com.test.app")
        self.assertFalse(ok)
        self.assertIn("命令退出码异常: 1", err)

    def test_launch_app_status_failed(self):
        # 状态 failed
        self.mock_client.request.return_value = ApiResponse(
            status=200, headers={}, body=json.dumps({"status": "failed", "error": "device offline"}).encode("utf-8")
        )
        ok, err = open_app.launch_app(self.mock_client, "serial-1", "com.test.app")
        self.assertFalse(ok)
        self.assertIn("命令未完成: status=failed", err)

    # --- 3. restart_app 行为与异常测试 ---
    def test_force_stop_app_success(self):
        self.mock_client.request.return_value = ApiResponse(
            status=200, headers={}, body=json.dumps({"status": "completed", "exit_code": 0}).encode("utf-8")
        )
        ok, err = restart_app.force_stop_app(self.mock_client, "serial-1", "com.test.app")
        self.assertTrue(ok)
        self.assertEqual(err, "")

    def test_force_stop_app_failed_exit_code(self):
        self.mock_client.request.return_value = ApiResponse(
            status=200, headers={}, body=json.dumps({"status": "completed", "exit_code": 255, "stderr": "error"}).encode("utf-8")
        )
        ok, err = restart_app.force_stop_app(self.mock_client, "serial-1", "com.test.app")
        self.assertFalse(ok)
        self.assertIn("停止命令退出码异常: 255", err)

    # --- 4. watch_video 行为与异常测试 ---
    def test_watch_video_param_nan_inf(self):
        # NaN / Inf 校验
        with patch.object(sys, "argv", ["watch_video.py", "--serial", "phone", "--base", "http://localhost", "--min-stay", "nan"]):
            self.assertEqual(watch_video.main(), 1)

    def test_watch_video_untrusted_page_aborts(self):
        # 页面信息丢失不可信时中止
        with patch.object(watch_video, "get_current_page", return_value={}):
            with patch.object(sys, "argv", ["watch_video.py", "--serial", "phone", "--base", "http://localhost"]):
                self.assertEqual(watch_video.main(), 1)

    def test_swipe_up_contract(self):
        # 契约必须为 status="executed" 且 executed=True
        self.mock_client.request.return_value = ApiResponse(
            status=200, headers={}, body=json.dumps({"status": "executed", "executed": True}).encode("utf-8")
        )
        ok, err = watch_video.swipe_up(self.mock_client, "serial-1")
        self.assertTrue(ok)
        self.assertEqual(err, "")

        # 若后端返回 failed
        self.mock_client.request.return_value = ApiResponse(
            status=200, headers={}, body=json.dumps({"status": "failed", "executed": False, "error_code": "device_busy"}).encode("utf-8")
        )
        ok, err = watch_video.swipe_up(self.mock_client, "serial-1")
        self.assertFalse(ok)
        self.assertIn("滑动未执行成功", err)

    def test_watch_video_budget_enforcement(self):
        # 预算不足 min-stay 时立即退出，不超时溢出
        with patch.object(watch_video, "get_current_page", return_value={"package_name": "com.test.video"}):
            with patch.object(watch_video, "swipe_up", return_value=(True, "")):
                # 只有 1 秒总时限，min-stay 为 5 秒
                with patch.object(sys, "argv", ["watch_video.py", "--serial", "phone", "--base", "http://localhost", "--total-duration", "1.0", "--min-stay", "5.0", "--max-stay", "5.0"]):
                    ret = watch_video.main()
                    self.assertEqual(ret, 0)

    # --- 5. try_return 行为与异常测试 ---
    def test_try_return_pre_state_failed_aborts_without_key(self):
        # 前置状态失败禁止发键
        with patch.object(try_return, "get_state", return_value={}):
            with patch.object(try_return, "send_escape") as mock_escape:
                with patch.object(sys, "argv", ["try_return.py", "--serial", "phone", "--base", "http://localhost"]):
                    ret = try_return.main()
                    self.assertEqual(ret, 1)
                    mock_escape.assert_not_called()

    def test_send_escape_contract(self):
        self.mock_client.request.return_value = ApiResponse(
            status=200, headers={}, body=json.dumps({"status": "executed", "executed": True}).encode("utf-8")
        )
        ok, err = try_return.send_escape(self.mock_client, "serial-1")
        self.assertTrue(ok)

        self.mock_client.request.return_value = ApiResponse(
            status=200, headers={}, body=json.dumps({"status": "failed", "executed": False}).encode("utf-8")
        )
        ok, err = try_return.send_escape(self.mock_client, "serial-1")
        self.assertFalse(ok)

    def test_analyze_diff_detection(self):
        # 1. 包名变化 -> changed
        pre = {"observation_id": "1", "page_info": {"package_name": "pkg.a", "activity": "ActA"}}
        post = {"observation_id": "2", "page_info": {"package_name": "pkg.b", "activity": "ActA"}}
        st, _ = try_return.analyze_diff(pre, post)
        self.assertEqual(st, "changed")

        # 2. Activity 变化 -> changed
        post_act = {"observation_id": "2", "page_info": {"package_name": "pkg.a", "activity": "ActB"}}
        st, _ = try_return.analyze_diff(pre, post_act)
        self.assertEqual(st, "changed")

        # 3. 页面 XML 结构变化 (如弹层关闭) -> changed
        pre_xml = {"observation_id": "1", "page_info": {"package_name": "pkg.a", "activity": "ActA"}, "ui_xml": "<dialog>Ad</dialog>"}
        post_xml = {"observation_id": "2", "page_info": {"package_name": "pkg.a", "activity": "ActA"}, "ui_xml": "<main>Content</main>"}
        st, _ = try_return.analyze_diff(pre_xml, post_xml)
        self.assertEqual(st, "changed")

        # 4. 未变化 -> unchanged
        same = {"observation_id": "2", "page_info": {"package_name": "pkg.a", "activity": "ActA"}}
        st, _ = try_return.analyze_diff(pre, same)
        self.assertEqual(st, "unchanged")

        # 5. 丢失 -> unknown
        st, _ = try_return.analyze_diff(pre, {})
        self.assertEqual(st, "unknown")


if __name__ == "__main__":
    unittest.main()
