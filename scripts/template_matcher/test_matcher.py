from pathlib import Path
import numpy as np
import cv2

from template_matcher.template_loader import TemplateLoader, TemplateItem
from template_matcher.matcher import TemplateMatcher


def test_loader_disk():
    templates_dir = Path(__file__).resolve().parent.parent.parent / "data" / "templates"
    loader = TemplateLoader(templates_root=templates_dir, serial="10.0.0.11:5555", from_disk=True)
    items = loader.load_all()
    assert len(items) >= 2, f"应该至少加载到 global 或 10.0.0.11 的模板，实际: {len(items)}"
    print(f"✅ Loader 磁盘模式测试通过: 成功载入 {len(items)} 个模板: {[i.name for i in items]}")


def test_matcher_synthetic():
    h, w = 400, 300
    canvas = np.zeros((h, w, 3), dtype=np.uint8)

    patch = np.ones((40, 40, 3), dtype=np.uint8) * 200
    patch[15:25, 15:25] = 0

    canvas[150:190, 100:140] = patch

    tmpl = TemplateItem(
        id="test_tmpl",
        name="测试方块",
        serial="global",
        threshold=0.8,
        grayscale=True,
        scales=[1.0],
        image=patch,
        gray_image=cv2.cvtColor(patch, cv2.COLOR_BGR2GRAY),
        width=40,
        height=40,
    )

    matcher = TemplateMatcher()
    results = matcher.match_frame(canvas, [tmpl])

    assert len(results) == 1, f"应当精准命中 1 个目标，实际: {len(results)}"
    res = results[0]
    assert res.label == "测试方块"
    assert res.confidence >= 0.99
    expected_x = 100 / w
    expected_y = 150 / h
    assert abs(res.x - expected_x) < 0.01
    assert abs(res.y - expected_y) < 0.01
    print(f"✅ Matcher 逻辑测试通过: 坐标与置信度完全匹配: {res.to_dict()}")


if __name__ == "__main__":
    test_loader_disk()
    test_matcher_synthetic()
