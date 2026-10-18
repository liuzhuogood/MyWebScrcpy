#!/usr/bin/env python3
"""快手极速版主入口：支持指定并驱动各类 **_task.py 业务任务。"""

import argparse
from dataclasses import asdict, dataclass
import importlib
import json
import logging
from pathlib import Path
import sys
import time
from typing import Any

# 将当前目录加入模块搜索路径
CURRENT_DIR = Path(__file__).resolve().parent
if str(CURRENT_DIR) not in sys.path:
    sys.path.insert(0, str(CURRENT_DIR))

from navigate import get_base_url, resolve_device_serial
from mywebscrcpy import Client

logging.basicConfig(
    level=logging.INFO,
    format="[%(asctime)s] [%(levelname)s] %(message)s",
    datefmt="%H:%M:%S",
)
logger = logging.getLogger("kuaishou_main")


@dataclass
class VideoStreamInfo:
    serial: str
    session_id: str
    width: int
    height: int
    first_frame_id: int
    timestamp_ms: int
    synced_to_vision_frame: bool

    def to_dict(self) -> dict[str, Any]:
        return asdict(self)


def start_stream_probe(
    serial: str | None = None,
    base_url: str | None = None,
    timeout: float = 15.0,
    sync_vision_frame: bool = True,
) -> dict[str, Any]:
    """初始化并验证视频流通道就绪状态"""
    base = (base_url or get_base_url()).rstrip("/")
    serial = resolve_device_serial(serial, base)
    started_at = time.monotonic()

    with Client(base) as client:
        with client.startVideoStream(
            serial, timeout=timeout, sync_vision_frame=sync_vision_frame
        ) as stream:
            frame = stream.first_frame
            info = VideoStreamInfo(
                serial=serial,
                session_id=stream.session_id,
                width=frame.width,
                height=frame.height,
                first_frame_id=frame.frame_id,
                timestamp_ms=frame.timestamp,
                synced_to_vision_frame=stream.synced_to_vision_frame,
            )
            return {
                "status": "video_stream_ready",
                "serial": serial,
                "elapsed_seconds": round(time.monotonic() - started_at, 2),
                "step": "acquire_video_stream",
                "video_stream": info.to_dict(),
            }


def discover_available_tasks() -> dict[str, Path]:
    """扫描当前目录下的所有 **_task.py 任务脚本"""
    task_files = list(CURRENT_DIR.glob("*_task.py"))
    mapping: dict[str, Path] = {}
    for tf in task_files:
        name = tf.stem  # 例如 brush_video_task
        short_name = name[:-5] if name.endswith("_task") else name  # 例如 brush_video
        mapping[name] = tf
        mapping[short_name] = tf
        # 兼容中文映射
        if "brush_video" in name:
            mapping["刷视频"] = tf
            mapping["刷视频任务"] = tf
    return mapping


