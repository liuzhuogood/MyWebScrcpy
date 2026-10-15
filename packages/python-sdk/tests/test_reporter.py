from __future__ import annotations

import json
import os
import sys
import unittest
from http.server import BaseHTTPRequestHandler, HTTPServer
from pathlib import Path
import threading
import time

_SRC = Path(__file__).resolve().parents[1] / "src"
if str(_SRC) not in sys.path:
    sys.path.insert(0, str(_SRC))

from mywebscrcpy import TaskReporter
from mywebscrcpy.errors import SDKError


class MockReporterServer(BaseHTTPRequestHandler):
    runs = {}
    logs = {}
    finishes = {}
    heartbeats = 0

    def log_message(self, format, *args):
        pass  # 禁用标准控制台打印

    def do_POST(self):
        length = int(self.headers.get("Content-Length", 0))
        body = json.loads(self.rfile.read(length)) if length > 0 else {}

        if self.path == "/api/task-runs":
            run_id = f"run_{len(self.runs) + 1}"
            self.runs[run_id] = body
            self.send_response(201)
            self.send_header("Content-Type", "application/json")
            self.end_headers()
            self.wfile.write(json.dumps({"id": run_id, "status": "running", "acknowledged_seq": 0}).encode("utf-8"))
            return

        if "/heartbeat" in self.path:
            MockReporterServer.heartbeats += 1
            self.send_response(200)
            self.send_header("Content-Type", "application/json")
            self.end_headers()
            self.wfile.write(b'{"status": "running"}')
            return

        if "/logs" in self.path:
            parts = self.path.split("/")
            run_id = parts[3]
            entries = body.get("entries", [])
            max_seq = 0
            if run_id not in self.logs:
                self.logs[run_id] = []
            for e in entries:
                self.logs[run_id].append(e)
                max_seq = max(max_seq, e["seq"])
            self.send_response(200)
            self.send_header("Content-Type", "application/json")
            self.end_headers()
            self.wfile.write(json.dumps({"id": run_id, "acknowledged_seq": max_seq, "truncated": False}).encode("utf-8"))
            return

        if "/finish" in self.path:
            parts = self.path.split("/")
            run_id = parts[3]
            self.finishes[run_id] = body
            self.send_response(200)
            self.send_header("Content-Type", "application/json")
            self.end_headers()
            self.wfile.write(json.dumps({"id": run_id, "status": body.get("status"), "ended_at": 1000}).encode("utf-8"))
            return

        self.send_response(404)
        self.end_headers()


