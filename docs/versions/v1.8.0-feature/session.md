# v1.8.0-feature 会话记录

## 2026-09-14

**用户**：/lz-vlog 创建一个版本增加 opencv 模板匹配的通用方法， 但保留之前的websocket接口，也就是自己实现一套模板匹配管理的接口

模板管理：
- 增删改查，调整匹配参数，模板有命名，区分设备，可以选择设备
- 在区域截图那里可以直接区域保存为模板匹配
- 获取模板匹配的匹配情况，返回匹配的坐标、名称、匹配度
- 可以开启或关闭整个模板匹配功能
- 在投屏上显示绿框（已有功能，对接就行）
- 先设计一下版本，不直接开发代码
- 可以参考Python项目: /Users/liuzhuo/code/yolo_wepork/opencv_match

**执行工具**：
1. 深入研读现有视频流通信机制与 Vision 模块（`internal/vision`、`internal/ws`、`scripts/vision` 及 `docs/vision-integration.md`）。确认既有 WebSocket 契约（`/api/vision/stream`、`/api/vision/results`、`/api/vision/stats`）为成熟稳定的视频流旁路通道，必须 100% 保持向前兼容。
2. 调研参考项目 `/Users/liuzhuo/code/yolo_wepork/opencv_match` 的模板匹配实现模式（`matcher.py` 多尺度归一化互相关匹配、色彩过滤、阈值控制与 watchdog 热重载；`service.py` 任务管理与视频帧对接）。
3. 保持独立架构判断，完成 `v1.8.0-feature` 版本的初步设计与文档建立（`session.md`、`design.md`、`tasks.md`、`version.md` 与 `implementation_plan.md`）。

**用户**：/lz-vlog 开始实现吧， 注意要用go来实现， 做一下单元测试 /goal

**执行工具**：
用户要求使用纯 Go 实现并覆盖单元测试，开启 `/goal` 长程执行模式。主代理将任务分阶段拆解并调度 Luna 子代理分步实施：

- **Stage 1（核心算法与存储引擎）**：
  - 由 Luna 子代理创建 `internal/template` 模块，编写 `model.go`、`matcher.go`、`storage.go` 以及 `matcher_test.go`、`storage_test.go`。
  - 使用纯 Go 基于积分图（Integral Image）与平方积分图实现高性能 `TM_CCOEFF_NORMED` 归一化互相关匹配算法，支持双线性插值多尺度、灰度模式及局部极大值 NMS 去重。
  - 实现基于本地目录与读写锁内存缓存的持久化存储引擎，支持设备隔离（特定设备 vs 全局）。
  - 编写合成图案测试、亮度抗噪测试、多尺度测试及并发读写测试，单测 100% 通过。

- **Stage 2（业务服务与 REST 接口）**：
  - 由 Luna 子代理编写 `service.go` 与 `handler.go`，导出 `Hub.PublishVisionResult` 广播接口。
  - 服务层通过周期性抓屏、调度匹配、未命中清空绿框、状态机启停构建完整的后台匹配 worker。
  - 实现完整的 RESTful 路由（包含模板 CRUD、设备开关、状态与命中查询、单次 detect 接口）。
  - 编写 `service_test.go` 与 `handler_test.go`，覆盖全套端到端路由与并发测试，单测通过率 100%。

- **Stage 3（系统集成与前端接入）**：
  - 在 `main.go` 中完成存储与服务的初始化、生命周期管理与路由注册。
  - 在 `web/player.html` 的截图工具栏中集成“存为模板”按钮，编写 `template-match.js` 实现选区切片直存模板。
  - 在播放器更多菜单中增加“模板匹配”开关与“模板管理”入口；实现模板卡片列表、缩略图、阈值滑块、独立开关、删除及实时命中状态展示。
  - 投屏绿框复用既有 `detection-overlay` 画布与 `/api/vision/results` 广播通道，零冗余代码。
  - 执行全工程 `go test -v -race -count=1 ./...`，全部测试均通过，无竞态问题。

**用户**：我看了一下模板匹配的按钮不应该是 swtich形式吧， 还有默认不要开启。我看了一下现在模板匹配窗口，就算不打开也会显示在投屏的后面， 很奇怪

**执行工具**：
1. **菜单与控件形态优化**：
   - 将更多菜单中的“模板匹配”由 switch 样式改为标准的动作按钮（`<button id="btn-template-match">`），移除多余并列项，统一由此按钮直接打开“模板匹配”弹窗。
   - 在模板匹配弹窗（`#template-manage-dialog`）头部将启停控件置为清晰的 switch 控件（`#template-matching-switch`）。
   - 保证匹配功能默认处于“未开启”状态，仅在用户显式打开 switch 后才向后端请求启动 worker 截屏匹配并开启结果轮询。
2. **修复未打开弹窗透出在投屏后方的 CSS Bug**：
   - 原因排查：原 CSS 中 `.template-manage-modal { display: flex; }` 具有 `(0, 1, 0)` 特异性，覆盖了浏览器对未打开 `<dialog:not([open])>` 原生的 `display: none`，导致弹窗在未调用 `showModal()` 时依然作为普通块级元素参与常规流布局，落在了投屏主画面底层。
   - 修复方案：在 `style.css` 中显式添加 `.template-manage-modal:not([open]), .template-save-modal:not([open]) { display: none !important; }`，并将 `display: flex;` 严格限制在 `.template-manage-modal[open]` 下。弹窗未打开时完全隐藏，无任何穿透和层级异常。

**用户**：模板应该支持导入，导出， zip形式，以文件名作为模板名称，png格式，页面上要说明这种。也可以单张上传、下载

