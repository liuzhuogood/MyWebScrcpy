from dataclasses import dataclass


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
