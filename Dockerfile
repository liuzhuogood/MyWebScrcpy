FROM debian:bookworm-slim

LABEL maintainer="liuzhuogood"
LABEL description="MyWebScrcpy - Browser-based Android screen mirroring"

# 安装 ADB、基础工具、Python 运行时（模板匹配依赖）
RUN apt-get update && apt-get install -y --no-install-recommends \
    android-tools-adb \
    ca-certificates \
    python3 \
    python3-pip \
    python3-venv \
    # opencv-python wheel 运行所需系统库
    libgl1 \
    libglib2.0-0 \
    && rm -rf /var/lib/apt/lists/*

# 创建 Python 匹配虚拟环境并安装依赖
# 模板匹配由 pymatcher.py 完成：PyAV 解码 H264（含 CABAC）+ OpenCV 模板匹配
RUN python3 -m venv /app/pymatcher_venv \
    && /app/pymatcher_venv/bin/pip install --no-cache-dir \
        av \
        numpy \
        opencv-python \
    && rm -rf /root/.cache/pip

# 创建工作目录
WORKDIR /app

# 复制二进制文件
COPY mywebscrcpy-linux-amd64 /app/mywebscrcpy

# 指定模板匹配进程使用的 Python 解释器（venv 内已含 av/cv2）
ENV MYWEBSCRCPY_PYTHON=/app/pymatcher_venv/bin/python

# 创建数据目录
RUN mkdir -p /data

# 暴露端口
EXPOSE 8080

# 健康检查
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD curl -f http://localhost:8080/api/devices || exit 1

# 启动命令
ENTRYPOINT ["/app/mywebscrcpy"]
CMD ["-https"]
