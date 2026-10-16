import importlib
from contextlib import redirect_stdout
from importlib.metadata import requires, version
import json
import sys

errors = []
try:
    from packaging.requirements import Requirement

    requirements = json.loads(sys.argv[1])
    if sys.argv[2] == "matching":
        sdk_requirements = requires("mywebscrcpy-sdk")
        if sdk_requirements is None:
            raise RuntimeError("SDK 缺少依赖声明")
        requirements.extend(sdk_requirements)
    for text in requirements:
        requirement = Requirement(text)
        if requirement.marker and not requirement.marker.evaluate({"extra": "vision"}):
            continue
        installed = version(requirement.name)
        if installed not in requirement.specifier:
            errors.append(f"{requirement.name} {installed} 不满足 {requirement.specifier}")
    modules = {
        "matching": ["mywebscrcpy", "httpx", "websocket", "av", "cv2", "numpy"],
        "indraocr": ["indra_ocr", "onnxruntime", "cv2", "numpy", "fastapi", "uvicorn", "multipart"],
        "paddleocr": ["paddle", "paddleocr", "fastapi", "uvicorn", "multipart", "cv2", "numpy"],
    }[sys.argv[2]]
    with redirect_stdout(sys.stderr):
        for name in modules:
            importlib.import_module(name)
        if sys.argv[2] == "matching":
            from mywebscrcpy import Client, Device, Matcher, FrameTimeout
        elif sys.argv[2] == "indraocr":
            from indra_ocr import OrtOCR, model_dir
            if not callable(OrtOCR) or not callable(model_dir):
                errors.append("Indra OCR 接口无效")
        else:
            from paddleocr import PaddleOCR
            if not callable(PaddleOCR.predict):
                errors.append("PaddleOCR 缺少 predict 接口")
except Exception as error:
    errors.append(f"{type(error).__name__}: {error}")
print(json.dumps({"ready": not errors, "error": "；".join(errors)}, ensure_ascii=False))
