import json
import threading
import time
import unittest
from dataclasses import replace
from fractions import Fraction

import av
import cv2
import httpx
import numpy as np
from websockets.sync.server import serve
from websockets.exceptions import ConnectionClosed

from mywebscrcpy import (ActionError, ActionTimeout, Client, FrameTimeout, MatchTimeout,
                         Matcher, ProtocolError, SDKError, TemplateDisabled, UnsupportedTemplateMethod)
from mywebscrcpy import ConnectionError as SDKConnectionError
from mywebscrcpy.decoder import configuration
from mywebscrcpy.templates import template_from_wire


def assets():
    pattern = np.random.default_rng(4).integers(0, 255, (12, 15, 3), dtype=np.uint8)
    image = np.zeros((64, 96, 3), dtype=np.uint8)
    image[23:35, 37:52] = pattern
    codec = av.CodecContext.create('libx264', 'w')
    codec.width, codec.height, codec.pix_fmt = 96, 64, 'yuv444p'
    codec.time_base = Fraction(1, 10)
    codec.flags |= av.codec.context.Flags.global_header
    codec.options = {'preset': 'ultrafast', 'tune': 'zerolatency', 'crf': '0', 'keyint': '1'}
    frame = av.VideoFrame.from_ndarray(image, format='bgr24'); frame.pts = 0
    packets = codec.encode(frame) + codec.encode(None)
    return pattern, image, codec.extradata, bytes(packets[0])


def template_wire():
    return dict(id='target', name='目标', serial='global', threshold=.95, method='TM_CCOEFF_NORMED', grayscale=False,
                scales=[1.], enabled=True, width=15, height=12, image_count=1, created_at='2026-10-13T00:00:00Z', updated_at='2026-10-13T00:00:00Z')


class Fixture:
    def __init__(self, *, static=False, malformed=False, fail_action=False, no_reply=False, reverse=False, replayed=False, snapshot=False, heartbeat=True):
        self.pattern, self.image, self.config, self.packet = assets()
        self.static, self.malformed, self.replayed = static, malformed, replayed
        self.snapshot = snapshot
        self.heartbeat = heartbeat
        self.enabled = True
        self.fail_action, self.no_reply, self.reverse = fail_action, no_reply, reverse
        self.sent, self.actions, self.messages = [], [], []
        self.stop = threading.Event()
        self.reset = threading.Event()
        self.server = serve(self.handler, '127.0.0.1', 0)
        self.port = self.server.socket.getsockname()[1]
        self.thread = threading.Thread(target=self.server.serve_forever, daemon=True)
        self.thread.start()

    def handler(self, ws):
        try:
            ws.send(json.dumps(dict(type='hello', protocol_version='1', device_id='phone', session_id='session', format='scrcpy-frame', codec='h264', width=96, height=64)))
            hello = json.loads(ws.recv(timeout=2)); self.messages.append(hello)
            ws.send(json.dumps(dict(type='hello.ack', device_id='phone', session_id='session', max_fps=0)))
            frame_id = 0
            def packet(kind, payload):
                nonlocal frame_id
                frame_id += 1
                pts = frame_id * 100000
                ws.send(json.dumps(dict(type='frame', device_id='phone', session_id='session', frame_id=frame_id, pts=pts, timestamp=round(time.time()*1000), width=96, height=64, kind=kind, codec='h264', replayed=self.replayed, snapshot_complete=self.snapshot and kind == 1)))
                ws.send(bytes([kind]) + (pts + int(self.malformed)).to_bytes(8, 'big') + payload)
            packet(0, self.config)
            if self.snapshot:
                packet(1, self.packet)
            last = 0
            pending = []
            while not self.stop.is_set():
                if self.reset.is_set():
                    self.reset.clear()
                    packet(3, (96).to_bytes(4,'big')+(64).to_bytes(4,'big'))
                    packet(0, self.config)
                if not self.static and time.monotonic() - last >= .05:
                    packet(1, self.packet); last = time.monotonic()
                try:
                    message = json.loads(ws.recv(timeout=.01))
                except TimeoutError:
                    continue
                self.messages.append(message)
                if message['type'] == 'heartbeat' and self.heartbeat:
                    ws.send(json.dumps(dict(type='heartbeat', device_id='phone', session_id='session', timestamp=round(time.time()*1000))))
                if message['type'] == 'action.request':
                    self.actions.append(message)
                    pending.append(message)
                    if self.no_reply or self.reverse and len(pending) < 2:
                        continue
                    for action in reversed(pending):
                        ws.send(json.dumps(dict(type='action.result', device_id='phone', session_id='session', request_id=action['request_id'], accepted=True, executed=not self.fail_action, error_code='blocked' if self.fail_action else '')))
                    pending.clear()
                if message['type'] == 'detection.result':
                    self.sent.append(message)
        except ConnectionClosed:
            pass  # Client deliberately closes the peer in lifecycle cases.

    def client(self):
        client = Client(f'http://127.0.0.1:{self.port}')
        client._http.close()
        _, png = cv2.imencode('.png', self.pattern)
        def handle(request):
            if request.url.path.endswith('/image'):
                return httpx.Response(200, content=png.tobytes(), headers={'content-type': 'image/png'})
            if request.url.path == '/api/templates':
                templates = [template_wire()] if self.enabled else []
                return httpx.Response(200, json={'templates': templates, 'total': len(templates)})
            return httpx.Response(200, json=template_wire())
        client._http = httpx.Client(base_url=client._base_url, transport=httpx.MockTransport(handle))
        return client

    def close(self):
        self.stop.set(); self.server.shutdown(); self.thread.join(2)


