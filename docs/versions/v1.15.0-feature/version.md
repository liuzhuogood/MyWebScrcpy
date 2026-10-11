# v1.15.0-feature 版本说明：独立模板匹配工程与实时绿框展示

更新日期：2026-10-11。

**状态：已实现并完成 9222 端口 Chrome CDP 浏览器实机自动化验收。**

## 版本定位与背景

在 v1.11.0 架构重构中，为确保核心投屏低延迟与高稳定性，彻底移除了 Go 服务内嵌的 Python/OpenCV Worker 实时匹配进程，服务端仅保留模板资产管理（CRUD、图片预览、ZIP 导出导入），并将投屏视频链路与算法解耦。
同时，Go 服务沉淀出了稳定可靠的旁路 WebSocket 通道：
- `/api/vision/stream?serial={serial}`：外部程序可订阅 H.264 视频流（支持 Annex-B/AVCC，协商 FPS），并上行回传目标识别框；
- `/api/vision/results?serial={serial}`：服务端将识别结果作为 `detection.result` 实时广播给浏览器。

本版本在当前项目下建立了**完全独立解耦的模板匹配子工程**（基于 Python + OpenCV + PyAV），并在前端 Web 播放器打通了实时绿框高亮渲染闭环。

## 核心交付内容

1. **独立模板匹配工程 (`scripts/template_matcher/`)**：
   - 纯外置 Python 客户端，通过 WebSocket 接入 `/api/vision/stream`。
   - 默认通过 HTTP API（`GET /api/templates` 与 `GET /api/templates/{id}/image`）动态下载模板与图片到内存，无需客户端本地保存模板文件，支持 Web 端增删改热更新；保留 `--from-disk` 离线兜底。
   - 借鉴 `yolo_wepork` 的 `LatestFrames` 异步无锁单容量队列机制，当匹配耗时时自动丢弃积压帧，保证检测永远针对最新画面。
   - 使用 PyAV 在内存中解码 H.264 视频帧为 BGR ndarray，绝不将视频帧落地写入磁盘，零 I/O 开销。
   - 执行 OpenCV 多尺度模板匹配（`TM_CCOEFF_NORMED` + NMS 非极大值抑制）。
   - 将匹配命中的目标以 0~1 归一化坐标封装为 `detection.result` 消息上行回传服务端。
   - 提供完备的 CLI 启动参数（`--serial`, `--url`, `--fps`, `--threshold`, `--from-disk`, `--template-dir`, `--global-only`, `--insecure`）与使用说明。

2. **前端播放器绿框渲染闭环 (`frontend/src/template/useTemplateMatches.ts`)**：
   - 连接 `/api/vision/results?serial={serial}` WebSocket 广播通道，支持断线自动重连。
   - 接收 `detection.result` 事件并在播放器 Overlay Canvas 上实时绘制高亮绿框（`#00e676`）与名称置信度标签。
   - 深度联动“显示匹配绿框”开关、窗口缩放 resize 自适应与 `expires_ms` 定时清框防残影机制。

## 验收结论与证据

- **HTTP 接口模板拉取**：成功通过 `GET /api/templates` 和 `GET /api/templates/{id}/image` 在线流式下载并解码模板。
- **旁路收流**：成功连接真机 `10.0.0.30:5555`，`subscribers=1`，视频流帧数持续递增且服务端 `dropped_frames=0`。
- **CDP 实机渲染验收**：
  - 在已连接 9222 端口 Chrome 中打开实机投屏页面，Overlay Canvas 尺寸与视频流自动同步为 448 × 1024；
  - 下发检测结果后，Canvas 像素分析检测到 2008 个 `#00e676` 亮绿像素，目标边框与文字渲染正常；
  - 5 秒后超时清框逻辑生效，绿框像素归零，防残影机制完全符合预期。
