from dataclasses import dataclass
from typing import Any


@dataclass(frozen=True)
class CommandResult:
    status: str
    exit_code: int | None
    stdout: str
    stderr: str
    truncated: bool


@dataclass(frozen=True)
class PageInfo:
    width: int
    height: int


@dataclass(frozen=True)
class DeviceInfo:
    serial: str
    state: str
    source: str
    auto_reconnect: bool
    alias: str = ""
    product: str = ""
    model: str = ""
    device: str = ""
    transport_id: str = ""


@dataclass(frozen=True)
class Template:
    id: str
    name: str
    serial: str
    threshold: float
    method: str
    grayscale: bool
    enabled: bool
    width: int
    height: int
    image_count: int
    created_at: str
    updated_at: str
    scales: tuple[float, ...] = (1.0,)
    scene_width: int = 0
    scene_height: int = 0


@dataclass(frozen=True)
class Frame:
    frame_id: int
    session_id: str
    device_id: str
    width: int
    height: int
    timestamp: int
    received_at: float
    image: Any
    generation: int
    connection_id: str = ""
    replayed: bool = False


@dataclass(frozen=True)
class Match:
    template_id: str
    name: str
    confidence: float
    x: float
    y: float
    w: float
    h: float
    frame_id: int
    session_id: str
    device_id: str
    received_at: float
    generation: int
    connection_id: str = ""
    replayed: bool = False

    @property
    def center(self):
        return (self.x + self.w / 2, self.y + self.h / 2)

    def to_object(self):
        return dict(label=self.name, confidence=self.confidence, x=self.x, y=self.y, w=self.w, h=self.h)


@dataclass(frozen=True)
class ActionResult:
    request_id: str
    device_id: str
    session_id: str
    accepted: bool
    executed: bool
    error_code: str


@dataclass(frozen=True)
class ImportResult:
    success: bool
    imported: int
    templates: tuple[Template, ...]
