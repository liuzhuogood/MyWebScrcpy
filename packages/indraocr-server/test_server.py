import importlib.util
from pathlib import Path
import threading
import time
from types import SimpleNamespace
import unittest

import cv2
from fastapi.testclient import TestClient
import numpy as np

spec = importlib.util.spec_from_file_location("indra_server", Path(__file__).with_name("server.py"))
server = importlib.util.module_from_spec(spec)
spec.loader.exec_module(server)


class SidecarTests(unittest.TestCase):
    def image_bytes(self):
        ok, data = cv2.imencode('.png', np.zeros((100, 200, 3), dtype=np.uint8))
        self.assertTrue(ok)
        return data.tobytes()

    def engine(self, gray, turn):
        self.assertFalse(turn)
        self.assertEqual(gray.shape, (100, 200))
        return SimpleNamespace(rotation=0, lines=[SimpleNamespace(text="测试", conf=.9, box=[10, 20, 80, 40])])

    def test_contract_and_bad_image(self):
        with TestClient(server.create_app(engine_factory=lambda: self.engine)) as client:
            self.assertEqual(client.get('/health').json()['engine'], 'indraocr')
            response = client.post('/upload', files={'file': ('x.png', self.image_bytes())})
            self.assertEqual(response.status_code, 200)
            self.assertEqual(response.json()['ocr_boxes'], [dict(text='测试', confidence=.9, x=10., y=20., w=70., h=20.)])
            self.assertEqual(client.post('/upload', files={'file': ('x.png', b'bad')}).status_code, 400)

    def test_timeout_retains_capacity_until_inference_finishes(self):
        release = threading.Event()
        finished = threading.Event()

        def slow(gray, turn):
            release.wait(timeout=2)
            finished.set()
            return self.engine(gray, turn)

        with TestClient(server.create_app(timeout=.03, engine_factory=lambda: slow)) as client:
            try:
                files = {'file': ('x.png', self.image_bytes())}
                self.assertEqual(client.post('/upload', files=files).status_code, 504)
                self.assertEqual(client.post('/upload', files=files).status_code, 503)
            finally:
                release.set()
            self.assertTrue(finished.wait(timeout=1))
            time.sleep(.02)
            self.assertEqual(client.post('/upload', files=files).status_code, 200)


if __name__ == '__main__':
    unittest.main()