**执行工具**：
1. **后端导入/导出扩展 (`internal/template/handler.go`)**：
   - 增加 `GET /api/templates/export?serial=...&scope=...`：使用 Go 标准库 `archive/zip` 将指定作用域（当前设备/全局/全部）下的模板导出为 ZIP 压缩包，包内文件格式为 `{TemplateName}.png`，自动处理文件名重名冲突。
   - 增加 `POST /api/templates/import`：接收 ZIP 文件并流式解析，过滤 `__MACOSX/` 与隐藏文件，仅提取有效 `.png`，以文件名（剥离扩展名）作为模板名称批量创建并持久化。
   - 扩展 `GET /api/templates/{id}/image?download=1`：设置 `Content-Disposition: attachment; filename="{TemplateName}.png"` 支持单张模板原图附件下载。
   - 扩展 `POST /api/templates`：单张上传时若未显式传名称，自动从上传文件名提取并去除后缀作为模板名称。
2. **前端界面与交互绑定 (`web/player.html`、`web/js/template-match.js`)**：
   - 模板匹配弹窗工具栏增加“上传单张”、“导入 ZIP”、“导出 ZIP”按钮，并展示提示文案：“💡 提示：支持导入/导出 ZIP（压缩包内为 PNG 图片，文件名即为模板名称）；亦支持单张 PNG 上传与下载。”。
   - 卡片上增加单张原图下载按钮，点击直接下载 `{TemplateName}.png`。
   - 完成文件上传/导入/导出的事件绑定与作用域联动。
3. **OpenAPI 契约与全套单测**：
   - 更新 `web/openapi.json` 并扩展 `api_docs_test.go`。
   - 执行 `go test -v -race -count=1 ./...` 全工程通过，所有单测 100% PASS。

**用户**：保存模板失败: invalid template: invalid serial: unsafe path characters in "10.0.0.11:5555"

**执行工具**：
1. **原因定位**：
   - 之前存储层 `validateSegment` 将 `serial` 和 `id` 一律按严格路径分段校验（拦截包含 `:` 的字符串）。
   - 网络 ADB 设备的设备标识格式均为 `IP:PORT`（如 `10.0.0.11:5555`），导致被误判为包含非法字符。
2. **架构优化与修复 (`internal/template/storage.go`)**：
   - 拆分 ID 校验与 Serial 校验：保留 `validateSegment` 用于模板 ID 校验；新增 `validateSerial` 允许包含正常设备标识字符（含冒号 `:`），严防路径穿越（`/`、`\`、`\x00` 及 `..`）。
   - 跨平台安全目录映射：新增 `serialDir(serial)`，在磁盘创建文件夹时将 `:` 自动转换为 `_`（如 `10.0.0.11:5555` -> `10.0.0.11_5555`），杜绝 Windows NTFS/FAT 目录名非法字符问题。
   - 保留原有兼容性：新增 `getTemplateDir`，优先检查未转换的原始目录名，确保平滑兼容；内存中 `Template.Serial` 与 `meta.json` 仍完整保存用户原始的 `10.0.0.11:5555` 串，业务逻辑与 API 返回完全一致。
3. **测试覆盖 (`internal/template/storage_test.go`)**：
   - 新增 `TestStorageNetworkDeviceSerial`，覆盖模板创建、安全目录落盘、内存原始值保持、详情查询、列表检索、参数更新、重启重新加载及安全删除全生命周期，单测 100% PASS。

**用户**：为什么没有显示模板匹配的绿框在投屏上？另外，请在下载模板、删除模板前面增加一个 转成全局模板的按钮， 在全局那边也要能转成当前设备的按钮

**执行工具**：
1. **绿框不显示根因排查与修复 (`internal/ws/vision.go`、`internal/template/service.go`)**：
   - **根因**：浏览器播放器连接 `/api/vision/results?serial=...&session_id=...` 时携带了 `activeSessionID`，而模板匹配属于设备级全局广播（`msg.SessionID == ""`）。`internal/ws/vision.go` 原有的会话过滤条件为 `if sessionID != "" && sessionID != msg.SessionID`，导致设备级广播被误判为会话不匹配而被直接跳过丢弃。
   - **修复**：将过滤条件精确化为 `if sessionID != "" && msg.SessionID != "" && sessionID != msg.SessionID`，仅在消息显式指定特定会话且不一致时跳过，设备级广播（`msg.SessionID == ""`）正常下发给该设备的所有会话通道；`subscribeResults` 同步支持设备级最新状态重放。
   - **打通单次检测下发**：在 `internal/template/service.go` 暴露 `BroadcastMatches`，并在 `handleDetect` 接口中添加自动广播，保证通过 HTTP 调用单次检测时也能即时在投屏上显示绿框。
2. **模板作用域一键切换（“转为全局模板 / 转为当前设备模板”）**：
   - 在 `web/js/template-match.js` 的卡片操作区（下载/删除按钮前）增加切换按钮（`.template-card-scope-btn`）：
     * 若属于当前设备：显示地球图标，点击后发送 `PUT /api/templates/{id}` 将 `serial` 改为 `"global"`。
     * 若属于全局：显示手机设备图标，点击后发送 `PUT /api/templates/{id}` 将 `serial` 改为当前设备的 `serial`。
     * 切换成功后自动调用 `loadTemplates()` 刷新卡片与作用域标签。
   - 在 `web/css/style.css` 中适配统一的悬浮视觉样式。
3. **单元测试与验证**：
   - 在 `internal/ws/vision_results_test.go` 新增 `TestResultSubscriberReceivesDeviceLevelBroadcast`；
   - 执行 `go test -v -race -count=1 ./...` 全库通过。
