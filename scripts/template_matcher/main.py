#!/usr/bin/env python3
"""MyWebScrcpy 独立模板匹配工程

通过 HTTP API 自动拉取模板与图片（亦支持本地目录），通过 WebSocket (/api/vision/stream) 订阅 H.264 视频流，
在内存中解码并使用 OpenCV 匹配模板，将识别目标通过 detection.result 回传，在前端播放器上实时显示绿框。
"""

from __future__ import annotations

import argparse
import logging
import signal
import sys
from pathlib import Path

# 确保能正确导入同级包
current_dir = Path(__file__).resolve().parent
if str(current_dir.parent) not in sys.path:
    sys.path.insert(0, str(current_dir.parent))

from template_matcher.client import StreamMatcherClient
from template_matcher.matcher import TemplateMatcher
from template_matcher.template_loader import TemplateLoader


def setup_logger(verbose: bool = False):
    level = logging.DEBUG if verbose else logging.INFO
    logging.basicConfig(
        level=level,
        format="%(asctime)s [%(levelname)s] %(message)s",
        datefmt="%H:%M:%S",
    )


def main():
    parser = argparse.ArgumentParser(description="MyWebScrcpy 独立模板匹配客户端")
    parser.add_argument("--serial", required=True, help="目标设备序列号 (例如: 10.0.0.11:5555 或 usb 序列号)")
    parser.add_argument("--url", default="http://127.0.0.1:8080", help="MyWebScrcpy 服务地址 (默认: http://127.0.0.1:8080)")
    parser.add_argument("--fps", type=int, default=5, help="请求视频降采样帧率 (默认: 5 FPS)")
    parser.add_argument("--threshold", type=float, default=None, help="覆盖模板自身阈值，统一置信度阈值 (0.1 ~ 0.99)")
    parser.add_argument(
        "--from-disk",
        action="store_true",
        help="强制从本地磁盘目录加载模板，而不是通过 HTTP 接口下载",
    )
    parser.add_argument(
        "--template-dir",
        default=str(current_dir.parent.parent / "data" / "templates"),
        help="本地模板根目录路径 (仅在使用 --from-disk 或 API 不可用时生效)",
    )
    parser.add_argument("--global-only", action="store_true", help="仅加载 global 范围内的通用模板")
    parser.add_argument("--insecure", action="store_true", help="连接 https/wss 时跳过 SSL 证书检查")
    parser.add_argument("-v", "--verbose", action="store_true", help="输出更详细的调试日志")

    args = parser.parse_args()
    setup_logger(args.verbose)

    logging.info(
        "初始化模板加载器 (服务: %s, 设备: %s, 模式: %s)",
        args.url,
        args.serial,
        "本地磁盘" if args.from_disk else "HTTP API 在线加载",
    )
    loader = TemplateLoader(
        base_url=args.url,
        serial=args.serial,
        templates_root=args.template_dir,
        from_disk=args.from_disk,
        insecure=args.insecure,
    )
    templates = loader.load_all(global_only=args.global_only)
    if not templates:
        logging.warning("警告: 未找到任何启用的模板！如果服务刚启动，请先在 Web 界面上传或创建模板。")

    matcher = TemplateMatcher(override_threshold=args.threshold)
    client = StreamMatcherClient(
        base_url=args.url,
        serial=args.serial,
        loader=loader,
        matcher=matcher,
        max_fps=args.fps,
        insecure=args.insecure,
    )

    def handle_signal(sig, frame):
        logging.info("收到退出信号，正在关闭匹配客户端...")
        client.stop()
        sys.exit(0)

    signal.signal(signal.SIGINT, handle_signal)
    signal.signal(signal.SIGTERM, handle_signal)

    logging.info("正在启动视频流订阅与实时模板匹配...")
    try:
        client.start()
    except KeyboardInterrupt:
        client.stop()


if __name__ == "__main__":
    main()
