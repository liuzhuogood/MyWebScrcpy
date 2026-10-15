"""Local transport checks; fixtures do not represent project API routes."""

from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
import json
import os
from pathlib import Path
import shutil
import ssl
import subprocess
import sys
import tempfile
import threading
import unittest
from unittest.mock import patch
from urllib.error import URLError

from request_client import ApiClient


class FixtureHandler(BaseHTTPRequestHandler):
    def log_message(self, *_args):
        pass

    def do_GET(self):
        self.respond()

    def do_POST(self):
        self.respond()

    def respond(self):
        body = self.rfile.read(int(self.headers.get('Content-Length', '0')))
        if self.path == '/redirect':
            self.send_response(302)
            self.send_header('Location', '/redirect-target')
            payload = b'redirect'
        elif self.path == '/redirect-target':
            self.server.redirect_visits += 1
            self.send_response(200)
            payload = b'unexpected follow'
        elif self.path == '/error':
            self.send_response(418)
            payload = b'{"fixture_error":"test"}'
        elif self.path == '/binary':
            self.send_response(200)
            payload = b'\x00\xff\x80fixture'
        else:
            self.send_response(200)
            payload = json.dumps({'path': self.path, 'method': self.command,
                                  'headers': dict(self.headers),
                                  'body': body.decode('utf-8')}).encode()
        self.send_header('Content-Length', str(len(payload)))
        self.end_headers()
        self.wfile.write(payload)


class ClientTests(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.server = ThreadingHTTPServer(('127.0.0.1', 0), FixtureHandler)
        cls.server.redirect_visits = 0
        cls.thread = threading.Thread(target=cls.server.serve_forever, daemon=True)
        cls.thread.start()
        cls.url = f'http://127.0.0.1:{cls.server.server_port}'

    @classmethod
    def tearDownClass(cls):
        cls.server.shutdown()
        cls.server.server_close()
        cls.thread.join()

    def setUp(self):
        self.environment = patch.dict(os.environ, {
            'MYWEBSCRCPY_API_TOKEN': '', 'MYWEBSCRCPY_API_TOKEN_HEADER': '',
            'MYWEBSCRCPY_API_TOKEN_PREFIX': '', 'MYWEBSCRCPY_API_URL': self.url,
        })
        self.environment.start()
        self.addCleanup(self.environment.stop)

    def cli(self, *args, stdin=None):
        return subprocess.run([sys.executable, str(Path(__file__).with_name('request_client.py')),
                               *args], input=stdin, capture_output=True, check=False)

    def test_query_and_unconfigured_auth(self):
        response = ApiClient(self.url).request('GET', '/echo', query=[
            ('serial', '10.0.0.30:5555'), ('q', '中文 & value'), ('q', 'second')])
        value = json.loads(response.body)
        self.assertEqual(response.status, 200)
        self.assertIn('serial=10.0.0.30%3A5555', value['path'])
        self.assertIn('q=%E4%B8%AD%E6%96%87+%26+value&q=second', value['path'])
        self.assertNotIn('Authorization', value['headers'])

    def test_token_configuration_and_override(self):
        os.environ.update(MYWEBSCRCPY_API_TOKEN='fixture-secret',
                          MYWEBSCRCPY_API_TOKEN_HEADER='X-Fixture-Token')
        client = ApiClient(self.url)
        self.assertEqual(json.loads(client.request('GET', '/echo').body)['headers'][
            'X-Fixture-Token'], 'fixture-secret')
        os.environ['MYWEBSCRCPY_API_TOKEN_PREFIX'] = 'Fixture'
        client = ApiClient(self.url)
        self.assertEqual(json.loads(client.request('GET', '/echo').body)['headers'][
            'X-Fixture-Token'], 'Fixture fixture-secret')
        with self.assertRaises(ValueError):
            client.request('GET', '/echo', headers={'x-fixture-token': 'override'})

    def test_missing_auth_header_and_invalid_token_do_not_leak(self):
        os.environ['MYWEBSCRCPY_API_TOKEN'] = 'fixture-secret'
        result = self.cli('/echo')
        self.assertNotEqual(result.returncode, 0)
        self.assertNotIn(b'fixture-secret', result.stdout + result.stderr)
        os.environ['MYWEBSCRCPY_API_TOKEN_HEADER'] = 'X-Fixture-Token'
        os.environ['MYWEBSCRCPY_API_TOKEN'] = 'fixture-secret\ninjected'
        result = self.cli('/echo')
        self.assertNotEqual(result.returncode, 0)
        self.assertNotIn(b'fixture-secret', result.stdout + result.stderr)

    def test_redirect_is_not_followed(self):
        previous = self.server.redirect_visits
        result = self.cli('/redirect')
        self.assertEqual(result.returncode, 1)
        self.assertEqual(result.stdout, b'redirect')
        self.assertEqual(self.server.redirect_visits, previous)

    def test_json_stdin_and_binary_output(self):
        result = self.cli('/echo', '--method', 'POST', '--json-file', '-', stdin=b'{"test":1}')
        self.assertEqual(result.returncode, 0, result.stderr)
        value = json.loads(result.stdout)
        self.assertEqual(value['method'], 'POST')
        self.assertEqual(value['headers']['Content-Type'], 'application/json')
        self.assertEqual(value['body'], '{"test":1}')
        with tempfile.TemporaryDirectory() as folder:
            output = Path(folder) / 'response.bin'
            result = self.cli('/binary', '--output', str(output))
            self.assertEqual(result.returncode, 0, result.stderr)
            self.assertEqual(result.stdout, b'')
            self.assertEqual(output.read_bytes(), b'\x00\xff\x80fixture')

    def test_http_error_keeps_body_and_fails(self):
        result = self.cli('/error')
        self.assertEqual(result.returncode, 1)
        self.assertEqual(result.stdout, b'{"fixture_error":"test"}')
        self.assertIn(b'HTTP 418', result.stderr)

    def test_invalid_targets_and_timeout(self):
        for path in ('http://other.example/echo', '//other.example/echo', '/echo?q=1'):
            with self.assertRaises(ValueError):
                ApiClient(self.url).request('GET', path)
        for timeout in (0, -1, float('inf'), float('nan')):
            with self.assertRaises(ValueError):
                ApiClient(self.url, timeout=timeout)

    @unittest.skipUnless(shutil.which('openssl'), 'openssl needed for self-signed TLS fixture')
    def test_self_signed_tls_requires_explicit_insecure(self):
        with tempfile.TemporaryDirectory() as folder:
            cert, key = Path(folder) / 'cert.pem', Path(folder) / 'key.pem'
            subprocess.run(['openssl', 'req', '-x509', '-newkey', 'rsa:2048', '-nodes',
                            '-keyout', str(key), '-out', str(cert), '-days', '1',
                            '-subj', '/CN=localhost'], capture_output=True, check=True)
            server = ThreadingHTTPServer(('127.0.0.1', 0), FixtureHandler)
            context = ssl.SSLContext(ssl.PROTOCOL_TLS_SERVER)
            context.load_cert_chain(cert, key)
            server.socket = context.wrap_socket(server.socket, server_side=True)
            thread = threading.Thread(target=server.serve_forever, daemon=True)
            thread.start()
            url = f'https://127.0.0.1:{server.server_port}'
            try:
                with self.assertRaises(URLError):
                    ApiClient(url).request('GET', '/echo')
                result = self.cli('--base-url', url, '/echo', '--insecure')
                self.assertEqual(result.returncode, 0, result.stderr)
            finally:
                server.shutdown()
                server.server_close()
                thread.join()


if __name__ == '__main__':
    unittest.main()
