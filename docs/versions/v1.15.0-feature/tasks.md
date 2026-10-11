# v1.15.0-feature 任务清单：独立模板匹配工程与实时绿框展示

**状态：全链路功能开发完成，已通过 9222 端口 Chrome CDP 浏览器实机自动化验收。**

## 1. 独立模板匹配工程 (`scripts/template_matcher/`)

- [x] 1.1 创建 `scripts/template_matcher/requirements.txt`（声明 `websocket-client`、`av`、`opencv-python`、`numpy` 等依赖）。
- [x] 1.2 实现 `scripts/template_matcher/template_loader.py`（默认通过 HTTP API 动态下载模板列表与图片并缓存；支持本地离线扫描兜底）。
- [x] 1.3 实现 `scripts/template_matcher/matcher.py`（OpenCV 多尺度匹配算法、阈值过滤、NMS 及归一化坐标计算）。
- [x] 1.4 实现 `scripts/template_matcher/client.py`（基于 `LatestFrames` 异步丢帧队列消费、PyAV H.264 内存流解码、心跳维持、透传 `session_id` 与检测结果上行回传）。
- [x] 1.5 实现 `scripts/template_matcher/main.py`（CLI 参数解析与主事件循环装配，默认在线 API 模式）。
- [x] 1.6 编写 `scripts/template_matcher/README.md`（说明使用方法、在线/离线模式与运行示例）。

## 2. 前端实时绿框渲染打通 (`frontend/`)

- [x] 2.1 改造 `frontend/src/template/useTemplateMatches.ts`：
  - 连接 `WS /api/vision/results?serial={serial}` 广播通道。
  - 自动管理 WebSocket 连接与断线重连。
  - 解析 `detection.result` 事件并同步覆盖 Canvas 尺寸。
  - 绘制 `#00e676` 高亮绿框与置信度标签文本。
  - 接入 `expires_ms` 定时清框与开关关闭时清理逻辑。
- [x] 2.2 在 `frontend/src/views/PlayerView.vue` 与 `PlayerSettingsPanel.vue` 核对 `useTemplateMatches` 生命周期装载与 `boxesVisible` 开关联动。

## 3. 联调验证与验收

- [x] 3.1 运行前端 `pnpm run build`，确保 TypeScript 类型与前端工程构建无误。
- [x] 3.2 运行 Go 基础测试 `go test ./...` 确保服务端既有接口完好。
- [x] 3.3 运行 `test_matcher.py` 自动化测试，验证 Loader 扫描与 OpenCV 匹配及归一化坐标换算无误。
- [x] 3.4 连接在线真机 `10.0.0.30:5555` 实机测试，成功通过 HTTP API 在线同步模板，并通过 WebSocket 建立 Session (`a313fbd15c68326362c2f9af`) 稳定消费视频流无丢包。
- [x] 3.5 通过 9222 端口 Chrome CDP 浏览器完成实机端到端渲染验收：
  - 浏览器加载投屏后，overlay Canvas 物理分辨率与视频尺寸精确对齐（448 × 1024）；
  - 接收到 `detection.result` 后，Canvas 成功绘制亮绿色（`#00e676`）目标外框与标签文本（检出亮绿像素 2008 px）；
  - 超过 `expires_ms` 后定时清框逻辑触发，画布亮绿像素清零（0 px），防残影机制验证通过。
