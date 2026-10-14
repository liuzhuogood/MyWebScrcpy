"""One receiver owns a Vision socket; action replies never compete with frames."""
import json
import logging
import ssl
import threading
import time
import uuid
from urllib.parse import urlencode

import websocket

from .contracts import Deadline, field
from .errors import ActionError, ActionTimeout, ConnectionError, FrameTimeout, ProtocolError, SDKError
from .models import ActionResult, Frame

logger = logging.getLogger(__name__)


class Vision:
    def __init__(self, client, serial, deadline, *, video=False):
        self.serial, self.client = serial, client
        self._condition = threading.Condition()
        self._send_lock = threading.Lock()
        self._closed = False
        self._error = None
        self._latest = None
        self._current = None
        self._continuous_publish = False
        self._heartbeat_sent_at = None
        self._sequence = 0
        self.generation = 0
        self.connection_id = uuid.uuid4().hex
        self._pending = {}
        self._video = video
        self._decoder = None
        self._config = None
        self._metadata = None
        self._last_frame_id = 0
        url = ('wss' + client._base_url[5:] if client._base_url.startswith('https:') else 'ws' + client._base_url[4:]) + '/api/vision/stream?' + urlencode({'serial': serial, 'current_frame': 'true'})
        sslopt = {}
        if client._verify is False:
            sslopt = {'cert_reqs': ssl.CERT_NONE, 'check_hostname': False}
        elif isinstance(client._verify, str):
            sslopt = {'ca_certs': client._verify}
        try:
            self._socket = websocket.create_connection(url, timeout=deadline.remaining(), header=[f'{k}: {v}' for k, v in client._headers.items()], sslopt=sslopt, enable_multithread=True, http_no_proxy=['*'])
            try:
                self._socket.settimeout(deadline.remaining(ConnectionError))
                hello = self._socket.recv()
                if not isinstance(hello, str):
                    raise ProtocolError('握手必须是 JSON')
                hello = json.loads(hello)
                if field(hello, 'type', str) != 'hello' or field(hello, 'protocol_version', str) != '1' or field(hello, 'format', str) != 'scrcpy-frame' or field(hello, 'codec', str) != 'h264':
                    raise ProtocolError('不支持的 Vision 握手')
                self.session_id = field(hello, 'session_id', str)
                if not self.session_id or field(hello, 'device_id', str) != serial or field(hello, 'width', int) <= 0 or field(hello, 'height', int) <= 0:
                    raise ProtocolError('握手设备、会话或尺寸不合法')
                self._socket.settimeout(deadline.remaining(ConnectionError))
                self._send('hello', max_fps=0)  # Decode every encoded packet; throttle matching instead.
                self._socket.settimeout(.2)
                deadline.remaining(ConnectionError)
            except Exception:
                self._socket.close()
                raise
        except (websocket.WebSocketException, OSError) as error:
            raise ConnectionError('Vision 连接失败') from error
        except (ValueError, TypeError) as error:
            raise ProtocolError('握手 JSON 不合法') from error
        self._thread = threading.Thread(target=self._read, name='mywebscrcpy-vision', daemon=True)
        self._thread.start()
        logger.info('Vision connected: serial=%s session=%s', serial, self.session_id)

    def _check(self):
        if self._error is not None:
            raise self._error
        if self._closed:
            raise SDKError('Vision 已关闭')

    def _send(self, kind, *, _deadline=None, **fields):
        acquired = self._send_lock.acquire(timeout=_deadline.remaining(ActionTimeout)) if _deadline is not None else self._send_lock.acquire()
        if not acquired:
            raise ActionTimeout('发送锁等待超过动作期限')
        try:
            self._check()
            if _deadline is not None:
                fields["expires_ms"] = max(1, round(_deadline.remaining(ActionTimeout) * 1000))
            try:
                self._socket.send(json.dumps(dict(fields, type=kind, protocol_version='1', device_id=self.serial, session_id=self.session_id)))
            except (websocket.WebSocketException, OSError) as error:
                raise ConnectionError('Vision 发送失败') from error
        finally:
            self._send_lock.release()

    def _read(self):
        heartbeat = time.monotonic()
        try:
            while not self._closed:
                try:
                    message = self._socket.recv()
                except websocket.WebSocketTimeoutException:
                    message = None
                if message == '' or message == b'':
                    raise ConnectionError('Vision 已断流')
                if isinstance(message, str):
                    self._message(json.loads(message))
                elif isinstance(message, bytes):
                    self._binary(message)
                now = time.monotonic()
                if self._heartbeat_sent_at is not None and now - self._heartbeat_sent_at >= 5:
                    raise ConnectionError('Vision 心跳响应超时')
                if now - heartbeat >= 5 and self._heartbeat_sent_at is None:
                    self._heartbeat_sent_at = now
                    self._send('heartbeat'); heartbeat = now
        except Exception as error:
            if not self._closed:
                known = error if isinstance(error, SDKError) else ProtocolError(f'Vision 收包失败：{type(error).__name__}')
                logger.warning('Vision receiver stopped: serial=%s type=%s', self.serial, type(known).__name__)
                with self._condition:
                    self._error = known
                    self._latest = self._current = None
                    self._condition.notify_all()
        finally:
            self._socket.close()

    def _message(self, value):
        kind = field(value, 'type', str)
        if kind == 'error':
            raise ProtocolError('Vision 服务错误：' + field(value, 'code', str, '') + ' ' + field(value, 'message', str, ''))
        if field(value, 'device_id', str) != self.serial or field(value, 'session_id', str) != self.session_id:
            raise ProtocolError('Vision 设备或会话发生变化')
        if kind == 'frame':
            if self._metadata is not None:
                raise ProtocolError('视频元数据尚未收到对应二进制包')
            meta = {name: field(value, name, int) for name in ('frame_id', 'pts', 'timestamp', 'width', 'height', 'kind')}
            if field(value, 'codec', str) != 'h264' or type(value.get('replayed')) is not bool or meta['frame_id'] <= self._last_frame_id or meta['kind'] not in {0, 1, 2, 3} or min(meta['width'], meta['height']) <= 0:
                raise ProtocolError('视频元数据不合法')
            meta['replayed'] = field(value, 'replayed', bool)
            # An optional explicit marker proves replay has reached the current
            # image. A lone cached key frame is only decoder bootstrap.
            meta['snapshot_complete'] = field(value, 'snapshot_complete', bool, False)
            meta['received_at'] = time.monotonic()
            self._last_frame_id = meta['frame_id']
            self._metadata = meta
        elif kind == 'action.result':
            result = ActionResult(field(value, 'request_id', str), self.serial, self.session_id, field(value, 'accepted', bool), field(value, 'executed', bool), field(value, 'error_code', str))
            with self._condition:
                if result.request_id in self._pending:
                    self._pending[result.request_id] = result
                    self._condition.notify_all()
        elif kind == 'hello.ack':
            if field(value, 'max_fps', int) != 0:
                raise ProtocolError('服务端未确认完整视频包订阅')
        elif kind == 'heartbeat':
            if field(value, 'timestamp', int) <= 0:
                raise ProtocolError('心跳时间戳不合法')
            self._heartbeat_sent_at = None
        else:
            raise ProtocolError('未知 Vision 消息：' + kind)

    def _binary(self, payload):
        meta, self._metadata = self._metadata, None
        if meta is None or len(payload) < 9 or payload[0] != meta['kind'] or int.from_bytes(payload[1:9], 'big') != meta['pts']:
            raise ProtocolError('视频元数据与二进制不匹配')
        kind = meta['kind']
        if kind in {0, 3}:
            if kind == 3 and (len(payload) != 17 or int.from_bytes(payload[9:13], 'big') != meta['width'] or int.from_bytes(payload[13:17], 'big') != meta['height']):
                raise ProtocolError('旋转包尺寸不一致')
            with self._condition:
                self.generation += 1
                self._latest = self._current = None
                self._condition.notify_all()
            if self._continuous_publish:
                self.clear()
            self._config = payload[9:] if kind == 0 else None
            if self._decoder is not None:
                self._decoder.reset()
            if kind == 3:
                return
        if not self._video:
            return
        if self._decoder is None:
            from .decoder import Decoder
            self._decoder = Decoder()
            if self._config is not None and kind != 0:
                self._decoder.configure(self._config)
        if kind == 0:
            self._decoder.configure(payload[9:])
            return
        for source, image in self._decoder.decode(kind, payload[9:], meta):
            if source['replayed'] and not source['snapshot_complete']:
                continue  # Cached key frames initialize decoding, never satisfy fresh frame requests.
            height, width = image.shape[:2]
            if (width, height) != (source['width'], source['height']):
                raise ProtocolError('解码尺寸与来源元数据不一致')
            image.setflags(write=False)
            frame = Frame(source['frame_id'], self.session_id, self.serial, width, height, source['timestamp'], source['received_at'], image, self.generation, self.connection_id, source['replayed'])
            with self._condition:
                self._current = frame
                if not frame.replayed:
                    self._latest = frame
                    self._sequence += 1
                self._condition.notify_all()

    def frame(self, deadline):
        from .dependencies import vision_dependencies
        vision_dependencies()
        with self._condition:
            self._check()
            self._video = True
            previous = self._sequence
            after = time.monotonic()
            while self._sequence == previous or self._latest is None or self._latest.received_at <= after:
                self._condition.wait(deadline.remaining(FrameTimeout))
                self._check()
            deadline.remaining(FrameTimeout)
            return self._latest

    def current_frame(self, deadline):
        """Read-only matching can reuse the same image until a reset or disconnect."""
        from .dependencies import vision_dependencies
        vision_dependencies()
        with self._condition:
            self._check()
            self._video = True
            while self._current is None:
                self._condition.wait(deadline.remaining(FrameTimeout))
                self._check()
            deadline.remaining(FrameTimeout)
            return self._current

    def action(self, action, deadline, **fields):
        request_id = uuid.uuid4().hex
        with self._condition:
            self._check()
            self._pending[request_id] = None
        try:
            self._send('action.request', _deadline=deadline, request_id=request_id, action=action, **fields)
            with self._condition:
                while self._pending[request_id] is None:
                    self._condition.wait(deadline.remaining(ActionTimeout))
                    self._check()
                result = self._pending[request_id]
            if not result.accepted or not result.executed or result.error_code:
                raise ActionError(result)
            logger.info('Action completed: serial=%s request=%s action=%s', self.serial, request_id, action)
            return result
        except ActionTimeout:
            logger.warning('Action outcome unknown: serial=%s request=%s action=%s', self.serial, request_id, action)
            raise
        finally:
            with self._condition:
                self._pending.pop(request_id, None)

    def publish(self, frame, matches):
        if frame.replayed:
            raise SDKError('回放画面不能作为新帧发布')
        if frame.device_id != self.serial or frame.session_id != self.session_id or frame.generation != self.generation or frame.connection_id != self.connection_id:
            raise SDKError('不能发布旧会话或重置前的匹配结果')
        age = (time.monotonic() - frame.received_at) * 1000
        if age >= 600:
            raise SDKError('匹配结果超过 600ms')
        self._send('detection.result', frame_id=frame.frame_id, timestamp=round(time.time() * 1000), expires_ms=max(1, int(600 - age)), objects=[m.to_object() for m in matches])

    def publish_current(self, frame, matches, *, expires_ms):
        with self._condition:
            self._check()
            current = self._current
            if current is None or frame.generation != current.generation or frame.connection_id != current.connection_id:
                return False
            lifetime = expires_ms
            if current is not frame:
                # A moving stream may advance while matching. Keep the existing
                # fresh-frame allowance, but never renew a superseded old image.
                remaining = int(600 - (time.monotonic() - frame.received_at) * 1000)
                if remaining <= 0:
                    return False
                lifetime = min(lifetime, remaining)
            self._continuous_publish = True
            self._send('detection.result', frame_id=frame.frame_id, timestamp=round(time.time() * 1000), expires_ms=lifetime, objects=[m.to_object() for m in matches])
            return True

    def clear(self):
        self._send('detection.result', timestamp=round(time.time() * 1000), expires_ms=600, objects=[])

    def close(self):
        with self._condition:
            if self._closed:
                return
            self._closed = True
            self._latest = self._current = None
            self._condition.notify_all()
        self._socket.close(timeout=1)
        if threading.current_thread() is not self._thread:
            self._thread.join(timeout=2)
            if self._thread.is_alive():
                raise SDKError('Vision 收包线程尚未退出')
        logger.info('Vision closed: serial=%s', self.serial)
