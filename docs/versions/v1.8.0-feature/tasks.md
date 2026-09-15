# v1.8.0-feature 实施任务清单

## 1. 规格与设计
- [x] 1.1 完成 session.md、design.md、tasks.md 与 version.md 初稿。
- [x] 1.2 明确模板数据结构定义（ID、名称、设备标识、阈值、尺度、灰度、开关等）。
- [x] 1.3 确认既有 `/api/vision/*` WebSocket 协议保持完全向后兼容。

## 2. 后端模板管理与 OpenCV 匹配模块 (internal/template)
- [x] 2.1 创建 `internal/template` 独立包，定义 `Template`、`MatchResult`、`DeviceStatus` 等结构体（`model.go`）。
- [x] 2.2 实现纯 Go OpenCV 模板匹配引擎（`matcher.go`）：基于积分图与平方积分图加速的 `TM_CCOEFF_NORMED` / `TM_SQDIFF_NORMED`、双线性插值多尺度、灰度模式及局部极大值 NMS 去重。
- [x] 2.3 实现文件持久化引擎（`storage.go`）：支持设备隔离（`data/templates/{serial}/` 与 `data/templates/global/`）、并发读写锁安全缓存与模板全套 CRUD。
- [x] 2.4 实现服务控制层（`service.go`）：管理后台周期性截屏匹配 goroutine、转换结果并推送到既有 Hub 广播通道，未命中或关闭时广播空 objects 清屏。
- [x] 2.5 实现模板管理与检测 REST API 路由（`handler.go`）：涵盖模板 CRUD、设备匹配状态开关、最新匹配结果查询、单次即时检测端点。
- [x] 2.6 编写详尽单测（`matcher_test.go`、`storage_test.go`、`service_test.go`、`handler_test.go`），全量通过 `-race` 检查，覆盖率近 80%。

## 3. 前端交互与播放器集成 (web)
- [x] 3.1 区域截图选区联动：在 `player.html` 的 `capture-actions` 工具栏中新增“存为模板”按钮（`#capture-save-template`）。
- [x] 3.2 实现极简模板保存弹窗（`#template-save-dialog`）：复用 Canvas 区域切片，配置模板名称、设备/全局生效范围及匹配阈值，一键提交持久化。
- [x] 3.3 播放器更多菜单集成：新增“模板匹配”设备级启停开关（`#btn-toggle-template-matching`）与“模板管理”入口（`#btn-template-manage`）。
- [x] 3.4 播放器模板管理面板（`#template-manage-dialog`）：卡片式网格展示、缩略图预览、阈值滑动实时调节、独立启用开关、删除二次确认及顶部实时匹配命中状态展示。
- [x] 3.5 投屏绿框复用：匹配结果统一以 `detection.result` 通过既有 `/api/vision/results` WebSocket 广播，无缝复用播放器已有 `detection-overlay` 绿框高亮渲染。
- [x] 3.6 编写 `web/js/template-match.js` 与 `web/css/style.css` 响应式与暗色风格样式。

## 4. 主程序接入与测试验证
- [x] 4.1 在 `main.go` 中初始化存储与服务，注册 `/api/templates/*` 路由并配置优雅退出。
- [x] 4.2 更新 `web/openapi.json` 并扩展 `api_docs_test.go`，确保 API 文档一致性验证通过。
- [x] 4.3 运行全仓库 `go test -v -race ./...`，全量测试 100% 通过，0 竞态问题，原有接口保持 100% 向前兼容。

## 5. 模板导入/导出与控件优化
- [x] 5.1 更多菜单“模板匹配”改为普通按钮（非 switch），默认不开启匹配；弹窗内“开启匹配”使用 switch 控件。
- [x] 5.2 修复弹窗未打开时穿透显示在投屏后方的 CSS 层级与特异性问题。
- [x] 5.3 支持 ZIP 格式模板批量导出（`GET /api/templates/export?serial=...&scope=...`，压缩包内为 PNG 格式，以文件名作为模板名称）。
- [x] 5.4 支持 ZIP 格式模板批量导入（`POST /api/templates/import`，自动解析过滤非 PNG，以文件名建模板）。
- [x] 5.5 支持单张 PNG 模板上传（`POST /api/templates`，自动提取文件名作为模板名）与单张下载（`GET /api/templates/{id}/image?download=1`）。
- [x] 5.6 页面提供格式提示文案，补全 OpenAPI 契约与单元测试。
- [x] 5.7 支持网络 ADB 设备序列号（如 `10.0.0.11:5555`，含冒号 `:`）模板持久化存储，拆分 ID 与 Serial 校验并做跨平台安全目录映射。
- [x] 5.8 修复 `internal/ws/vision.go` 中设备级消息误过滤 Bug，使模板匹配检测绿框正常投射在播放器投屏上；打通 `POST /api/templates/detect` 单次检测绿框下发通道。
- [x] 5.9 模板卡片操作区新增“转为全局模板 / 转为当前设备模板”快捷切换按钮及样式适配。

