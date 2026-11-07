"""Explicitly closeable matching iterator, owned by the device lifetime."""
import logging
import math
import time
from dataclasses import replace

from .contracts import Deadline
from .dependencies import vision_dependencies
from .errors import SDKError
from .matcher import Matcher

logger = logging.getLogger(__name__)


class Watch:
    def __init__(self, device, *, publish, max_fps, timeout):
        if type(max_fps) not in {int, float} or not 0 < max_fps <= 30 or type(publish) is not bool:
            raise ValueError('max_fps 必须为 0 到 30，publish 必须为 bool')
        Deadline(timeout)
        device._check_open()
        with device._lock:
            if device._watch is not None:
                raise SDKError('该设备已有持续匹配任务')
            device._watch = self
        self.device, self.publish, self.timeout = device, publish, timeout
        self.interval = 1 / max_fps
        self.closed, self._last, self._refresh_at = False, 0., 0.
        self._templates = []
        self._matcher = Matcher()
        self._matched_frame = None
        self._matched_templates = ()
        self._matches = ()

    def _match(self, frame):
        previous = self._matched_frame
        same_templates = len(self._templates) == len(self._matched_templates) and all(
            template == old_template and image is old_image
            for (template, image), (old_template, old_image) in zip(self._templates, self._matched_templates)
        )
        same_source = previous is not None and (
            frame.session_id, frame.connection_id, frame.generation, frame.width, frame.height
        ) == (
            previous.session_id, previous.connection_id, previous.generation, previous.width, previous.height
        )
        unchanged = same_templates and same_source and frame is previous
        if same_templates and same_source and not unchanged:
            _, _, np = vision_dependencies()
            # Exact pixels only: no downsampling or approximate signatures that
            # could miss small targets. Retain only one previous source image.
            unchanged = np.array_equal(frame.image, previous.image)
        if unchanged:
            if frame is not previous:
                # Identical pixels from a real new frame can reuse geometry.
                # Re-publishing the same old frame keeps its original timestamp.
                self._matches = tuple(replace(match, frame_id=frame.frame_id, session_id=frame.session_id,
                    device_id=frame.device_id, received_at=frame.received_at, generation=frame.generation,
                    connection_id=frame.connection_id, replayed=frame.replayed) for match in self._matches)
        else:
            self._matches = tuple(match for template, image in self._templates
                                  for match in self._matcher.match_frame(frame, template, image))
        self._matched_frame = frame
        self._matched_templates = tuple(self._templates)
        return list(self._matches)

    def __iter__(self):
        return self

    def __next__(self):
        if self.closed:
            raise StopIteration
        try:
            deadline = Deadline(self.timeout)
            wait = self.interval - (time.monotonic() - self._last)
            if wait > 0:
                if self.device._stop.wait(min(wait, deadline.remaining())):
                    raise SDKError('设备已关闭')
            self.device._check_open()
            if time.monotonic() >= self._refresh_at:
                self._templates = self.device._cache.refresh(deadline)
                self._refresh_at = time.monotonic() + 10
            session = self.device._session(deadline, video=True)
            frame = session.current_frame(deadline)
            matches = self._match(frame)
            deadline.remaining()
            if self.publish:
                # Overlay lifetime belongs to this detection batch. Source frame
                # time stays unchanged, so tap_match still rejects old images.
                if not session.publish_current(frame, matches, expires_ms=max(600, math.ceil(self.interval * 2000))):
                    matches = []  # A new image/rotation arrived during matching.
            self._last = time.monotonic()
            return matches
        except Exception:
            self.close()
            raise

    def close(self):
        if self.closed:
            return
        self.closed = True
        self._matched_frame, self._matched_templates, self._matches = None, (), ()
        with self.device._lock:
            if self.device._watch is self:
                self.device._watch = None
        if self.publish and self.device._vision is not None:
            self.device._vision._continuous_publish = False
            try:
                self.device._vision.clear()
            except SDKError as error:
                logger.warning('Could not clear published matches: type=%s', type(error).__name__)

    def __enter__(self):
        return self

    def __exit__(self, *_):
        self.close()
