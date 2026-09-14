# v1.9.0-feature 实施任务清单

## 1. 规格与设计基准
- [x] 1.1 完成 `version.md`、`design.md`、`tasks.md` 与 `session.md` 版本四件套初稿。
- [x] 1.2 明确 `mode` 参数枚举取值（`"sdk"` 默认、`"adb"`）及 `duration_ms` 参数行为规范。
- [x] 1.3 确立向后兼容约束：未提供 `mode` 字段的存量请求一律无缝走现有 `sdk` 模式和 `humanize` 逻辑。

## 2. 数据结构与协议模型扩展
- [x] 2.1 扩展 `internal/action/action.go` 中的 `Request` 结构体，增加 `Mode string` 与 `DurationMS int64` 字段。
- [x] 2.2 扩展 `internal/vision/protocol.go` 中的 `Message` 结构体，支持解析客户端上行的 `mode` 与 `duration_ms`。
- [x] 2.3 扩展 `internal/action/action.go` 内部校验逻辑：支持合法的 `mode` 校验，非法模式返回明确的错误码（`unsupported_mode`）。

## 3. 核心执行层分流与 ADB 命令封装
- [x] 3.1 为 `deviceActionExecutor` 注入 `adbcommand.Manager` 依赖引用，确保共享已有 ADB 执行器。
- [x] 3.2 实现 `deviceActionExecutor.executeADB` 分支逻辑：
  - [x] 3.2.1 实现归一化坐标至设备屏幕实际物理像素的转换逻辑（包含四舍五入与边界安全 clamp）。
  - [x] 3.2.2 封装针对 `tap` 的 `adb shell input tap <x> <y>` 命令构建与执行。
  - [x] 3.2.3 支持 ADB 物理手指区域拟人化点击（`humanize=true`）：落点施加高斯微偏移模拟接触面积，起点终点相同 `swipe` 模拟 60~150ms 真实物理按压时长。
  - [x] 3.2.4 封装针对 `swipe` 的 `adb shell input swipe <x1> <y1> <x2> <y2> [duration]` 命令构建与执行。
- [x] 3.3 确保 ADB 模式与 `devicegate.Gate` 互斥锁及设备队列串行执行逻辑完整协同。
- [x] 3.4 保持触控可视化一致性：ADB 模式成功执行后，调用 `touchPublisher.emit` 广播 `touch.event`。

## 4. 接口层适配与集成
- [x] 4.1 在 `internal/ws/vision.go` 中解析并透传 `msg.Mode` 和 `msg.DurationMS` 至 `action.Request`。
- [x] 4.2 在 `internal/ws/hub.go` 中提供 `SubmitAction` 接口，支持向设备会话提交通用动作。
- [x] 4.3 处理 ADB 模式下的异常响应映射（如 ADB 离线、超时、命令失败返回对应 `error_code`）。

## 5. 模板匹配与自动点击扩展 (internal/template)
- [x] 5.1 在 `internal/template/model.go` 中定义 `ClickOptions`、`ClickRequest`、`ClickResult` 及 `ErrTemplateNotMatched`。
- [x] 5.2 在 `internal/template/storage.go` 中增加 `FindTemplate` 方法，支持按 ID 或按名称（区分/忽略大小写）两级检索模板。
- [x] 5.3 在 `internal/template/service.go` 中实现 `ClickTemplate`：
  - 自动在当前屏幕进行模板匹配定位；未匹配返回 `ErrTemplateNotMatched`；
  - 默认计算几何中心精准点击 $(cX, cY)$；
  - 支持 `random_offset=true` 启用在目标模板矩形内部的受限随机微偏移（不越界）；
  - 支持 `mode="sdk"` 与 `mode="adb"`（包含拟人化区域点击）。
- [x] 5.4 在 `internal/template/handler.go` 中注册并暴露 `POST /api/templates/click` HTTP RESTful 路由。
- [x] 5.5 前端播放器集成：在 `web/js/template-match.js` 模板卡片上增加一键点击按钮，支持投屏端联动执行并弹出未匹配反馈提示。

## 6. 测试与验证
- [x] 6.1 编写 `internal/action/action_test.go`：覆盖 `Mode` 与 `DurationMS` 字段的序列化、反序列化及校验用例。
- [x] 6.2 编写 `internal/adbcommand/manager_test.go`：测试 `ExecuteDirect` 执行与参数校验。
- [x] 6.3 编写 `internal/ws/adb_mode_test.go`：测试 ADB tap（精确与拟人化）、ADB swipe 与 `touch.event` 广播。
- [x] 6.4 编写 `internal/template/click_test.go`：全量覆盖名称点击、ID 点击、中心点击、随机偏移点击、未匹配报错（422）及 REST 接口端到端验证。
- [x] 6.5 运行全仓库全量单元测试与竞态检测：`go test -race -count=1 ./...`，全量通过（100% PASS）。

## 7. 文档与规范同步
- [x] 7.1 更新 `docs/vision-integration.md`，增加 `action.request` 的 `mode` 与 `duration_ms` 参数文档与示例，及模板点击 API 说明。
- [x] 7.2 同步更新 `web/openapi.json` API 规范文件（包含 `/api/templates/click` 与 `mode` 字段）。
- [x] 7.3 运行 `api_docs_test.go` 确保 OpenAPI 文档合规性校验通过。

