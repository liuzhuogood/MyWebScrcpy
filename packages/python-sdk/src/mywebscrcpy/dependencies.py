"""Vision imports stay lazy: base import and template CRUD need no decoder."""
from importlib import import_module

from .errors import MissingDependency


def vision_dependencies():
    try:
        return import_module("av"), import_module("cv2"), import_module("numpy")
    except ImportError as error:
        raise MissingDependency('请安装识图依赖：python -m pip install "mywebscrcpy-sdk[vision]"；本地安装使用 "./packages/python-sdk[vision]"') from error