class TestTaskReporter(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        MockReporterServer.runs = {}
        MockReporterServer.logs = {}
        MockReporterServer.finishes = {}
        MockReporterServer.heartbeats = 0
        cls.server = HTTPServer(("127.0.0.1", 0), MockReporterServer)
        cls.port = cls.server.server_address[1]
        cls.base_url = f"http://127.0.0.1:{cls.port}"
        cls.thread = threading.Thread(target=cls.server.serve_forever, daemon=True)
        cls.thread.start()

    @classmethod
    def tearDownClass(cls):
        cls.server.shutdown()
        cls.server.server_close()

    def test_reporter_lifecycle(self):
        reporter = TaskReporter(
            self.base_url,
            name="Python测试任务",
            device_serial="device-1",
            heartbeat_interval=0.2,
        )
        run_id = reporter.start()
        self.assertIsNotNone(run_id)
        self.assertEqual(reporter.status, "running")

        # 追加日志
        reporter.log("第一行日志")
        reporter.log("第二行日志", stream="stderr")
        reporter.flush()
        self.assertEqual(reporter.acknowledged_seq, 2)
        self.assertEqual(len(MockReporterServer.logs[run_id]), 2)

        # 心跳
        time.sleep(0.3)
        self.assertGreaterEqual(MockReporterServer.heartbeats, 1)

        # 结束
        reporter.finish("succeeded", exit_code=0, summary="顺利执行")
        self.assertEqual(reporter.status, "succeeded")
        self.assertIn(run_id, MockReporterServer.finishes)
        self.assertEqual(MockReporterServer.finishes[run_id]["status"], "succeeded")
        reporter.close()

    def test_capture_output(self):
        reporter = TaskReporter(
            self.base_url,
            name="捕获测试",
            device_serial="device-2",
        )
        reporter.start()
        with reporter.capture_output():
            print("捕获标准输出 1")
            print("捕获标准输出 2", file=sys.stderr)

        reporter.flush()
        run_id = reporter.run_id
        logs = MockReporterServer.logs[run_id]
        messages = [l["message"] for l in logs]
        self.assertIn("捕获标准输出 1", messages)
        self.assertIn("捕获标准输出 2", messages)
        reporter.finish("succeeded")
        reporter.close()

    def test_context_manager_exception(self):
        reporter = TaskReporter(
            self.base_url,
            name="异常捕获测试",
            device_serial="device-3",
        )
        with self.assertRaises(ValueError):
            with reporter:
                reporter.log("执行中...")
                raise ValueError("故意抛出的错误")

        run_id = reporter.run_id
        finish_info = MockReporterServer.finishes[run_id]
        self.assertEqual(finish_info["status"], "failed")
        self.assertEqual(finish_info["exit_code"], 1)
        self.assertIn("故意抛出的错误", finish_info["summary"])

class TestReporterDeliveryBoundaries(unittest.TestCase):
    def reporter(self, handler, **options):
        import httpx
        reporter = TaskReporter('http://review.local', name='review', device_serial='fixture', **options)
        reporter._http.close()
        reporter._http = httpx.Client(base_url='http://review.local', transport=httpx.MockTransport(handler))
        self.addCleanup(reporter.close)
        return reporter

    def test_large_backlog_is_batched_and_acknowledged(self):
        import httpx
        batches = []
        def handler(request):
            body = json.loads(request.content)
            if request.url.path == '/api/task-runs':
                return httpx.Response(201, json={'id': 'batch-run'})
            batches.append((len(request.content), body['entries']))
            return httpx.Response(200, json={'acknowledged_seq': body['entries'][-1]['seq']})
        reporter = self.reporter(handler)
        reporter.start()
        for _ in range(100):
            reporter.log('中文测试' * 8000)
        reporter.flush()
        self.assertGreater(len(batches), 1)
        self.assertTrue(all(size < 4 * 1024 * 1024 for size, _ in batches))
        self.assertEqual([entry['seq'] for _, batch in batches for entry in batch], list(range(1, 101)))
        self.assertEqual(reporter.acknowledged_seq, 100)

    def test_missing_ack_cannot_finish_successfully(self):
        import httpx
        import tempfile
        finishes = []
        def handler(request):
            if request.url.path == '/api/task-runs':
                return httpx.Response(201, json={'id': 'unconfirmed'})
            if request.url.path.endswith('/finish'):
                finishes.append(request)
            return httpx.Response(200, json={})
        reporter = self.reporter(handler)
        reporter.start()
        reporter.log('must be acknowledged')
        with tempfile.TemporaryDirectory() as directory:
            previous = os.getcwd()
            try:
                os.chdir(directory)
                with self.assertRaises(SDKError):
                    reporter.finish('succeeded', max_retries=1)
                evidence = list(Path(directory).glob('.task-run-error-*.json'))
                self.assertEqual(len(evidence), 1)
                self.assertEqual(evidence[0].stat().st_mode & 0o777, 0o600)
                self.assertEqual(len(json.loads(evidence[0].read_text())['unconfirmed_logs']), 1)
            finally:
                os.chdir(previous)
        self.assertFalse(finishes)
        self.assertEqual(reporter.status, 'running')

    def test_buffer_overflow_does_not_create_sequence_gap(self):
        import httpx
        seen = []
        def handler(request):
            if request.url.path == '/api/task-runs':
                return httpx.Response(201, json={'id': 'bounded'})
            entries = json.loads(request.content)['entries']
            seen.extend(entry['seq'] for entry in entries)
            return httpx.Response(200, json={'acknowledged_seq': entries[-1]['seq']})
        reporter = self.reporter(handler, max_buffer_size=1)
        reporter.start()
        reporter.log('first')
        with self.assertRaises(SDKError):
            reporter.log('overflow')
        reporter.flush()
        reporter.log('retry after flush')
        reporter.flush()
        self.assertEqual(seen, [1, 2])
        with self.assertRaises(ValueError):
            reporter.log('bad stream', stream='guessed-stream')


if __name__ == "__main__":
    unittest.main()
