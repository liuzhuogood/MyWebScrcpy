"""Explicit SDK errors; device actions are never automatically retried."""


class SDKError(RuntimeError):
    pass


class ConnectionError(SDKError):
    pass


class ProtocolError(SDKError):
    pass


class HTTPError(SDKError):
    def __init__(self, status: int, code: str, detail: str):
        self.status = status
        self.code = code
        self.detail = detail
        super().__init__(f"HTTP {status}: {code}: {detail}")


class CommandError(SDKError):
    def __init__(self, result):
        self.result = result
        super().__init__(f"ADB command {result.status}, exit_code={result.exit_code}")
