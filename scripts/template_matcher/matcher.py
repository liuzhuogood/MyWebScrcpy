from __future__ import annotations

import logging
from dataclasses import dataclass
from typing import List, Optional

import cv2
import numpy as np

from .template_loader import TemplateItem

logger = logging.getLogger("template_matcher")


@dataclass
class MatchResult:
    label: str
    confidence: float
    x: float  # 归一化 0.0 ~ 1.0 (left)
    y: float  # 归一化 0.0 ~ 1.0 (top)
    w: float  # 归一化 0.0 ~ 1.0 (width)
    h: float  # 归一化 0.0 ~ 1.0 (height)
    template_id: str = ""

    def to_dict(self) -> dict:
        return {
            "label": self.label,
            "confidence": round(self.confidence, 4),
            "x": round(self.x, 4),
            "y": round(self.y, 4),
            "w": round(self.w, 4),
            "h": round(self.h, 4),
        }


def nms_boxes(boxes: List[dict], overlap_thresh: float = 0.4) -> List[dict]:
    """非极大值抑制（基于像素绝对坐标）"""
    if len(boxes) == 0:
        return []

    boxes = sorted(boxes, key=lambda b: b["score"], reverse=True)
    keep: List[dict] = []

    while boxes:
        current = boxes.pop(0)
        keep.append(current)

        x1_a, y1_a, w_a, h_a = current["x"], current["y"], current["w"], current["h"]
        x2_a = x1_a + w_a
        y2_a = y1_a + h_a
        area_a = w_a * h_a

        remaining = []
        for other in boxes:
            x1_b, y1_b, w_b, h_b = other["x"], other["y"], other["w"], other["h"]
            x2_b = x1_b + w_b
            y2_b = y1_b + h_b
            area_b = w_b * h_b

            xx1 = max(x1_a, x1_b)
            yy1 = max(y1_a, y1_b)
            xx2 = min(x2_a, x2_b)
            yy2 = min(y2_a, y2_b)

            w_inter = max(0, xx2 - xx1)
            h_inter = max(0, yy2 - yy1)
            inter = w_inter * h_inter

            union = area_a + area_b - inter
            iou = inter / union if union > 0 else 0
            if iou < overlap_thresh:
                remaining.append(other)
        boxes = remaining

    return keep


class TemplateMatcher:
    def __init__(self, override_threshold: Optional[float] = None):
        self.override_threshold = override_threshold

    def match_frame(
        self,
        frame_bgr: np.ndarray,
        templates: List[TemplateItem],
        max_matches_per_template: int = 5,
    ) -> List[MatchResult]:
        """对输入的当前帧进行多模板、多尺度匹配，并返回归一化坐标的目标列表"""
        frame_h, frame_w = frame_bgr.shape[:2]
        if frame_h == 0 or frame_w == 0 or not templates:
            return []

        frame_gray: Optional[np.ndarray] = None
        results: List[MatchResult] = []

        for tmpl in templates:
            thresh = self.override_threshold if self.override_threshold is not None else tmpl.threshold
            use_gray = tmpl.grayscale

            if use_gray:
                if frame_gray is None:
                    frame_gray = cv2.cvtColor(frame_bgr, cv2.COLOR_BGR2GRAY)
                source_img = frame_gray
                tpl_base = tmpl.gray_image
            else:
                source_img = frame_bgr
                tpl_base = tmpl.image

            if tpl_base is None:
                continue

            raw_candidate_boxes: List[dict] = []

            for scale in tmpl.scales:
                if scale == 1.0:
                    tpl_scaled = tpl_base
                else:
                    target_w = int(tmpl.width * scale)
                    target_h = int(tmpl.height * scale)
                    if target_w <= 0 or target_h <= 0:
                        continue
                    if target_w > frame_w or target_h > frame_h:
                        continue
                    tpl_scaled = cv2.resize(tpl_base, (target_w, target_h), interpolation=cv2.INTER_LINEAR)

                th, tw = tpl_scaled.shape[:2]
                if th > frame_h or tw > frame_w:
                    continue

                res = cv2.matchTemplate(source_img, tpl_scaled, cv2.TM_CCOEFF_NORMED)
                locs = np.where(res >= thresh)

                for pt_y, pt_x in zip(locs[0], locs[1]):
                    score = float(res[pt_y, pt_x])
                    raw_candidate_boxes.append({
                        "x": int(pt_x),
                        "y": int(pt_y),
                        "w": int(tw),
                        "h": int(th),
                        "score": score,
                    })

            if not raw_candidate_boxes:
                continue

            filtered_boxes = nms_boxes(raw_candidate_boxes, overlap_thresh=0.35)
            filtered_boxes = filtered_boxes[:max_matches_per_template]

            for b in filtered_boxes:
                norm_x = max(0.0, min(1.0, b["x"] / frame_w))
                norm_y = max(0.0, min(1.0, b["y"] / frame_h))
                norm_w = max(0.0, min(1.0 - norm_x, b["w"] / frame_w))
                norm_h = max(0.0, min(1.0 - norm_y, b["h"] / frame_h))

                results.append(
                    MatchResult(
                        label=tmpl.name,
                        confidence=b["score"],
                        x=norm_x,
                        y=norm_y,
                        w=norm_w,
                        h=norm_h,
                        template_id=tmpl.id,
                    )
                )

        return results
