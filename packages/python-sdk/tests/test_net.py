import os
import unittest
from unittest.mock import MagicMock, patch

import numpy as np

from mywebscrcpy import Client
from mywebscrcpy import ConnectionError as SDKConnectionError
from mywebscrcpy import ocr as ocr_module
from mywebscrcpy import vision as vision_module
from mywebscrcpy.contracts import Deadline
from mywebscrcpy.net import ENV_MODE, ENV_VAR, httpx_options, resolve_proxy, urllib_opener, websocket_options

PROXY = "http://127.0.0.1:7890"


class ResolveProxyTests(unittest.TestCase):
    def test_default_is_no_proxy(self):
        with patch.dict(os.environ, {}, clear=True):
            self.assertIsNone(resolve_proxy())

    def test_environment_variable_is_used_when_argument_missing(self):
        with patch.dict(os.environ, {ENV_VAR: PROXY}):
            self.assertEqual(resolve_proxy(), PROXY)

    def test_argument_wins_over_environment_variable(self):
        with patch.dict(os.environ, {ENV_VAR: PROXY}):
            self.assertEqual(resolve_proxy("http://127.0.0.1:1080"), "http://127.0.0.1:1080")

    def test_env_mode_and_blank_are_distinct(self):
        self.assertEqual(resolve_proxy("env"), ENV_MODE)
        self.assertEqual(resolve_proxy("ENV"), ENV_MODE)
        with patch.dict(os.environ, {}, clear=True):
            self.assertIsNone(resolve_proxy("   "))

    def test_invalid_proxy_is_rejected(self):
        for value in ["127.0.0.1:7890", "socks5://127.0.0.1:1080", "http://127.0.0.1", "http://:7890"]:
            with self.assertRaises(ValueError):
                resolve_proxy(value)


class TransportOptionTests(unittest.TestCase):
    def test_httpx_options(self):
        self.assertEqual(httpx_options(None), {"trust_env": False})
        self.assertEqual(httpx_options(ENV_MODE), {"trust_env": True})
        self.assertEqual(httpx_options(PROXY), {"proxy": PROXY, "trust_env": False})

    def test_websocket_options(self):
        self.assertEqual(websocket_options(None), {"http_no_proxy": ["*"]})
        self.assertEqual(websocket_options(ENV_MODE), {})
        self.assertEqual(websocket_options(PROXY), {"http_proxy_host": "127.0.0.1", "http_proxy_port": 7890})
        self.assertEqual(websocket_options("http://user:pw@127.0.0.1:7890"),
                         {"http_proxy_host": "127.0.0.1", "http_proxy_port": 7890,
                          "http_proxy_auth": ("user", "pw")})

    def probe(self, opener):
        return [handler.proxies for handler in opener.handlers if hasattr(handler, "proxies")]

    def test_urllib_opener_direct_installs_no_proxy_mapping(self):
        with patch("urllib.request.getproxies", side_effect=AssertionError("不应读取环境代理")):
            self.assertEqual(self.probe(urllib_opener(None)), [])

    def test_urllib_opener_uses_configured_proxy(self):
        self.assertIn({"http": PROXY, "https": PROXY}, self.probe(urllib_opener(PROXY)))


class ClientProxyBindingTests(unittest.TestCase):
    def pools(self, client):
        return [type(transport._pool).__name__ for transport in client._http._mounts.values()]

    def test_http_requests_use_configured_proxy(self):
        client = Client("http://10.0.0.6:8080", proxy=PROXY)
        self.addCleanup(client.close)
        self.assertEqual(client._proxy, PROXY)
        self.assertIn("HTTPProxy", self.pools(client))

    def test_http_requests_stay_direct_by_default(self):
        with patch.dict(os.environ, {}, clear=True):
            client = Client("http://10.0.0.6:8080")
        self.addCleanup(client.close)
        self.assertIsNone(client._proxy)
        self.assertNotIn("HTTPProxy", self.pools(client))

    def test_environment_variable_configures_client(self):
        with patch.dict(os.environ, {ENV_VAR: PROXY}):
            client = Client("http://10.0.0.6:8080")
        self.addCleanup(client.close)
        self.assertEqual(client._proxy, PROXY)

    def connect_options(self, client):
        with patch.object(vision_module.websocket, "create_connection",
                          side_effect=vision_module.websocket.WebSocketException("stop")) as connect:
            with self.assertRaises(SDKConnectionError):
                vision_module.Vision(client, "phone", Deadline(5))
        return connect.call_args.kwargs

    def test_vision_connection_uses_configured_proxy(self):
        client = Client("http://10.0.0.6:8080", proxy=PROXY)
        self.addCleanup(client.close)
        options = self.connect_options(client)
        self.assertEqual(options["http_proxy_host"], "127.0.0.1")
        self.assertEqual(options["http_proxy_port"], 7890)
        self.assertNotIn("http_no_proxy", options)

    def test_vision_connection_stays_direct_by_default(self):
        with patch.dict(os.environ, {}, clear=True):
            client = Client("http://10.0.0.6:8080")
        self.addCleanup(client.close)
        self.assertEqual(self.connect_options(client)["http_no_proxy"], ["*"])

    def test_ocr_upload_uses_configured_proxy(self):
        response = MagicMock()
        response.read.return_value = b'{"ok": true}'
        opener = MagicMock()
        opener.open.return_value.__enter__.return_value = response
        with patch.object(ocr_module, "urllib_opener", return_value=opener) as make_opener:
            result = ocr_module.upload_image(np.zeros((4, 4, 3), dtype=np.uint8),
                                             "http://127.0.0.1:8032/upload", proxy=PROXY)
        self.assertEqual(result, {"ok": True})
        make_opener.assert_called_once_with(PROXY)

    def test_ocr_upload_stays_direct_by_default(self):
        response = MagicMock()
        response.read.return_value = b'{"ok": true}'
        opener = MagicMock()
        opener.open.return_value.__enter__.return_value = response
        with patch.dict(os.environ, {}, clear=True):
            with patch.object(ocr_module, "urllib_opener", return_value=opener) as make_opener:
                ocr_module.upload_image(np.zeros((4, 4, 3), dtype=np.uint8), "http://127.0.0.1:8032/upload")
        make_opener.assert_called_once_with(None)


if __name__ == "__main__":
    unittest.main()