class VisionTests(unittest.TestCase):
    def fixture(self, **options):
        fixture = Fixture(**options); self.addCleanup(fixture.close)
        return fixture

    def test_real_h264_decode_match_and_explicit_click(self):
        fixture = self.fixture()
        with fixture.client() as client, client.device('phone') as device:
            target = device.wait_for('target', timeout=3)
            self.assertAlmostEqual(target.x, 37/96, places=3)
            self.assertAlmostEqual(target.y, 23/64, places=3)
            self.assertGreater(target.confidence, .99)
            self.assertFalse(fixture.actions, 'matching must not click implicitly')
            result = device.tap_match(target, timeout=1)
            self.assertTrue(result.executed)
            self.assertEqual(fixture.messages[0]['max_fps'], 0)
            action = fixture.actions[0]
            self.assertEqual(action['frame_id'], target.frame_id)
            self.assertEqual(action['protocol_version'], '1')
            with self.assertRaises(SDKError): device.tap_match(replace(target, session_id='old'))
            with self.assertRaises(SDKError): device.tap_match(replace(target, generation=target.generation-1))
            with self.assertRaises(SDKError): device.tap_match(replace(target, received_at=time.monotonic()-2))
            thread = device._vision._thread
        self.assertFalse(thread.is_alive())

    def test_action_failures_and_unknown_outcomes_never_retry(self):
        for options, error in [({'fail_action': True}, ActionError), ({'no_reply': True}, ActionTimeout)]:
            fixture = self.fixture(**options)
            with fixture.client() as client, client.device('phone') as device:
                with self.assertRaises(error): device.tap((.2,.3),timeout=.4)
                self.assertEqual(len(fixture.actions),1)

    def test_send_lock_does_not_extend_action_deadline(self):
        from mywebscrcpy.contracts import Deadline
        fixture = self.fixture()
        with fixture.client() as client, client.device('phone') as device:
            session = device._session(Deadline(2))
            with session._send_lock:
                started = time.monotonic()
                with self.assertRaises(ActionTimeout): device.key(3, timeout=.05)
                self.assertLess(time.monotonic()-started, .3)
            self.assertFalse(fixture.actions)

    def test_single_receiver_routes_out_of_order_action_results(self):
        fixture = self.fixture(reverse=True)
        with fixture.client() as client, client.device('phone') as device:
            device._session(__import__('mywebscrcpy.contracts',fromlist=['Deadline']).Deadline(2))
            results, errors = [], []
            def invoke(code):
                try: results.append(device.key(code, timeout=2))
                except Exception as error: errors.append(error)
            threads = [threading.Thread(target=invoke,args=(code,)) for code in (3,4)]
            for thread in threads: thread.start()
            for thread in threads: thread.join(3)
            self.assertFalse(errors)
            self.assertEqual(len({r.request_id for r in results}),2)

    def test_static_stream_is_timeout_not_old_frame_or_empty_matches(self):
        fixture = self.fixture(static=True)
        with fixture.client() as client, client.device('phone') as device:
            with self.assertRaises(FrameTimeout): device.frame(timeout=.4)
            with self.assertRaises(MatchTimeout): device.wait_for('target',timeout=.4)

    def test_replayed_cache_is_decoded_but_never_returned_as_fresh(self):
        fixture = self.fixture(replayed=True)
        with fixture.client() as client, client.device('phone') as device:
            with self.assertRaises(FrameTimeout): device.frame(timeout=.3)

    def test_disconnect_is_connection_error(self):
        fixture = self.fixture()
        with fixture.client() as client, client.device('phone') as device:
            device.frame(timeout=2)
            fixture.stop.set()
            with self.assertRaises(SDKConnectionError): device.frame(timeout=1)

    def test_malformed_pair_is_protocol_error(self):
        fixture = self.fixture(malformed=True)
        with fixture.client() as client, client.device('phone') as device:
            with self.assertRaises(ProtocolError): device.frame(timeout=1)

    def test_watch_single_owner_publish_and_clear(self):
        fixture = self.fixture()
        with fixture.client() as client, client.device('phone') as device:
            with device.watch_templates(publish=True,max_fps=5) as watch:
                self.assertTrue(next(watch))
                with self.assertRaises(SDKError): device.watch_templates()
                next(watch)
            time.sleep(.05)
            self.assertTrue(any(m['objects'] for m in fixture.sent))
            self.assertEqual(fixture.sent[-1]['objects'],[])
            self.assertEqual(set(fixture.sent[0]['objects'][0]),{'label','confidence','x','y','w','h'})

    def test_watch_rechecks_static_image_without_renewing_click_freshness(self):
        fixture = self.fixture()
        with fixture.client() as client, client.device('phone') as device:
            with device.watch_templates(publish=True, max_fps=5, timeout=.5) as watch:
                self.assertTrue(next(watch))
                fixture.static = True
                time.sleep(.1)  # Let the last already queued frame finish decoding.
                initial = next(watch)[0]
                started_ms = round(time.time() * 1000)
                batches = [next(watch) for _ in range(18)]  # Beyond the worker's old 3s timeout.
                self.assertTrue(all(batch for batch in batches))
                self.assertTrue(all(batch[0].frame_id == initial.frame_id for batch in batches))
                self.assertTrue(all(batch[0].received_at == initial.received_at for batch in batches))
                with self.assertRaises(SDKError): device.tap_match(batches[-1][0])
                self.assertFalse(fixture.actions)
                time.sleep(.05)
                self.assertGreater(fixture.sent[-1]['timestamp'], started_ms + 3000)
                self.assertEqual(fixture.sent[-1]['expires_ms'], 600)
                fixture.enabled = False
                watch._refresh_at = 0
                self.assertEqual(next(watch), [])
                time.sleep(.05)
                self.assertEqual(fixture.sent[-1]['objects'], [])

    def test_watch_uses_new_image_and_retains_normal_click_freshness(self):
        fixture = self.fixture()
        with fixture.client() as client, client.device('phone') as device:
            with device.watch_templates(publish=True, timeout=1) as watch:
                first = next(watch)[0]
                second = next(watch)[0]
                self.assertGreater(second.frame_id, first.frame_id)
                self.assertTrue(device.tap_match(second).executed)
                # Replace the source with an actual newly encoded blank image.
                codec = av.CodecContext.create('libx264', 'w')
                codec.width, codec.height, codec.pix_fmt = 96, 64, 'yuv444p'
                codec.time_base = Fraction(1, 10)
                codec.flags |= av.codec.context.Flags.global_header
                codec.options = {'preset': 'ultrafast', 'tune': 'zerolatency', 'crf': '0', 'keyint': '1'}
                blank = av.VideoFrame.from_ndarray(np.zeros_like(fixture.image), format='bgr24')
                blank.pts = 0
                fixture.packet = bytes((codec.encode(blank) + codec.encode(None))[0])
                limit = time.monotonic() + 1
                while next(watch):
                    self.assertLess(time.monotonic(), limit)
                time.sleep(.05)
                self.assertEqual(fixture.sent[-1]['objects'], [])

    def test_watch_static_bootstrap_is_read_only_and_reset_discards_cache(self):
        fixture = self.fixture(static=True, replayed=True, snapshot=True)
        with fixture.client() as client, client.device('phone') as device:
            with device.watch_templates(publish=True, timeout=.3) as watch:
                target = next(watch)[0]
                self.assertTrue(target.replayed)
                with self.assertRaises(SDKError): device.tap_match(target)
                with self.assertRaises(FrameTimeout): device.frame(timeout=.1)
                fixture.reset.set()
                limit = time.monotonic() + 1
                while device._vision.generation == target.generation and time.monotonic() < limit:
                    time.sleep(.01)
                self.assertIsNone(device._vision._current)
                with self.assertRaises(FrameTimeout): next(watch)
                time.sleep(.05)
                self.assertEqual(fixture.sent[-1]['objects'], [])
                self.assertFalse(fixture.actions)

    def test_watch_disconnect_stops_static_cache_reuse(self):
        fixture = self.fixture(static=True, replayed=True, snapshot=True)
        with fixture.client() as client, client.device('phone') as device:
            with device.watch_templates(publish=True, timeout=.5) as watch:
                self.assertTrue(next(watch))
                fixture.stop.set()
                fixture.server.shutdown()
                limit = time.monotonic() + 1
                while device._vision._error is None and time.monotonic() < limit:
                    time.sleep(.01)
                with self.assertRaises(SDKConnectionError): next(watch)
                self.assertIsNone(device._vision._current)

    def test_watch_missing_heartbeat_response_stops_cache_reuse(self):
        fixture = self.fixture(static=True, replayed=True, snapshot=True, heartbeat=False)
        with fixture.client() as client, client.device('phone') as device:
            with device.watch_templates(publish=True, max_fps=5, timeout=.5) as watch:
                self.assertTrue(next(watch))
                started = time.monotonic()
                with self.assertRaises(SDKConnectionError):
                    while time.monotonic() - started < 12:
                        self.assertTrue(next(watch))
                self.assertIsNone(device._vision._current)
                self.assertFalse(fixture.actions)

    def test_reset_invalidates_match_and_slow_consumer_gets_newest_frame(self):
        fixture = self.fixture()
        with fixture.client() as client, client.device('phone') as device:
            target = device.wait_for('target',timeout=2)
            first = device.frame(timeout=2)
            time.sleep(.2)
            after = time.monotonic()
            latest = device.frame(timeout=2)
            self.assertGreater(latest.frame_id,first.frame_id+2)
            self.assertGreater(latest.received_at,after)
            fixture.reset.set()
            limit = time.monotonic()+1
            while device._vision.generation == target.generation and time.monotonic()<limit:
                time.sleep(.01)
            with self.assertRaises(SDKError): device.tap_match(target)
            self.assertTrue(device.wait_for('target',timeout=2))

    def test_close_interrupts_frame_wait_and_reclaims_thread(self):
        fixture = self.fixture(static=True)
        with fixture.client() as client:
            device=client.device('phone')
            errors=[]
            def wait():
                try: device.frame(timeout=3)
                except Exception as error: errors.append(error)
            thread=threading.Thread(target=wait);thread.start()
            limit=time.monotonic()+1
            while device._vision is None and time.monotonic()<limit: time.sleep(.01)
            device.close();thread.join(1)
            self.assertFalse(thread.is_alive())
            self.assertTrue(errors and isinstance(errors[0],SDKError))

    def test_offline_match_and_contracts(self):
        pattern,image,config,_ = assets()
        matches=Matcher().match_image(image,pattern,grayscale=False,threshold=.99,scales=(.5,1.,2.))
        self.assertEqual(len(matches),1)
        self.assertEqual(len(Matcher().match_image(image,pattern,grayscale=True,threshold=.99)),1)
        enlarged = cv2.resize(pattern, None, fx=2, fy=2)
        scaled_image = np.zeros_like(image)
        scaled_image[8:32, 12:42] = enlarged
        self.assertEqual(len(Matcher().match_image(scaled_image,pattern,grayscale=False,scales=(2.,),threshold=.99)),1)
        self.assertEqual(Matcher().match_image(np.zeros_like(image),pattern),[])
        self.assertEqual(Matcher().match_image(image,pattern,scales=(.01,)),[])
        with self.assertRaises(UnsupportedTemplateMethod): Matcher().match_image(image,pattern,method='guess')
        with self.assertRaises(SDKError): Matcher().match_image(image,np.zeros_like(pattern))
        for value in [{}, dict(template_wire(), enabled='true'), dict(template_wire(), scales=[]), dict(template_wire(), threshold=float('nan'))]:
            with self.assertRaises(ProtocolError): template_from_wire(value)
        with self.assertRaises(ProtocolError): configuration(b'\x01\x00')


if __name__ == '__main__': unittest.main()
