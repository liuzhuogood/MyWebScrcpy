"""Bound conversion work and preserve source provenance under slow consumers."""
import threading
import unittest
from dataclasses import replace
from fractions import Fraction
from unittest.mock import Mock, patch

import av
import numpy as np

from mywebscrcpy.contracts import Deadline
from mywebscrcpy.decoder import Decoder
from mywebscrcpy.models import Frame
from mywebscrcpy.vision import Vision
from mywebscrcpy.watch import Watch
from test_vision import assets, template_wire
from mywebscrcpy.templates import template_from_wire


class VisionPerformanceTests(unittest.TestCase):
    def vision(self):
        session = Vision.__new__(Vision)
        session.serial, session.session_id, session.connection_id = 'phone', 'session', 'connection'
        session._condition = threading.Condition()
        session._closed, session._error = False, None
        session.generation, session._sequence = 1, 0
        session._latest = session._current = session._metadata = None
        session._continuous_publish, session._video = False, True
        session._config = None
        session._decoder = Mock()
        session._pending = {}
        return session

    def feed(self, session, number, image, *, convert=None):
        raw = Mock(width=image.shape[1], height=image.shape[0], shape=image.shape)
        raw.to_ndarray.side_effect = convert
        raw.to_ndarray.return_value = image.copy()
        meta = dict(frame_id=number, pts=number, timestamp=number, received_at=float(number),
                    width=image.shape[1], height=image.shape[0], kind=1,
                    replayed=False, snapshot_complete=False)
        session._decoder.decode.return_value = [(meta, raw)]
        session._metadata = meta
        session._binary(bytes([1]) + number.to_bytes(8, 'big') + b'encoded')
        return raw

    def test_decoder_keeps_native_frames_and_decodes_every_dependency(self):
        _, image, config, packet = assets()
        decoder = Decoder()
        decoder.configure(config)
        for number in range(1, 5):
            decoded = decoder.decode(1, packet, dict(frame_id=number))
            self.assertEqual(len(decoded), 1)
            metadata, frame = decoded[0]
            self.assertIsInstance(frame, av.VideoFrame)
            self.assertEqual(metadata['frame_id'], number)
            # The RGB/YUV round trip may differ by one level even in CRF 0.
            np.testing.assert_allclose(frame.to_ndarray(format='bgr24'), image, rtol=0, atol=1)

    def test_delta_packets_remain_decodable_when_intermediate_images_are_not_converted(self):
        codec = av.CodecContext.create('libx264', 'w')
        codec.width, codec.height, codec.pix_fmt = 96, 64, 'yuv444p'
        codec.time_base = Fraction(1, 30)
        codec.flags |= av.codec.context.Flags.global_header
        codec.options = {'preset': 'ultrafast', 'tune': 'zerolatency', 'crf': '0', 'keyint': '30'}
        packets = []
        image = np.zeros((64, 96, 3), dtype=np.uint8)
        for number in range(10):
            image = np.zeros_like(image)
            image[10:30, number * 4:number * 4 + 20] = [40, 180, 90]
            native = av.VideoFrame.from_ndarray(image, format='bgr24')
            native.pts = number
            packets.extend(codec.encode(native))
        packets.extend(codec.encode(None))
        self.assertTrue(any(not packet.is_keyframe for packet in packets))
        decoder = Decoder()
        decoder.configure(codec.extradata)
        decoded = []
        for number, packet in enumerate(packets, 1):
            decoded.extend(decoder.decode(1 if packet.is_keyframe else 2, bytes(packet), dict(frame_id=number)))
        self.assertEqual(len(decoded), 10)
        # Only materialize the final frame; its references still had to decode.
        np.testing.assert_allclose(decoded[-1][1].to_ndarray(format='bgr24'), image, rtol=0, atol=1)

    def test_slow_consumer_converts_only_latest_frame_once(self):
        session = self.vision()
        image = np.zeros((64, 96, 3), dtype=np.uint8)
        raw = [self.feed(session, i, image) for i in range(1, 21)]
        self.assertEqual(session._decoder.decode.call_count, 20)
        for frame in raw:
            frame.to_ndarray.assert_not_called()
        current = session.current_frame(Deadline(1))
        self.assertEqual(current.frame_id, 20)
        self.assertIsInstance(current.image, np.ndarray)
        self.assertFalse(current.image.flags.writeable)
        self.assertIs(session.current_frame(Deadline(1)), current)
        raw[-1].to_ndarray.assert_called_once_with(format='bgr24')
        for frame in raw[:-1]:
            frame.to_ndarray.assert_not_called()

    def test_conversion_does_not_block_action_reply_and_reset_discards_old_result(self):
        session = self.vision()
        image = np.zeros((64, 96, 3), dtype=np.uint8)
        started, release = threading.Event(), threading.Event()
        def convert(**_):
            started.set()
            if not release.wait(1):
                raise TimeoutError('test conversion not released')
            return image.copy()
        self.feed(session, 1, image, convert=convert)
        frames, failures = [], []
        def consume():
            try:
                frames.append(session.current_frame(Deadline(2)))
            except Exception as error:
                failures.append(error)
        thread = threading.Thread(target=consume)
        thread.start()
        try:
            self.assertTrue(started.wait(.3), 'conversion must happen on demand')
            session._pending['request'] = None
            session._message(dict(type='action.result', device_id='phone', session_id='session',
                                  request_id='request', accepted=True, executed=True, error_code=''))
            self.assertTrue(session._pending['request'].executed)
            session._metadata = dict(kind=3, pts=2, width=96, height=64)
            session._binary(bytes([3]) + (2).to_bytes(8, 'big') + (96).to_bytes(4, 'big') + (64).to_bytes(4, 'big'))
            self.feed(session, 3, image)
        finally:
            release.set()
            thread.join(2)
        self.assertFalse(thread.is_alive())
        self.assertFalse(failures)
        self.assertEqual(frames[0].frame_id, 3)
        self.assertEqual(frames[0].generation, session.generation)

    def watch(self, frames):
        pattern, _, _, _ = assets()
        device = Mock()
        device._lock = threading.RLock()
        device._watch = None
        device._stop = threading.Event()
        device._vision = None
        device._cache.refresh.return_value = [(template_from_wire(template_wire()), pattern)]
        session = Mock()
        session.current_frame.side_effect = frames
        session.publish_current.return_value = True
        device._session.return_value = session
        watch = Watch(device, publish=True, max_fps=30, timeout=1)
        self.addCleanup(watch.close)
        return watch, device, session

    def frame(self):
        _, image, _, _ = assets()
        return Frame(1, 'session', 'phone', 96, 64, 1, 10., image, 1, 'connection')

    def test_static_watch_reuses_work_but_renews_overlay_without_touch_freshness(self):
        frame = self.frame()
        watch, _, session = self.watch([frame] * 3)
        with patch.object(watch._matcher, 'match_frame', wraps=watch._matcher.match_frame) as match:
            batches = [next(watch) for _ in range(3)]
        self.assertEqual(match.call_count, 1)
        self.assertEqual(session.publish_current.call_count, 3)
        self.assertTrue(all(batch[0].received_at == frame.received_at for batch in batches))
        self.assertTrue(all(batch[0].frame_id == frame.frame_id for batch in batches))

    def test_identical_new_pixels_reuse_geometry_with_actual_new_frame_provenance(self):
        first = self.frame()
        second = replace(first, frame_id=2, received_at=11., image=first.image.copy())
        blank = replace(second, frame_id=3, received_at=12., image=np.zeros_like(first.image))
        watch, _, _ = self.watch([first, second, blank])
        with patch.object(watch._matcher, 'match_frame', wraps=watch._matcher.match_frame) as match:
            initial, reused, changed = next(watch), next(watch), next(watch)
        self.assertEqual(match.call_count, 2)
        self.assertTrue(initial)
        self.assertEqual(reused[0].frame_id, 2)
        self.assertEqual(reused[0].received_at, 11.)
        self.assertEqual(changed, [])

    def test_template_refresh_and_generation_change_invalidate_reused_matches(self):
        frame = self.frame()
        rotated = replace(frame, generation=2, frame_id=2)
        watch, device, _ = self.watch([frame, frame, rotated, rotated, rotated])
        with patch.object(watch._matcher, 'match_frame', wraps=watch._matcher.match_frame) as match:
            next(watch)
            # An unchanged asset refresh can reuse work, but content/settings changes cannot.
            watch._refresh_at = 0
            self.assertTrue(next(watch))
            self.assertTrue(next(watch))
            template, image = device._cache.refresh.return_value[0]
            device._cache.refresh.return_value = [(replace(template, threshold=.99), image)]
            watch._refresh_at = 0
            self.assertTrue(next(watch))
            device._cache.refresh.return_value = []
            watch._refresh_at = 0
            self.assertEqual(next(watch), [])
        self.assertEqual(match.call_count, 3)


if __name__ == '__main__':
    unittest.main()
