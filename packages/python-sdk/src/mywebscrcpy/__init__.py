"""MyWebScrcpy SDK: vision dependencies are loaded only when needed."""
from .client import Client
from .device import Device
from .matcher import Matcher
from .models import ActionResult, CommandResult, DeviceInfo, Frame, ImportResult, Match, PageInfo, Template
from .ocr import OCRBox, OCRResult
from .reporter import TaskReporter
from .errors import (ActionError, ActionTimeout, CommandError, ConnectionError, FrameTimeout,
                     HTTPError, MatchTimeout, MissingDependency, ProtocolError, SDKError,
                     TemplateDisabled, UnsupportedTemplateMethod)

__version__ = "1.26.1"
__all__ = ["Client", "Device", "Matcher", "TaskReporter", "ActionResult", "CommandResult", "DeviceInfo", "Frame",
           "ImportResult", "Match", "PageInfo", "Template", "OCRBox", "OCRResult",
           "ActionError", "ActionTimeout",
           "CommandError", "ConnectionError", "FrameTimeout", "HTTPError", "MatchTimeout",
           "MissingDependency", "ProtocolError", "SDKError", "TemplateDisabled", "UnsupportedTemplateMethod"]
