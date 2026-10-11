# v1.15.0-feature 会话记录

## 记录原则

- 用户原话完整记录。
- 实际工具回复精简记录，只保留结论、承诺、关键问题和关键决策。
- 每次用户对话都要追加记录。

## 会话

### 1. 用户

> 增加模板匹配模板工程，在当前项目下， 模板工程启动后显示绿框， 设计一下这个匹配模板工程

### 1. OpenCode

- 分析当前系统架构：回顾 v1.11.0 移除 Go 内嵌 Python/OpenCV Worker 的根本原因（避免视频流解码与计算阻塞主投屏导致丢包花屏），明确独立外置工程的设计原则。
- 梳理现有可用旁路通道：`/api/vision/stream`（提供二进制 H.264 视频流订阅与结果上行）及 `/api/vision/results`（向前端广播 `detection.result`）。
- 梳理前端渲染点：`PlayerView.vue` 内已有预留的 `tm.overlayRef` 与 `useTemplateMatches.ts`。
- 输出独立模板匹配工程（`scripts/template_matcher/`）的设计方案、时序图与前端 Canvas 绿框承载逻辑。

### 2. 用户

> 改成 V1.15版本吧，

### 2. OpenCode

- 将方案版本定位调整为 `v1.15.0-feature`（独立模板匹配工程与实时绿框展示）。
- 梳理具体的任务执行阶段：版本文档初始化、前端绿框联动打通、独立模板匹配子工程实现、联调验证。

### 3. 用户

> 1 $lz-vlog 使用版本工作流来完成这个任务，去生成版本四件套

### 3. OpenCode

- 一句话复述理解：按版本工作流创建 `v1.15.0-feature` 版本四件套文档并推进独立模板匹配工程与前端绿框联动设计。
- 创建 `docs/versions/v1.15.0-feature/` 四件套文档（`version.md`, `design.md`, `tasks.md`, `session.md`）。
- 同步更新总览文档 `docs/versions/README.md`。

### 4. 用户

> 1

### 4. OpenCode

- 一句话复述理解：推进实施 v1.15.0-feature，完成前端实时绿框渲染打通与独立 Python 模板匹配工程编写及联调。
- 推进阶段实施：
  1. **前端绿框打通**：在 `frontend/src/template/useTemplateMatches.ts` 中实现订阅 `/api/vision/results` WebSocket 广播、动态对齐 Canvas 尺寸、绘制高亮绿框（`#00e676`）与半透明标签背景、置信度显示及 `expires_ms` 定时清框。
  2. **独立匹配工程实现**：在 `scripts/template_matcher/` 实现 `requirements.txt`、`template_loader.py`（自动读取 `data/templates/` 本地模板）、`matcher.py`（OpenCV 多尺度匹配 + NMS）、`client.py`（PyAV H.264 内存流解码与双向 WebSocket 通信）、`main.py`（CLI 参数支持）与详细 `README.md`。
  3. **自动化测试与联合验证**：
     - 编写并运行 `test_matcher.py`，验证模板扫描与合成图像坐标匹配通过；
     - 运行 `scripts/build.sh`（前端 `vue-tsc` 检查、Vite 打包和 Go 编译）全流程通过；
     - 运行 Go 单测 `go test ./...` 全部通过。
  4. 同步更新任务清单与版本说明。

### 5. 用户

> 其实我本地有个 yolo_wepork 项目你可稍微看一下，已经实现了模板匹配等

### 5. OpenCode

- 深入研读本地 `/Users/liuzhuo/code/yolo_wepork` 项目实现（重点关注 `opencv_match/stream_vision.py` 与 `service.py`）：
  - 核心亮点：`LatestFrames` 单容量异步队列（生产者后台持续解码，消费者按需取最新画面，自动清空积压旧帧，避免算法耗时导致帧堆积延迟）。
  - 协议要点：透传并携带握手拿到的 `session_id`；直接使用 `av.Packet(annexb_nals(payload))` 进行极速解包；空目标明确回传 `objects: []` 促使前端精准清框。
- 对齐吸收至当前项目：优化 `scripts/template_matcher/client.py`，引入 `FrameDecoder` 与 `LatestFrames` 异步丢帧队列机制，并透传 `session_id`。
- 回归运行 `test_matcher.py` 验证完全兼容。

### 6. 用户

> 现在模板是加载的接口的吗， 不是本地的图片吧

### 6. OpenCode

- 用户敏锐指出模板源应走接口以支持远程与界面热更新。
- 改造 `template_loader.py`：
  - 默认模式：通过 `GET /api/templates?serial=...` 拉取元数据，通过 `GET /api/templates/{id}/image` 在线下载图片二进制流并用 `cv2.imdecode` 解码至内存；
  - 内存缓存：按 `updated_at` 对比模板变更，未变更的模板无需重复下载；
  - 兜底与离线：支持 `--from-disk` 命令行开关，离线环境可直接从本地 `data/templates` 扫描。
- 同步更新 `main.py` 与 `README.md`。

### 7. 用户

> 1

### 7. OpenCode

- 一句话复述理解：进行真机链路联调，验证在线 API 模板同步与实时视频流握手及消费。
- 实机验证：
  - 探测到在线真机 `10.0.0.30:5555`；
  - 测试通过 HTTP API 从运行中的 MyWebScrcpy 服务在线同步到模板并在内存中成功解码；
  - 运行 `main.py` 启动匹配客户端连接真机 `10.0.0.30:5555`，成功建立 Session `a313fbd15c68326362c2f9af` 并稳定消费视频流；
  - 检查服务端 `/api/vision/stats` 确认订阅者增加、视频流正常转发且丢帧数为 0；
  - 同步更新任务与版本文档。

### 8. 用户

> 再帮我测试一下

### 8. OpenCode

- 借助当前正在运行的 9222 端口 Chrome 浏览器进行全链路 CDP 实机渲染自动化测试：
  1. 通过 CDP 连接至 Chrome 实例并将页面导航至 `http://127.0.0.1:8080/player?serial=10.0.0.30:5555`；
  2. 验证前端视频画面与各层 Canvas 就绪，确认 `tm.overlayRef` 分辨率自动匹配为 448 × 1024；
  3. 建立旁路 WebSocket 推流，发送 `detection.result` 测试绿框；
  4. 采用 CDP `Runtime.evaluate` 读取 `tm.overlayRef` 画布 ImageData，检测到 2008 个 `#00e676` 亮绿像素，证实绿框成功绘制且位置尺寸准确；
  5. 等待 5.5 秒观察 `expires_ms` 定时清框防残影机制，再次读取 Canvas 像素发现绿框像素归零，验证防残影完全有效。
- 更新任务清单与版本说明。
