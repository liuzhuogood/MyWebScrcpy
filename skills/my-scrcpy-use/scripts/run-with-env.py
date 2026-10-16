#!/usr/bin/env python3
"""Register a task environment and launch it in its own temporary directory."""

import argparse
import os
from pathlib import Path
import sys
import tempfile
from urllib.parse import urlsplit


def main():
    parser = argparse.ArgumentParser(description="注册环境变量并在独立临时目录执行任务")
    parser.add_argument("--base-url", required=True, help="已确认的服务根地址")
    parser.add_argument("--serial", required=True, help="已确认的设备 serial")
    parser.add_argument("command", nargs=argparse.REMAINDER, help="-- 后的任务命令；脚本路径须为绝对路径")
    args = parser.parse_args()
    if not args.command or args.command[0] != "--" or len(args.command) < 2:
        parser.error("请用 -- 分隔任务命令")
    try:
        task_url = urlsplit(args.base_url)
        task_host = task_url.hostname
        task_url.port
    except ValueError:
        parser.error("服务根地址格式不合法")
    if (task_url.scheme not in {"http", "https"} or not task_host
            or task_url.username is not None or task_url.password is not None
            or task_url.path not in {"", "/"} or task_url.query or task_url.fragment):
        parser.error("服务根地址须为不含凭据、接口路径或查询参数的 HTTP(S) 地址")
    if not args.serial.strip():
        parser.error("设备 serial 不能为空")

    task_env = os.environ.copy()
    task_run_dir = Path(tempfile.mkdtemp(prefix="my-scrcpy-run-")).resolve()
    task_env.update({
        "MYWEBSCRCPY_API_URL": args.base_url.rstrip("/"),
        "MYWEBSCRCPY_SERIAL": args.serial,
        "MYWEBSCRCPY_RUN_DIR": str(task_run_dir),
        "TMPDIR": str(task_run_dir),
        "TEMP": str(task_run_dir),
        "TMP": str(task_run_dir),
    })
    print(f"[env] run_dir={task_run_dir}", file=sys.stderr, flush=True)
    os.chdir(task_run_dir)
    try:
        os.execvpe(args.command[1], args.command[1:], task_env)
    except FileNotFoundError:
        print("[env] 任务命令不存在；执行目录已保留", file=sys.stderr)
        return 127
    except OSError as error:
        print(f"[env] 任务启动失败：{type(error).__name__}；执行目录已保留", file=sys.stderr)
        return 126


if __name__ == "__main__":
    raise SystemExit(main())
