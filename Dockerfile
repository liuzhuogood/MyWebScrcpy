FROM debian:bookworm-slim

LABEL maintainer="liuzhuogood"
LABEL description="MyWebScrcpy - Browser-based Android screen mirroring"

# 安装 ADB 和基础工具
RUN apt-get update && apt-get install -y --no-install-recommends \
    android-tools-adb \
    ca-certificates \
    && rm -rf /var/lib/apt/lists/*

# 创建工作目录
WORKDIR /app

# 复制二进制文件
COPY mywebscrcpy-linux-amd64 /app/mywebscrcpy

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
