"""One process supervises read-only SDK workers for active player sessions."""
import json
import logging
import os
import signal
import sys
import threading

from mywebscrcpy import Client, FrameTimeout, SDKError
from mywebscrcpy.dependencies import vision_dependencies

STOP = threading.Event()
OUTPUT = threading.Lock()
logging.basicConfig(stream=sys.stderr, level=logging.WARNING)
URL = os.environ['MYWEBSCRCPY_MATCHING_URL']
TOKEN = os.environ['MYWEBSCRCPY_MATCHING_TOKEN']
HEADERS = {'Authorization': 'Bearer ' + TOKEN}


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
        self.thread = threading.Thread(target=self.run, name='match-' + serial, daemon=True)
        self.thread.start()

    def state(self, state, error=''):
        if (state, error) != self.last_state:
            self.last_state = (state, error)
            emit('device', serial=self.serial, state=state, error=error)
            return True
        return False

    def run(self):
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
