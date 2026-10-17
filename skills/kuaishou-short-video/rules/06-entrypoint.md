# 入口实现源

```python path=scripts/main.py
import argparse
import json
import os
from pathlib import Path
import time
import traceback
import httpx
from mywebscrcpy import Client, TaskReporter
from lib.flow import Flow
from lib.runtime import Runtime

def main():
    parser = argparse.ArgumentParser(description="快手极速版任务与观看模式")
    parser.add_argument("--mode", choices=("tasks", "watch"), default="tasks")
    parser.add_argument("--watch-seconds", type=float, default=120)
    parser.add_argument("--ocr-engine", choices=("macocr", "indraocr", "paddleocr", "auto"), default="macocr")
    args = parser.parse_args()
    if not 0 < args.watch_seconds <= 14400:
        parser.error("观看时长必须在(0,14400]秒之间")
    base, serial = os.environ["MYWEBSCRCPY_API_URL"], os.environ["MYWEBSCRCPY_SERIAL"]
    root = Path(os.environ["MYWEBSCRCPY_RUN_DIR"]).resolve()
    if not root.is_dir():
        raise RuntimeError("执行目录不存在，请通过run-with-env.py运行")
    with httpx.Client(base_url=base, timeout=10, trust_env=False) as http, Client(base) as client:
        response = http.get("/api/openapi.json")
        response.raise_for_status()
        schema = response.json()
        (root/"openapi.json").write_text(json.dumps(schema, ensure_ascii=False))
        if not any(d.serial == serial and d.state == "device" for d in client.devices.list()):
            raise RuntimeError("指定设备未连接")
        with client.device(serial) as device, TaskReporter(
                base, name="快手极速版"+("观看模式验证" if args.mode == "watch" else "短视频任务"),
                device_serial=serial, source="codex") as report, report.capture_output():
            runtime = Runtime(device, http, serial, root, args.ocr_engine,
                              total_timeout=max(480, args.watch_seconds+240))
            outcome, failure, cancelled = None, None, False
            try:
                recording_ms = max(600000, round((args.watch_seconds+300)*1000))
                runtime.start_recording(recording_ms)
                runtime.prepare(schema)
                flow = Flow(runtime)
                if args.mode == "watch":
                    flow.wait(lambda v: True, 15)
                    runtime.navigation("首页", time.monotonic()+20)
                    outcome = flow.watch(args.watch_seconds)
                else:
                    outcome = flow.tasks(args.watch_seconds)
            except BaseException as error:
                failure, cancelled = error, isinstance(error, KeyboardInterrupt)
                traceback.print_exc()
                try:
                    runtime.until = time.monotonic()+10
                    runtime.read(runtime.until)
                except Exception:
                    traceback.print_exc()
            finally:
                try:
                    runtime.stop_recording()
                except Exception as error:
                    traceback.print_exc()
                    if failure is None:
                        failure = error
            status = "cancelled" if cancelled else ("failed" if failure is not None else "succeeded")
            result = {"status": status, "outcome": outcome,
                      "error_type": type(failure).__name__ if failure is not None else None}
            (root/"result.json").write_text(json.dumps(result, ensure_ascii=False, indent=2))
            print(json.dumps(result, ensure_ascii=False), flush=True)
            summary = ("观看指定时长并返回赚钱页；平台签到状态另行核对。" if args.mode == "watch"
                       else "任务流程完成，详见步骤结果。")
            if failure is not None:
                summary = "执行未完成，已保留异常与现场证据。"
            report.finish(status, exit_code=1 if failure is not None else 0, summary=summary,
                          report_markdown="结果："+json.dumps(result, ensure_ascii=False)[:170])
            return 1 if failure is not None else 0

if __name__ == "__main__":
    raise SystemExit(main())
```
