from __future__ import annotations

import contextlib
import io
import tempfile
import json
import logging
import os
import sys
import threading
import time
import uuid
from typing import Any

try:
    import httpx
except ImportError as error:
    raise ImportError("请在当前 Python 环境安装 mywebscrcpy-sdk，或安装 httpx") from error

from .errors import HTTPError, ProtocolError, SDKError

logger = logging.getLogger(__name__)

MAX_LOG_LINE_CHARS = 32 * 1024  # 单条日志字符上限 32KB
MAX_LOG_BUFFER_CHARS = 64 * 1024  # 捕获流缓冲区单行上限 64KB


class TaskReporter:
    """外部自动化任务运行上报客户端。

    支持向 MyWebScrcpy 上报任务生命周期、心跳保活、实时日志推送与最终结果。
    """

    def __init__(
        self,
        base_url: str,
        *,
        name: str,
        device_serial: str,
        client_run_id: str | None = None,
        source: str = "python-sdk",
        heartbeat_interval: float = 10.0,
        max_buffer_size: int = 5000,
        timeout: float = 15.0,
        verify: bool | str = True,
    ):
        if not base_url or not base_url.startswith(("http://", "https://")):
            raise ValueError("base_url 必须是以 http:// 或 https:// 开头的合法服务地址")
        if not name:
            raise ValueError("任务名称 name 不能为空")
        if not device_serial:
            raise ValueError("设备序列号 device_serial 不能为空")

        if heartbeat_interval <= 0 or max_buffer_size < 1:
            raise ValueError("heartbeat_interval 与 max_buffer_size 必须为正数")
        self.base_url = base_url.rstrip("/")
        self.name = name
        self.device_serial = device_serial
        self.client_run_id = client_run_id or f"client_{uuid.uuid4().hex}"
        self.source = source
        self.heartbeat_interval = heartbeat_interval
        self.max_buffer_size = max_buffer_size
        self.timeout = timeout
        self.verify = verify

        self._http = httpx.Client(
            base_url=self.base_url,
            headers={
                "Content-Type": "application/json",
            },
            timeout=self.timeout,
            verify=self.verify,
            trust_env=False,
        )

        self._lock = threading.Lock()
        self._upload_lock = threading.Lock()
        self._seq = 0
        self._acknowledged_seq = 0
        self._pending_entries: list[dict[str, Any]] = []
        self._run_id: str | None = None
        self._status = "pending"
        self._finished = False

        self._background_thread: threading.Thread | None = None
        self._stop_background = threading.Event()

    @property
    def run_id(self) -> str | None:
        return self._run_id

    @property
    def acknowledged_seq(self) -> int:
        with self._lock:
            return self._acknowledged_seq

    @property
    def status(self) -> str:
        with self._lock:
            return self._status

    def start(self) -> str:
        """创建任务运行并开启后台心跳与批量日志推送。"""
        with self._lock:
            if self._run_id is not None:
                return self._run_id

        payload = {
            "client_run_id": self.client_run_id,
            "name": self.name,
            "source": self.source,
            "device_serial": self.device_serial,
        }

        resp = self._request("POST", "/api/task-runs", json=payload)
        data = resp.json()
        if not isinstance(data, dict) or "id" not in data or not isinstance(data["id"], str):
            raise ProtocolError(f"创建运行未返回合法 id: {data}")

        run_id = data["id"]
        ack = data.get("acknowledged_seq")
        ack_int = ack if isinstance(ack, int) else 0

        with self._lock:
            self._run_id = run_id
            self._status = "running"
            self._acknowledged_seq = ack_int

        # 启动后台心跳与日志上传线程
        self._stop_background.clear()
        self._background_thread = threading.Thread(
            target=self._background_loop,
            name=f"task-reporter-bg-{run_id}",
            daemon=True,
        )
        self._background_thread.start()
        return run_id

    def _background_loop(self):
        while not self._stop_background.wait(self.heartbeat_interval):
            # 1. 定期刷新待传日志
            try:
                self.flush(max_retries=1)
            except Exception as e:
                logger.debug("后台上传日志暂未成功: %s", e)

            # 2. 发送保活心跳
            try:
                self.heartbeat()
            except Exception as e:
                logger.warning("任务 %s 心跳发送失败: %s", self._run_id, e)

    def heartbeat(self):
        """发送单次保活心跳。"""
        run_id = self._run_id
        if not run_id:
            return
        self._request("POST", f"/api/task-runs/{run_id}/heartbeat")

    def log(self, message: str, stream: str = "stdout"):
        """追加一条日志到本地缓冲。"""
        if stream not in ("stdout", "stderr"):
            raise ValueError("stream 只能为 stdout 或 stderr")
        msg_str = str(message)
        if len(msg_str) > MAX_LOG_LINE_CHARS:
            msg_str = msg_str[:MAX_LOG_LINE_CHARS] + " ...[truncated]"

        with self._lock:
            if self._finished:
                raise SDKError("已结束的运行不能继续写日志")
            if len(self._pending_entries) >= self.max_buffer_size:
                raise SDKError("任务日志缓冲已满，请先 flush 后重试")
            self._seq += 1
            entry = {
                "seq": self._seq,
                "timestamp": int(time.time() * 1000),
                "stream": stream,
                "message": msg_str,
            }
            self._pending_entries.append(entry)

    def flush(self, max_retries: int = 3):
        """有界分批上传，严格校验服务端连续确认，后台和手动上传串行。"""
        if max_retries < 1:
            raise ValueError("max_retries 必须大于 0")
        if not self._run_id:
            return
        with self._upload_lock:
            with self._lock:
                pending = list(self._pending_entries)
            while pending:
                batch = []
                size = 32
                for entry in pending:
                    entry_size = len(json.dumps(entry, ensure_ascii=True).encode("utf-8")) + 2
                    if batch and size + entry_size > 512 * 1024:
                        break
                    batch.append(entry)
                    size += entry_size
                for attempt in range(max_retries):
                    try:
                        resp = self._request("POST", f"/api/task-runs/{self._run_id}/logs", json={"entries": batch})
                        data = resp.json()
                        if not isinstance(data, dict) or type(data.get("acknowledged_seq")) is not int:
                            raise ProtocolError("服务端缺少合法 acknowledged_seq")
                        ack_seq = data["acknowledged_seq"]
                        if ack_seq != batch[-1]["seq"]:
                            raise ProtocolError("服务端未确认本批全部日志")
                        with self._lock:
                            self._acknowledged_seq = ack_seq
                            self._pending_entries = [e for e in self._pending_entries if e["seq"] > ack_seq]
                        break
                    except Exception as err:
                        if attempt + 1 == max_retries:
                            raise SDKError(f"日志上传失败: {err}") from err
                        time.sleep(0.05 * (attempt + 1))
                pending = pending[len(batch):]

    def finish(
        self,
        status: str,
        exit_code: int | None = None,
        summary: str = "",
        report_markdown: str = "",
        max_retries: int = 3,
    ):
        """提交任务结束状态与报告。先推送待传日志，日志全部确认后才提交终态。"""
        if status not in ("succeeded", "failed", "cancelled"):
            raise ValueError("status 必须是 succeeded、failed 或 cancelled")
        if max_retries < 1:
            raise ValueError("max_retries 必须大于 0")
        with self._lock:
            if self._finished:
                return

        # 1. 停止心跳后台线程
        self._stop_background.set()
        if self._background_thread and self._background_thread.is_alive():
            self._background_thread.join(timeout=1.0)

        run_id = self._run_id
        if not run_id:
            return

        # 2. 先 flush 日志。如果 flush 失败，不得静默提交终态！
        try:
            self.flush(max_retries=max_retries)
        except Exception as e:
            # 记录失败证据并直接抛出，保留重试能力，不掩盖日志丢失
            self._save_failure_evidence(
                {
                    "stage": "flush_before_finish",
                    "status": status,
                    "exit_code": exit_code,
                    "summary": summary,
                    "report_markdown": report_markdown,
                },
                e,
            )
            raise SDKError(f"提交终态前日志上传失败: {e}") from e

        # 3. 提交终态
        payload: dict[str, Any] = {
            "status": status,
            "summary": summary,
            "report_markdown": report_markdown,
        }
        if exit_code is not None:
            payload["exit_code"] = int(exit_code)

        last_error: Exception | None = None
        for attempt in range(max_retries):
            try:
                resp = self._request("POST", f"/api/task-runs/{run_id}/finish", json=payload)
                data = resp.json()
                if not isinstance(data, dict) or data.get("status") != status:
                    raise ProtocolError("服务端未确认请求的最终状态")
                with self._lock:
                    self._status = data["status"]
                    self._finished = True  # 只有在服务端返回合法确认后才完成本地终态标记
                return
            except Exception as err:
                last_error = err
                if attempt + 1 < max_retries:
                    time.sleep(0.05 * (attempt + 1))

        # 上传失败时在本地留下明确的未提交记录证据
        self._save_failure_evidence(payload, last_error)
        raise SDKError(f"提交任务终态失败: {last_error}") from last_error

    def _save_failure_evidence(self, payload: dict[str, Any], error: Exception | None):
        """本地持久化失败证据，避免静默失败，严格限制文件权限且不泄密。"""
        with self._lock:
            # 保存有限容量真实待传事件，防止无界占用
            pending_snapshot = list(self._pending_entries)

        evidence = {
            "run_id": self._run_id,
            "client_run_id": self.client_run_id,
            "failed_at": time.time(),
            "payload": payload,
            "error": str(error),
            "unconfirmed_logs_count": len(self._pending_entries),
            "unconfirmed_logs": pending_snapshot,
        }
        # 安全构造文件名，避免外部路径穿越
        safe_id = "".join(c for c in (self._run_id or "unknown") if c.isalnum() or c in "-_")
        try:
            fd, filename = tempfile.mkstemp(prefix=".task-run-error-", suffix=".json", dir=os.getcwd())
            with os.fdopen(fd, "w", encoding="utf-8") as f:
                json.dump(evidence, f, ensure_ascii=False, indent=2)
            logger.error("已在本地保存未提交终态证据: %s", filename)
        except Exception as write_err:
            logger.error("写入本地失败证据文件失败: %s", write_err)

    @contextlib.contextmanager
    def capture_output(self):
        """显式上下文管理器：捕获 stdout 和 stderr 并实时记录为日志。"""
        original_stdout = sys.stdout
        original_stderr = sys.stderr

        class _TeeStream(io.TextIOBase):
            def __init__(self, target, reporter_fn, stream_name):
                self.target = target
                self.reporter_fn = reporter_fn
                self.stream_name = stream_name
                self._buffer = ""

            def write(self, s: str) -> int:
                self.target.write(s)
                self.target.flush()
                self._buffer += s
                while "\n" in self._buffer:
                    line, self._buffer = self._buffer.split("\n", 1)
                    if line:
                        self.reporter_fn(line, stream=self.stream_name)
                # 针对无换行大文本防止无界内存增长
                while len(self._buffer) > MAX_LOG_BUFFER_CHARS:
                    flush_chunk = self._buffer[:MAX_LOG_BUFFER_CHARS]
                    self._buffer = self._buffer[MAX_LOG_BUFFER_CHARS:]
                    self.reporter_fn(flush_chunk, stream=self.stream_name)
                return len(s)

            def flush(self):
                if self._buffer:
                    self.reporter_fn(self._buffer, stream=self.stream_name)
                    self._buffer = ""
                self.target.flush()

        tee_stdout = _TeeStream(original_stdout, self.log, "stdout")
        tee_stderr = _TeeStream(original_stderr, self.log, "stderr")
        sys.stdout = tee_stdout  # type: ignore
        sys.stderr = tee_stderr  # type: ignore

        try:
            yield
        finally:
            try:
                tee_stdout.flush()
                tee_stderr.flush()
            finally:
                sys.stdout = original_stdout
                sys.stderr = original_stderr

    def _request(self, method: str, path: str, json: Any = None) -> httpx.Response:
        try:
            resp = self._http.request(method, path, json=json)
        except httpx.RequestError as error:
            raise SDKError(f"HTTP 请求失败 ({method} {path}): {error}") from error

        if resp.is_error:
            try:
                body = resp.json()
                if isinstance(body, dict):
                    if "error" in body and isinstance(body["error"], str):
                        msg = body["error"]
                    else:
                        msg = resp.text
                else:
                    msg = resp.text
            except Exception:
                msg = resp.text
            raise HTTPError(resp.status_code, str(resp.status_code), str(msg))
        return resp

    def close(self):
        self._stop_background.set()
        if self._background_thread and self._background_thread.is_alive():
            self._background_thread.join(timeout=1.0)
        self._http.close()

    def __enter__(self):
        self.start()
        return self

    def __exit__(self, exc_type, exc_val, exc_tb):
        try:
            if exc_type is not None:
                try:
                    self.finish(status="failed", exit_code=1, summary=f"异常中断: {exc_val}")
                except Exception as reporting_error:
                    logger.error("异常结果尚未上传: %s", reporting_error)
            elif not self._finished:
                self.finish(status="succeeded", exit_code=0)
        finally:
            self.close()
