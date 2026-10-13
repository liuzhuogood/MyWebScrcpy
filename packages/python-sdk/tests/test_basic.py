import json
import os
import subprocess
import sys
import tempfile
import threading
import unittest
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from pathlib import Path

import httpx
from mywebscrcpy import Client, CommandError, HTTPError, ProtocolError, SDKError


class BasicContractTests(unittest.TestCase):
    def fixture(self, response):
        seen = []
        client = Client("http://test.invalid")
        client._http.close()
        def handle(request):
            seen.append(request)
            return httpx.Response(200, json=response)
        client._http = httpx.Client(base_url="http://test.invalid", transport=httpx.MockTransport(handle))
        self.addCleanup(client.close)
        return client, seen

    def test_binding_units_optional_output_and_close(self):
        client, seen = self.fixture({"status": "completed", "exit_code": 0})
        with client.device("phone-A") as device:
            result = device.adb(["shell", "getprop", "ro.product.model"], timeout=1.25)
            self.assertEqual(result.stdout, "")
        request = seen[0]
        self.assertEqual(request.url.params["serial"], "phone-A")
        self.assertEqual(json.loads(request.content), {"args": ["shell", "getprop", "ro.product.model"], "timeout_ms": 1250})
        with self.assertRaises(SDKError):
            device.adb(["shell"])
        client.close()
        client.close()
        with self.assertRaises(SDKError):
            client.device("phone-A")

    def test_command_failure_truncation_and_missing_fields(self):
        for response in [{"status": "failed", "exit_code": 1}, {"status": "timed_out"}, {"status": "completed", "exit_code": 0, "truncated": True}]:
            client, seen = self.fixture(response)
            with self.assertRaises(CommandError):
                client.device("phone").adb(["shell"])
            self.assertEqual(len(seen), 1, "actions must never retry")
        for response in [{}, {"state": "completed", "exit_code": 0}, {"status": "completed"}, {"status": "completed", "exit_code": "0"}, {"status": "completed", "exit_code": 0, "stdout": None}]:
            client, _ = self.fixture(response)
            with self.assertRaises(ProtocolError):
                client.device("phone").adb(["shell"])

    def test_target_override_and_timeouts_reject_before_request(self):
        client, seen = self.fixture({"status": "completed", "exit_code": 0})
        device = client.device("phone")
        for args in [["-s", "other"], ["adb", "devices"], ["shell", "-Hhost"], [], ["shell", ""]]:
            with self.assertRaises(ValueError):
                device.adb(args)
        for timeout in [0, -1, 601, float("nan"), True]:
            with self.assertRaises(ValueError):
                device.adb(["shell"], timeout=timeout)
        self.assertEqual(seen, [])

    def test_exact_page_info_fields(self):
        client, _ = self.fixture({"display": {"width": 1080, "height": 2400}})
        self.assertEqual(client.device("phone").page_info().height, 2400)
        for response in [{"screen": {"width": 1080, "height": 2400}}, {"display": {"width": 0, "height": 2400}}]:
            client, _ = self.fixture(response)
            with self.assertRaises(ProtocolError):
                client.device("phone").page_info()

    def test_http_error_retains_exact_error_fields(self):
        client = Client("http://test.invalid")
        client._http.close()
        client._http = httpx.Client(base_url="http://test.invalid", transport=httpx.MockTransport(lambda r: httpx.Response(400, json={"error": "invalid_request", "message": "bad args"})))
        with client:
            with self.assertRaises(HTTPError) as caught:
                client.device("phone").adb(["shell"])
        self.assertEqual((caught.exception.status, caught.exception.code, caught.exception.detail), (400, "invalid_request", "bad args"))

    def test_portable_example_runs_outside_application_without_vision(self):
        requests = []
        class Handler(BaseHTTPRequestHandler):
            def do_POST(self):
                requests.append((self.path, json.loads(self.rfile.read(int(self.headers["Content-Length"])))))
                value = json.dumps({"status": "completed", "exit_code": 0, "stdout": "Portable Phone\n"}).encode()
                self.send_response(200)
                self.send_header("Content-Type", "application/json")
                self.send_header("Content-Length", str(len(value)))
                self.end_headers()
                self.wfile.write(value)
            def log_message(self, *args):
                pass
        server = ThreadingHTTPServer(("127.0.0.1", 0), Handler)
        thread = threading.Thread(target=server.serve_forever)
        thread.start()
        try:
            sample = Path(__file__).resolve().parents[1] / "examples/device_query.py"
            with tempfile.TemporaryDirectory() as directory:
                copied = Path(directory) / "workflow.py"
                copied.write_text(sample.read_text(), encoding="utf-8")
                result = subprocess.run([sys.executable, str(copied), "portable-phone", "--base-url", f"http://127.0.0.1:{server.server_port}"], cwd=directory, capture_output=True, text=True, timeout=10)
            self.assertEqual(result.returncode, 0, result.stderr)
            self.assertIn("Portable Phone", result.stdout)
            self.assertEqual(requests[0][1]["timeout_ms"], 10000)
            self.assertIn("serial=portable-phone", requests[0][0])
            self.assertNotIn("cv2", sys.modules)
            self.assertNotIn("av", sys.modules)
        finally:
            server.shutdown()
            server.server_close()
            thread.join()


if __name__ == "__main__":
    unittest.main()
