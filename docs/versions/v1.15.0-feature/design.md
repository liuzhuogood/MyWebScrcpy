# v1.15.0-feature 技术设计：独立模板匹配工程与实时绿框展示

## 1. 架构定位与设计哲学

在此前的版本演进中，将实时图像匹配强行嵌入 Go 服务进程导致了以下核心问题：
1. **CPU 与调度竞争**：高分辨率、高频的视频软解与图像矩阵运算极大消耗服务器 CPU，导致主投屏 WebSocket 堆积丢包、产生花屏与坏帧。
2. **进程稳定性耦合**：CGO/PyAV 依赖可能引发不可控的 Crash 或内存泄露。

因此，自 v1.11.0 起确立了**微内核 + 旁路扩展**的解耦原则：
- **Go 核心服务**：保持轻量高效，专注于 Android 设备 ADB 连接、投屏解复用、基础指令透传与 WebSocket 分发。
- **算法/匹配服务**：作为**完全解耦的独立客户端**运行在宿主机或外部容器中。通过标准旁路 WebSocket 消费视频流，并回传识别元数据。

```mermaid
graph LR
    subgraph Device [Android 设备]
        Scrcpy[scrcpy-server]
    end

    subgraph GoServer [MyWebScrcpy Go 服务]
        Hub[Hub 共享 Session]
        VisionStream["/api/vision/stream (H.264 推流)"]
        VisionResults["/api/vision/results (结果广播)"]
    end

    subgraph ExtProcess [独立模板匹配工程 (Python)]
        Decoder[PyAV 内存解码器]
        Matcher[OpenCV 多尺度匹配器]
    end

    subgraph Browser [Web 前端]
        Player[PlayerView 投屏]
        Overlay[Canvas 绿框图层]
    end

    Scrcpy -->|视频流| Hub
    Hub -->|投屏数据| Player
    Hub -->|H.264 NAL| VisionStream
    VisionStream -->|WebSocket| Decoder
    Decoder --> Matcher
    Matcher -->|detection.result| VisionStream
    VisionStream -->|广播| VisionResults
    VisionResults -->|WebSocket| Overlay
```

---

## 2. 外部匹配工程设计 (`scripts/template_matcher/`)

### 2.1 模块划分与职责

1. **`main.py`**：
   - 负责命令行参数解析与上下文初始化。
   - 参数列表：
     - `--url`：服务地址，默认 `ws://127.0.0.1:8080`（支持 `wss://`）。
     - `--serial`：目标设备序列号（必须或默认第一个可用设备）。
     - `--fps`：请求降采样帧率，默认 `5`（0 表示全速）。
     - `--threshold`：默认匹配置信度阈值，默认 `0.8`。
     - `--template-dir`：模板根目录，默认项目下的 `data/templates/`。
     - `--global-only`：仅加载全局模板。
     - `--insecure`：忽略 HTTPS/WSS 证书校验。
2. **`template_loader.py`**：
   - 扫描 `data/templates/{serial}/` 与 `data/templates/global/` 目录。
   - 读取各子目录下的 `meta.json` 或 `target.json`（名称、阈值、灰度标记、多尺度列表）。
   - 加载 `template.png`（或多图变体）为 OpenCV 图像矩阵（BGR / Grayscale）。
   - 暴露内存模板缓存与定时热重载接口。
3. **`matcher.py`**：
   - 核心匹配引擎：使用 `cv2.matchTemplate`（`TM_CCOEFF_NORMED`）。
   - 支持多尺度缩放（根据模板配置中的 `scales: [0.9, 1.0, 1.1]` 等缩放模板或画面）。
   - 阈值过滤与非极大值抑制（NMS），防止单个目标重复触发多个密集框。
   - 输出统一格式的目标集合：`label`, `confidence`, `x`, `y`, `w`, `h`（**必须转换为 0.0 ~ 1.0 的归一化浮点坐标**）。
4. **`client.py`**：
   - 封装 WebSocket 连接管理与重连机制。
   - 发送 `hello` 握手报文（协商 `max_fps`）。
   - 维持每 5 秒心跳报文（`heartbeat`）。
   - 接收二进制视频帧：
     - 长度为 9 字节头（1 字节 kind + 8 字节 PTS）+ 载荷 payload。
     - 处理 `kind=0`（SPS/PPS 配置帧，AVCC 转 Annex-B）。
     - 使用 `av.CodecContext.create('h264', 'r')` 与 `parse()` 进行帧解析与解码。
   - 图像解码成功后调用 `matcher.match(image)`。
   - 命中后组织报文并通过原连接发送：
     ```json
     {
       "type": "detection.result",
       "device_id": "10.0.0.30:5555",
       "frame_id": 123,
       "timestamp": 1788751244800,
       "expires_ms": 600,
       "objects": [
         {
           "label": "确认按钮",
           "confidence": 0.95,
           "x": 0.61,
           "y": 0.72,
           "w": 0.18,
           "h": 0.07
         }
       ]
     }
     ```

---

## 3. 前端实时绿框渲染设计 (`frontend/`)

### 3.1 状态与图层管理 (`useTemplateMatches.ts`)

- **DOM 挂载**：`PlayerView.vue` 中已存在与投屏 Canvas 绝对定位贴合的 `tm.overlayRef` Canvas。
- **连接通道**：
  - 前端建立 WebSocket 连接：
    `${proto}//${location.host}/api/vision/results?serial=${encodeURIComponent(serial)}`
  - 监听 `onmessage` 消息，筛选 `msg.type === 'detection.result'`。
- **绘制逻辑**：
  1. 读取投屏 Canvas 的实际逻辑尺寸（`canvas.width`, `canvas.height`），同步 `overlay.width` 和 `overlay.height`。
  2. 收到匹配结果后，清空画布，根据归一化坐标换算像素坐标：
     - $X = \text{obj.x} \times \text{width}$
     - $Y = \text{obj.y} \times \text{height}$
     - $W = \text{obj.w} \times \text{width}$
     - $H = \text{obj.h} \times \text{height}$
  3. 绘制亮绿色描边框：`strokeStyle = '#00e676'`，线宽 `max(2, width / 500)`。
  4. 绘制标签背景微黑半透明框 + 亮绿文字：`[label] [confidence%]`，字号 `max(12, width / 60)px`。
- **清除策略与防残影**：
  - 若收到的 `objects` 为空列表，立即执行 `clearRect`。
  - 若消息带 `expires_ms`，设置定时器在超时后自动清除画布，防止目标移开后绿框残留。
  - 响应设置面板中的 `showBoxes` 开关，关闭时立即清除并暂停绘制。
