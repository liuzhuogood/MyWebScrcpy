# MyWebScrcpy

基于 Go + WebCodecs 的浏览器端 Android 投屏工具。无需安装客户端，打开浏览器即可投屏和操控 Android 设备。

## 特性

- 纯浏览器端，无需安装任何客户端
- 基于 WebCodecs 硬件解码，低延迟
- 支持 H.264 / H.265 / AV1 编码
- 鼠标操控：点击、拖拽、滚轮、右键返回
- 键盘输入：文本注入、快捷键
- 触摸支持（移动端浏览器）
- 一键旋转屏幕、全屏模式
- 自动重连

## 快速开始

```bash
docker run -d \
  --name mywebscrcpy \
  --privileged \
  -p 8080:8080 \
  -v /dev/bus/usb:/dev/bus/usb \
  liuzhuogood/mywebscrcpy:latest
```

浏览器打开 `https://localhost:8080`（默认启用 HTTPS），点击设备即可投屏。

## 使用网络 ADB

如果你的设备通过 Wi-Fi 连接，使用 host 网络模式：

```bash
docker run -d \
  --name mywebscrcpy \
  --privileged \
  --network host \
  -v /dev/bus/usb:/dev/bus/usb \
  liuzhuogood/mywebscrcpy:latest
```

## 命令行参数

| 参数 | 说明 |
|------|------|
| `-https` | 启用 HTTPS（使用内置自签名证书，默认开启） |

## 环境变量

| 变量 | 说明 | 默认值 |
|------|------|--------|
| `PORT` | 监听端口 | `8080` |

## 标签

| 标签 | 说明 |
|------|------|
| `latest` | 最新稳定版 |
| `v1.0.x` | 指定版本 |

## 架构

```
浏览器 ──WebSocket──▶ Go Server ──ADB Forward──▶ scrcpy-server (设备端)
  │                       │
  │  H.264 视频帧         │  控制消息透传
  │  ◀──────────────────  │  ──────────────▶
  │                       │
WebCodecs 解码          app_process 启动
Canvas 渲染             视频编码 + 控制注入
```

## 相关链接

- [GitHub 仓库](https://github.com/liuzhuogood/MyWebScrcpy)
- [完整文档](https://github.com/liuzhuogood/MyWebScrcpy#readme)

## License

MIT
