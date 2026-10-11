# MyWebScrcpy 独立模板匹配工程

本工程为 **MyWebScrcpy** 的独立外置模板匹配客户端，基于 **Python + OpenCV + PyAV** 构建。
它通过 HTTP API 自动同步模板列表与图片，通过旁路 WebSocket 通道订阅设备的实时视频流并在内存中解码，执行 OpenCV 多尺度模板匹配，最后将识别目标通过协议上行回传并在前端 Web 播放器上渲染出鲜亮绿框。

---

## 架构优势与设计原则

1. **彻底解耦主服务**：不修改 Go 后端核心逻辑，不占用投屏核心分发线程与 CPU，完全杜绝因复杂图像匹配计算导致的主投屏丢包、坏帧与卡顿。
2. **零磁盘 I/O**：H.264 视频流直接在内存中解码为 BGR ndarray 矩阵，不生成任何临时视频或截图文件。
3. **接口自动同步**：默认直接请求 `GET /api/templates?serial=...` 与 `GET /api/templates/{id}/image` 接口，**无需客户端本地拥有模板文件**，在任意机器部署即可自动与服务端模板库同步，支持 Web 端增删改后的自动热更新。
4. **异步丢帧防延迟**：借鉴 `yolo_wepork` 的 `LatestFrames` 异步无锁单容量队列机制，当匹配耗时时自动丢弃积压帧，保证检测永远针对最新画面。
5. **前端闭环渲染**：通过 `/api/vision/results` 广播通道，前端播放器的 Overlay Canvas 自动接收并绘制目标高亮绿框与置信度标签。

---

## 环境准备与依赖安装

推荐使用 Python 3.10+ 环境。

```bash
cd scripts/template_matcher
pip install -r requirements.txt
```

---

## 使用指南

### 1. 快速启动 (默认在线 API 模式)

```bash
# 连接本地服务与目标设备 (以 10.0.0.11:5555 为例，模板自动从服务下载到内存)
python main.py --serial 10.0.0.11:5555
```

### 2. 常用命令行选项

| 参数 | 默认值 | 说明 |
| :--- | :--- | :--- |
| `--serial` | **必填** | 目标 Android 设备序列号 (如 `10.0.0.11:5555`) |
| `--url` | `http://127.0.0.1:8080` | MyWebScrcpy 服务地址 (支持 `http://` / `ws://` / `https://` / `wss://`) |
| `--fps` | `5` | 视频流降采样帧率 (建议 3~10 FPS，平衡 CPU 与实时性) |
| `--threshold` | `None` | 全局覆盖阈值 (例如 `0.85`；若不传则遵循各模板元数据设置) |
| `--from-disk` | `False` | 切换为本地磁盘模式 (从 `--template-dir` 离线扫描) |
| `--template-dir` | `../../data/templates` | 本地模板资产存放目录 (仅 `--from-disk` 或接口离线回退时使用) |
| `--global-only` | `False` | 仅加载 `global` 作用域下的公共模板 |
| `--insecure` | `False` | 当连接自签名证书服务时跳过 SSL 证书校验 |
| `-v, --verbose` | `False` | 打印更详细的收流与帧匹配日志 |

### 3. 连接远程服务端示例

```bash
python main.py \
  --url https://10.0.0.6:8080 \
  --serial 10.0.0.30:5555 \
  --insecure \
  --fps 5
```

---

## 界面绿框联动

1. 启动本程序后，打开 Web 浏览器访问 MyWebScrcpy 投屏页面 (`/player?serial=...`)。
2. 确保右侧设置面板中的 **「显示匹配绿框」** 处于开启状态。
3. 一旦手机画面中出现模板匹配目标，投屏区域将实时浮现带微透明背景与高亮绿边框（`#00e676`）的目标定位框，并显示模板名称与匹配置信度。
