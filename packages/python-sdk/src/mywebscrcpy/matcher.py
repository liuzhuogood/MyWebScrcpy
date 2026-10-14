"""Deterministic local matching using OpenCV; no network or implicit actions."""
import logging
import time

from .dependencies import vision_dependencies
from .errors import ProtocolError, SDKError, UnsupportedTemplateMethod
from .models import Frame, Match

logger = logging.getLogger(__name__)


class Matcher:
    def match_image(self, image, template, *, template_id='', name='template', threshold=.88, grayscale=True, scales=(1.0,), max_matches=5, method='TM_CCOEFF_NORMED'):
        _, cv2, np = vision_dependencies()
        if method != 'TM_CCOEFF_NORMED':
            raise UnsupportedTemplateMethod(method)
        for value in (image, template):
            if not isinstance(value, np.ndarray) or value.dtype != np.uint8 or value.ndim != 3 or value.shape[2] != 3 or min(value.shape[:2]) <= 0:
                raise ValueError('输入必须是非空 uint8 BGR ndarray')
        if type(threshold) not in {int, float} or not 0 < threshold <= 1 or type(grayscale) is not bool or type(max_matches) is not int or not 1 <= max_matches <= 100:
            raise ValueError('匹配参数不合法')
        if not scales or any(type(s) not in {int, float} or not 0 < s < float('inf') for s in scales):
            raise ValueError('scales 必须包含有限正数')
        source = cv2.cvtColor(image, cv2.COLOR_BGR2GRAY) if grayscale else image
        pattern = cv2.cvtColor(template, cv2.COLOR_BGR2GRAY) if grayscale else template
        if float(np.max(np.std(pattern.astype(np.float64), axis=(0, 1)))) < 1e-6:
            raise SDKError('模板没有足够纹理，无法执行归一化相关匹配')
        height, width = source.shape[:2]
        boxes, scores = [], []
        for scale in scales:
            th, tw = max(1, round(pattern.shape[0] * scale)), max(1, round(pattern.shape[1] * scale))
            if th > height or tw > width:
                continue
            scaled = pattern if scale == 1 else cv2.resize(pattern, (tw, th))
            if float(np.max(np.std(scaled.astype(np.float64), axis=(0, 1)))) < 1e-6:
                continue  # A tiny scale may remove all texture and falsely match every pixel.
            response = cv2.matchTemplate(source, scaled, cv2.TM_CCOEFF_NORMED)
            ys, xs = np.where((response >= threshold) & (response == cv2.dilate(response, np.ones((3, 3), np.uint8))))
            if len(xs) > 4096:
                top = np.argpartition(response[ys, xs], -4096)[-4096:]
                ys, xs = ys[top], xs[top]
            for x, y in zip(xs, ys):
                score = float(response[y, x])
                if not np.isfinite(score):
                    raise ProtocolError('匹配置信度不合法')
                boxes.append([int(x), int(y), tw, th]); scores.append(min(1., max(0., score)))
        if not boxes:
            return []
        indices = cv2.dnn.NMSBoxes(boxes, scores, 0., .35, top_k=4096)
        result = []
        for index in list(np.asarray(indices).reshape(-1))[:max_matches]:
            x, y, w, h = boxes[index]
            result.append(Match(template_id, name, scores[index], x / width, y / height, w / width, h / height, 0, '', '', time.monotonic(), 0))
        return result

    def match_frame(self, frame: Frame, template, image):
        from dataclasses import replace
        started = time.monotonic()
        matches = self.match_image(frame.image, image, template_id=template.id, name=template.name, threshold=template.threshold, grayscale=template.grayscale, scales=template.scales, method=template.method)
        logger.debug('Matched frame: template=%s frame=%s count=%s elapsed_ms=%.1f', template.id, frame.frame_id, len(matches), (time.monotonic() - started) * 1000)
        return [replace(match, frame_id=frame.frame_id, session_id=frame.session_id, device_id=frame.device_id, received_at=frame.received_at, generation=frame.generation, connection_id=frame.connection_id, replayed=frame.replayed) for match in matches]
