#!/usr/bin/env python3
"""Register a task environment and launch it in its dedicated task workspace directory."""

import argparse
import os
from pathlib import Path
import sys
import time
from urllib.parse import urlsplit


def main():
    parser = argparse.ArgumentParser(description="注册环境变量并在任务工作目录中执行任务")
    parser.add_argument("--base-url", required=True, help="已确认的服务根地址")
    parser.add_argument("--serial", required=True, help="已确认的设备 serial")
    parser.add_argument("--task-id", help="可选任务 ID (默认自动按时间戳生成)")
    parser.add_argument("--task-dir", help="可选显式指定任务工作目录")
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

    if args.task_dir:
        task_run_dir = Path(args.task_dir).resolve()
    else:
        root_dir = os.environ.get("MYWEBSCRCPY_TASK_ROOT")
        if root_dir:
            task_root = Path(root_dir).resolve()
        else:
            task_root = (Path.home() / ".mywebscrcppy" / "tasks").resolve()
        task_id = args.task_id.strip() if args.task_id else f"task-{int(time.time())}"
        task_run_dir = (task_root / task_id).resolve()

    task_run_dir.mkdir(parents=True, exist_ok=True)

    task_env = os.environ.copy()
    task_env.update({
        "MYWEBSCRCPY_API_URL": args.base_url.rstrip("/"),
        "MYWEBSCRCPY_BASE_URL": args.base_url.rstrip("/"),
        "MYWEBSCRCPY_SERIAL": args.serial,
        "MYWEBSCRCPY_TASK_DIR": str(task_run_dir),
        "MYWEBSCRCPY_RUN_DIR": str(task_run_dir),
        "MYWEBSCRCPY_TASK_ROOT": str(task_run_dir.parent),
    })
    print(f"[env] task_dir={task_run_dir}", file=sys.stderr, flush=True)
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
