"""One process supervises read-only SDK workers for active player sessions."""
import json
import logging
import os
import signal
import sys
import threading
import time
import urllib.parse
import urllib.request

from mywebscrcpy import Client, FrameTimeout, SDKError
from mywebscrcpy.contracts import Deadline
from mywebscrcpy.dependencies import vision_dependencies

STOP = threading.Event()
OUTPUT = threading.Lock()
logging.basicConfig(stream=sys.stderr, level=logging.WARNING)
URL = os.environ['MYWEBSCRCPY_MATCHING_URL']
TOKEN = os.environ['MYWEBSCRCPY_MATCHING_TOKEN']
HEADERS = {'Authorization': 'Bearer ' + TOKEN}
FRAME_UPLOAD_URL = URL.rstrip('/') + '/api/vision/frame'


def emit(kind, **values):
    with OUTPUT:
        print(json.dumps(dict(values, type=kind), ensure_ascii=False), flush=True)


class Worker:
    def __init__(self, serial):
        self.serial = serial
        self.stop = threading.Event()
        self.device = None
        self.lock = threading.Lock()
        self.last_state = None
        self.last_error = None
        self.last_frame_uploaded_at = 0.0
        self.thread = threading.Thread(target=self.run, name='match-' + serial, daemon=True)
        self.thread.start()

    def upload_frame_if_due(self, device, cv2):
        now = time.monotonic()
        if now - self.last_frame_uploaded_at < 1.0:
            return
        try:
            deadline = Deadline(1.0)
            frame = device._session(deadline, video=True).current_frame(deadline)
            if frame is None or getattr(frame, 'image', None) is None:
                return
            ok, buf = cv2.imencode('.jpg', frame.image, [cv2.IMWRITE_JPEG_QUALITY, 80])
            if not ok:
                return
            req = urllib.request.Request(
                FRAME_UPLOAD_URL + '?' + urllib.parse.urlencode({'serial': self.serial}),
                data=buf.tobytes(),
                headers={'Authorization': 'Bearer ' + TOKEN, 'Content-Type': 'image/jpeg'},
                method='POST',
            )
            with urllib.request.urlopen(req, timeout=1.0) as resp:
                _ = resp.read()
            self.last_frame_uploaded_at = now
        except Exception:
            pass

    def state(self, state, error=''):
        if (state, error) != self.last_state:
            self.last_state = (state, error)
            emit('device', serial=self.serial, state=state, error=error)
            return True
        return False

    def run(self):
        _, cv2, _ = vision_dependencies()
        while not self.stop.is_set() and not STOP.is_set():
            try:
                with Client(URL, headers=HEADERS) as client:
                    with client.device(self.serial) as device:
                        with self.lock:
                            self.device = device
                        while not self.stop.is_set() and not STOP.is_set():
                            try:
                                with device.watch_templates(publish=True, max_fps=5, timeout=3) as watch:
                                    if self.last_state is None:
                                        self.state('waiting')
                                    while not self.stop.is_set() and not STOP.is_set():
                                        next(watch)
                                        self.upload_frame_if_due(device, cv2)
                                        self.state('matching')
                                        self.last_error = None
                            except FrameTimeout:
                                self.state('waiting')  # Static screens are not a disconnection.
                                self.stop.wait(.2)
            except Exception as error:
                if not self.stop.is_set() and not STOP.is_set():
                    self.state('failed', str(error)[:500])
                    if self.last_error != str(error):
                        self.last_error = str(error)
                        logging.warning('Matching worker failed: serial=%s error=%s', self.serial, error)
                    self.stop.wait(2)  # Retry reads only; this worker never calls an action.
            finally:
                with self.lock:
                    self.device = None

    def close(self):
        self.stop.set()
        with self.lock:
            if self.device is not None:
                try:
                    self.device.close()
                except SDKError:
                    logging.exception('Worker close failed: %s', self.serial)
        self.thread.join(3)
        if self.thread.is_alive():
            raise RuntimeError('匹配线程未退出: ' + self.serial)
        self.state('removed')


def main():
    vision_dependencies()
    signal.signal(signal.SIGTERM, lambda *_: STOP.set())
    signal.signal(signal.SIGINT, lambda *_: STOP.set())
    threading.Thread(target=lambda: (sys.stdin.buffer.read(), STOP.set()), daemon=True).start()
    workers = {}
    emit('ready')
    try:
        with Client(URL, headers=HEADERS) as client:
            while not STOP.is_set():
                devices = client._json('GET', '/api/matching-runtime/devices', timeout=3)
                if not isinstance(devices, list) or any(not isinstance(v, str) for v in devices):
                    raise RuntimeError('设备列表结构错误')
                desired = set(devices)
                for serial in list(workers):
                    if serial not in desired:
                        workers.pop(serial).close()
                for serial in sorted(desired):
                    if serial not in workers:
                        workers[serial] = Worker(serial)
                STOP.wait(2)
    finally:
        STOP.set()
        for worker in workers.values():
            worker.close()


if __name__ == '__main__':
    main()