def resolve_task_module(task_name: str) -> Any:
    """根据任务名称解析并导入对应的任务模块"""
    tasks_map = discover_available_tasks()
    target_path = tasks_map.get(task_name) or tasks_map.get(task_name.strip().lower())

    if not target_path:
        # 尝试直接按文件名匹配
        candidate = CURRENT_DIR / f"{task_name}.py"
        candidate_task = CURRENT_DIR / f"{task_name}_task.py"
        if candidate_task.is_file():
            target_path = candidate_task
        elif candidate.is_file():
            target_path = candidate

    if not target_path or not target_path.is_file():
        available = sorted(set(p.stem for p in tasks_map.values()))
        raise ValueError(
            f"未找到指定的任务脚本 '{task_name}'。当前目录下可用任务: {', '.join(available)}"
        )

    module_name = target_path.stem
    return importlib.import_module(module_name)


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument(
        "--task",
        "-t",
        default="brush_video",
        help="指定调用的任务名称（如 brush_video / brush_video_task / 刷视频，默认 brush_video；传 stream 仅测试流就绪）",
    )
    parser.add_argument("--serial", help="设备序列号（若只有1台在线设备或配置了环境变量可省略）")
    parser.add_argument("--base-url", default=None, help="服务端根地址")
    parser.add_argument("--timeout", type=float, default=15.0, help="获取首帧超时秒数")
    parser.add_argument("--no-sync-frame", action="store_true", help="不上传首帧至设备总状态缓存")
    parser.add_argument("--batch-count", type=int, default=5, help="单轮刷视频批次数量 (默认 5)")
    parser.add_argument("--min-stay", type=float, default=5.0, help="单视频最短停留秒数")
    parser.add_argument("--max-stay", type=float, default=15.0, help="单视频最长停留秒数")
    parser.add_argument("--max-cycles", type=int, default=20, help="任务最大轮次上限")
    parser.add_argument("--list-tasks", action="store_true", help="列出当前所有可用的 **_task.py 脚本")
    parser.add_argument("--json", action="store_true", help="以 JSON 格式输出执行结果")
    args = parser.parse_args()

    # 1. 列出可用任务
    if args.list_tasks:
        tasks = sorted(set(p.stem for p in discover_available_tasks().values()))
        if args.json:
            print(json.dumps({"tasks": tasks}, ensure_ascii=False, indent=2))
        else:
            print("当前可用的任务列表:")
            for t in tasks:
                print(f"  - {t}")
        return 0

    base_url = (args.base_url or get_base_url()).rstrip("/")
    try:
        serial = resolve_device_serial(args.serial, base_url)
    except Exception as e:
        if args.json:
            print(json.dumps({"error": str(e), "status": "failed"}, ensure_ascii=False))
        else:
            logger.error(f"设备解析失败: {e}")
        return 1

    # 2. 仅测试视频流连接模式
    if args.task.lower() in ("stream", "probe"):
        try:
            result = start_stream_probe(
                serial=serial,
                base_url=base_url,
                timeout=args.timeout,
                sync_vision_frame=not args.no_sync_frame,
            )
            if args.json:
                print(json.dumps(result, ensure_ascii=False, indent=2))
            else:
                stream = result["video_stream"]
                print("=" * 60)
                print("【快手极速版视频流连接就绪】")
                print(f"设备序列号: {result['serial']}")
                print(f"会话 ID: {stream['session_id']} | 分辨率: {stream['width']}x{stream['height']}")
                print(f"耗时: {result['elapsed_seconds']}s")
                print("=" * 60)
            return 0
        except Exception as error:
            if args.json:
                print(json.dumps({"error": str(error), "status": "failed"}, ensure_ascii=False))
            else:
                logger.error("视频流连接失败: %s", error)
            return 1

    # 3. 指定并调用任务脚本
    try:
        task_module = resolve_task_module(args.task)
        if not hasattr(task_module, "run_task"):
            raise AttributeError(f"任务模块 '{task_module.__name__}' 未暴露 run_task 入口函数")

        logger.info(f"成功载入任务模块: {task_module.__name__}，开始执行...")
        result = task_module.run_task(
            serial=serial,
            base_url=base_url,
            batch_count=args.batch_count,
            min_stay=args.min_stay,
            max_stay=args.max_stay,
            max_cycles=args.max_cycles,
        )

        if args.json:
            print(json.dumps(result, ensure_ascii=False, indent=2))
        else:
            status = result.get("status")
            print("=" * 60)
            print(f"【任务 {args.task} 执行结束】")
            print(f"最终状态: {status}")
            if "claimed_coins" in result:
                print(f"已领金币: {result.get('claimed_coins')}")
            if "current" in result and "target" in result:
                print(f"任务进度: {result.get('current')} / {result.get('target')}")
            print(f"累计观看: {result.get('total_watched', 0)} 条 | 循环轮次: {result.get('cycles', 0)}")
            print(f"总耗时: {result.get('elapsed_seconds', 0)}s")
            print("=" * 60)
        return 0 if result.get("status") == "completed" else 1

    except Exception as error:
        if args.json:
            print(json.dumps({"error": str(error), "status": "failed"}, ensure_ascii=False))
        else:
            logger.error("任务执行失败: %s", error)
        return 1


if __name__ == "__main__":
    if len(sys.argv) <= 1:
        sys.argv = ["main.py", "--serial", "10.0.0.30:5555", "--task", "brush_video"]
    raise SystemExit(main())
