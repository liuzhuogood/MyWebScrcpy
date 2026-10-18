"""Serial fresh-frame sampling within a window and a single call deadline."""
from concurrent.futures import TimeoutError as FutureTimeout
import logging
import time

from .contracts import Deadline
from .errors import FrameTimeout
from .ocr import OCRResult, crop_image, perform_ocr
from .ocr_consensus import fuse_boxes, global_boxes, response_boxes
from .ocr_workers import submit_ocr

logger = logging.getLogger(__name__)


def _frame_context(frame):
    return (frame.session_id, frame.device_id, frame.connection_id,
            frame.generation, frame.width, frame.height)


def _validate_frame(session, frame):
    session._check()
    if (frame.replayed or frame.frame_id <= 0 or frame.session_id != session.session_id
            or frame.generation != session.generation or frame.connection_id != session.connection_id):
        raise FrameTimeout("OCR 画面已回放、重置或来自旧会话")


def _recognize(image, deadline, options):
    if options["engine"] == "parallel":
        return perform_ocr(image, timeout=deadline.remaining(FrameTimeout), **options)
    future = submit_ocr(perform_ocr, image, _deadline=deadline,
                        timeout=deadline.remaining(FrameTimeout), **options)
    try:
        result = future.result(timeout=deadline.remaining(FrameTimeout))
        deadline.remaining(FrameTimeout)
        return result
    except FutureTimeout as error:
        raise FrameTimeout("OCR 识别超过调用总期限") from error


def stabilized_ocr(device, *, stabilization_ms, bbox, keywords, mark, mark_duration_ms, timeout, **options):
    deadline = Deadline(timeout)
    started = time.monotonic()
    window_end = min(deadline.end, started + stabilization_ms / 1000)
    bbox = list(bbox) if bbox is not None else [0., 0., 1., 1.]
    if len(bbox) != 4 or any(not (0 <= v <= 1) for v in bbox):
        raise ValueError("bbox 必须包含 4 个 0 到 1 之间的数值 [x, y, w, h]")
    session = device._session(deadline, video=True)
    rounds, errors, engines_used = [], [], []
    previous, last_frame, engine_used = None, None, options["engine"]
    next_at = started
    for _ in range(101):
        device._check_open()
        delay = min(next_at, window_end) - time.monotonic()
        if delay > 0 and device._stop.wait(delay):
            device._check_open()
        if time.monotonic() >= window_end:
            break
        # Frame acquisition itself is bounded by the sampling window, while
        # an engine round that has already started may use the total budget.
        frame_remaining = window_end - time.monotonic()
        if frame_remaining <= 0:
            break
        frame_deadline = Deadline(frame_remaining)
        requested_at = time.monotonic()
        try:
            frame = session.frame(frame_deadline)
        except FrameTimeout:
            if not rounds:
                raise
            break
        if time.monotonic() >= window_end:
            break
        _validate_frame(session, frame)
        if frame.device_id != device.serial or frame.received_at <= requested_at:
            raise FrameTimeout("OCR 未取得当前设备的新帧")
        context = _frame_context(frame)
        if previous is not None:
            if context != _frame_context(previous):
                rounds.clear()  # Never combine different pages' coordinate systems.
            elif frame.frame_id <= previous.frame_id or frame.received_at <= previous.received_at:
                raise FrameTimeout("OCR 未取得新帧")
        round_started = time.monotonic()
        crop, pixel_bbox = crop_image(frame.image, bbox)
        height, width = frame.image.shape[:2]
        actual_bbox = [pixel_bbox["x"] / width, pixel_bbox["y"] / height,
                       pixel_bbox["width"] / width, pixel_bbox["height"] / height]
        result, engine_used, failures = _recognize(crop, deadline, options)
        _validate_frame(session, frame)
        rounds.append(fuse_boxes([global_boxes(response_boxes(result, engine_used, crop.shape), actual_bbox)]))
        used = result["engines_used"] if engine_used == "parallel" else [engine_used]
        engines_used.extend(engine for engine in used if engine not in engines_used)
        errors.extend(failure for failure in failures if failure not in errors)
        previous = last_frame = frame
        next_at = round_started + .05
    if not rounds:
        raise FrameTimeout("OCR 采样窗口内未取得有效识别结果")
    _validate_frame(session, last_frame)
    boxes = fuse_boxes(rounds, temporal=True)
    full_text = "\n".join(box.text for box in boxes)
    wanted = []
    if isinstance(keywords, str):
        wanted = [value.strip().casefold() for value in keywords.split(",") if value.strip()]
    elif isinstance(keywords, (list, tuple)):
        wanted = [str(value).strip().casefold() for value in keywords if str(value).strip()]
    if wanted:
        boxes = [box for box in boxes if any(value in box.text.casefold() for value in wanted)]
    result = OCRResult("matched" if boxes else "not_found", bool(boxes), full_text,
                       boxes, engine_used, last_frame.frame_id,
                       round((time.monotonic() - started) * 1000), errors,
                       samples=len(rounds), engines_used=engines_used)
    if mark and boxes:
        deadline.remaining(FrameTimeout)
        session.publish_current(last_frame, boxes, expires_ms=mark_duration_ms)
    logger.debug("OCR stabilized: serial=%s rounds=%d boxes=%d", device.serial, len(rounds), len(boxes))
    return result
