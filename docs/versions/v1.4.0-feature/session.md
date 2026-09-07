# v1.4.0-feature 会话记录

## 记录原则

- 本版本先沉淀现状核查与分阶段改造计划，不直接大改业务代码。
- 文档中的接口和模块名称是建议草案；进入实现前仍需按实际代码确认兼容细节。

## 会话

### 1. 用户

> Continuing from「scrcpy自动化识图」：检查 MyWebScrcpy 项目实际代码与目录结构，重点定位 scrcpy-server 启动与视频流接收/分发、Go 端设备管理与 ADB 控制入口、WebSocket/HTTP 接口、Vue/前端投屏组件与视频渲染方式，以及是否已有 frame/session/device 抽象。基于现状创建一版详细但务实的改造版本计划，不要直接大改代码。目标架构是 Go 继续负责设备、ADB、scrcpy、视频流；新增供 Python Vision 服务订阅的实时帧/视频流接口；Python 负责 OpenCV/YOLO/OCR 与自动化决策；检测结果通过 Go/WebSocket 回传前端并在投屏上用 Canvas/SVG overlay 反显；动作统一通过 Go Action API/队列执行。计划分阶段（MVP→低延迟优化→多设备/调试能力），列出建议新增/修改的具体文件或模块、接口草案、数据结构、风险点、兼容策略，并优先复用现有代码。

### 1. 代理

按 `lz-vlog` 版本工作流建立 `v1.4.0-feature` 设计记录。只读核查 `/Users/liuzhuo/code/MyWebScrcpy` 后确认：

- `main.go` 负责嵌入 scrcpy-server、初始化 `device.Manager`、`ws.Hub`、文件/脚本路由和少量 ADB REST 接口。
- `internal/device/manager.go` 只有基于 `adb devices -l` 的 2 秒轮询和 `Device` 列表缓存，没有设备会话生命周期。
- `internal/scrcpy/server.go` 完成 push、ADB forward、app_process 启动和停止；`connection.go` 完成视频/控制双 socket 握手、`Frame` 解析和控制写入。
- `internal/ws/hub.go` 的 `session` 是单个浏览器 WebSocket 私有对象；`pumpVideo` 将 `[kind][pts][payload]` 二进制帧直接写回浏览器，`pumpControl` 将浏览器二进制消息透传给 scrcpy control socket。
- 现有 HTTP 只有设备列表、旋转、屏幕状态、重启、文件和脚本等路由；不存在帧订阅、检测结果通道、动作队列或 Python 服务协议。
- 前端使用 `web/player.html`、`web/js/decoder.js`、`control.js` 和 `dashboard.js` 的原生 JavaScript + WebCodecs + Canvas，不是 Vue；Canvas 目前既是视频渲染面也是输入事件承载面，尚无独立检测 overlay。
- 已有 `scrcpy.Frame`、`ws.session`、设备 `Serial`，但没有可被多个消费者共享的 `frame/session/device` 服务抽象。

本版本计划优先抽取现有读帧和控制逻辑，保持 `/ws?serial=` 及现有二进制协议兼容，再逐步引入按设备复用的媒体会话、Vision 订阅、结果广播和动作队列。

### 2. 用户补充问题

用户确认：Python Vision 识别后，是否通过 WebSocket 把数据返回给 FrameHub。

### 2. 代理结论

MVP 建议 Python 与 Go 建立双向 WebSocket：Go FrameHub 向 Python 发送 JPEG/帧元数据，Python 向 Go 回传 `detection.result` 和可选 `action.request`。职责上，回传目标应称为 Go 的 `VisionGateway/ResultHub`，而不是让 FrameHub 直接承担结果业务；ResultHub 再负责前端广播，ActionQueue 负责动作执行。后续可按权限和部署需要拆分帧订阅连接与结果上报连接。

### 3. 用户补充意见

用户认为 Python 后期可能直接使用 ADB，但这与当前设计不冲突。

### 3. 代理结论

兼容，但需要把 Python ADB 定义为显式旁路：只读诊断、设备信息、安装或环境准备可独立执行；控制类 ADB 命令默认不与 Go scrcpy control 并发。若确需控制，必须标记控制来源、获取同一设备互斥租约并暂停/排空 Go ActionQueue，避免动作乱序和设备串线。该边界已补入设计、兼容策略、风险和待确认问题。

### 4. 本轮实施与验证（2026-09-07）

- 新增 `internal/framehub` 有界 latest-frame 发布/订阅基础包及慢消费者测试。
- 新增 `internal/action` 动作请求校验与串行队列；Vision 通道初始先接入 tap，后续已扩展为 swipe/key/text，越界坐标和未实现动作会被拒绝。
- 新增 `internal/vision` JSON 协议；`/api/vision/stream?serial=` 提供双向 Vision 通道，`/api/vision/results?serial=` 提供浏览器结果广播。
- 播放器和 dashboard 均新增独立 detection overlay Canvas，不改变原视频 Canvas 和输入事件。
- `10.0.0.30:5555` 在线验证通过：Vision hello、真实 scrcpy binary 帧、检测结果广播、越界动作拒绝；未发送有效点击。
- `go test ./...`、`go vet ./...`、前端语法检查和 `git diff --check` 通过。
- 增加 `scripts/vision/demo_vision.py`：默认连接 `10.0.0.30:5555`，PyAV 解码、OpenCV 显示绿色演示框，并通过 `detection.result` 回传浏览器；明确不发送动作。
- 已完成 Python 脚本 `py_compile`；随后使用项目虚拟环境完成真实 PyAV 解码复测，GUI 窗口显示仍需用户在本机桌面现场确认。

当前未完成：JPEG 转码/低延迟旁路和完整多设备离线调试能力仍留在后续任务。

### 5. SessionRegistry 推进与自测（2026-09-07）

- 新增 `internal/ws/registry.go`：按 `serial` 复用 scrcpy session，单一读帧 goroutine 向浏览器/Vision 订阅者广播；控制写入加互斥锁。
- 浏览器 `/ws` 与 `/api/vision/stream` 均改为订阅共享 session；同一设备不再重复 push server jar 或启动多套 scrcpy。
- 新订阅者会收到最近的 config/key 帧，避免只收到 delta 导致 WebCodecs watchdog 重连。
- 引用归零后保留 10 秒再释放，覆盖浏览器自动重连和设备帧恢复间隙。
- 自测：Go 测试、`go vet`、前端语法、Python 语法和 `git diff --check` 通过；`10.0.0.30:5555` 上双 Vision 连接并发收到帧，结果广播和越界动作拒绝通过，日志只出现一次 jar push。

剩余范围：Vision/Action 鉴权（用户明确延期）、完整离线事件和 JPEG 转码仍未完成。

### 6. Python 对接文档与控制队列（2026-09-07）

- 新增 `docs/vision-integration.md`，记录启动命令、Vision WebSocket、binary 帧格式、`detection.result`、`action.request`、重连、安全边界和 Python 示例。
- 浏览器 raw control 与 Vision 结构化动作现在共用设备级 ActionQueue；队列执行器统一持有 control 写锁，避免两类控制消息并发写入 scrcpy socket。
- 运行 `go test ./...`、`go vet ./...`、前端语法检查、Python 语法检查和中文文档检查均通过。

### 7. Dashboard 与范围调整（2026-09-07）

- dashboard 复用 `/api/vision/results`，增加独立检测框 Canvas overlay，并在销毁设备卡片时关闭结果连接。
- 用户确认暂不考虑 Vision/Action 鉴权、来源限制、设备授权和只读订阅模式；当前文档保留可信内网限制，不将鉴权列为本阶段完成条件。

### 8. 统计与心跳自测（2026-09-07）

- 新增 `GET /api/vision/stats?serial=`，返回订阅数、帧数、丢帧数、动作队列长度和最近帧时间。
- Vision 支持 `heartbeat` 请求/响应，便于 Python 端判断 Go 通道是否仍在线。
- 使用 `10.0.0.30:5555` 自测通过：hello、frame 元数据、binary 帧、heartbeat、越界动作拒绝和 stats 响应均正常。

### 9. 协议封包与 PyAV 解码复测（2026-09-07）

- 修复 shared frame envelope 的非 session 分支多分配 8 个尾部零字节问题；现在 key/delta/config 帧长度严格为 `9 + payload`，session 帧固定 17 字节。
- 旋转/尺寸变化帧会清除 Session 缓存的旧 config/key，避免新订阅者复用过期编解码参数。
- Vision 客户端断开时会主动结束读循环，避免无新帧时残留 goroutine。
- 浏览器共享视频订阅结束时会发送 `disconnected` 并关闭 WS，确保前端控制读循环退出并按既有逻辑重连。
- 使用 `/Users/liuzhuo/.venv/bin/python` + PyAV 对 10.0.0.30:5555 真实 H.264 流复测：收到 3 条 frame 元数据并成功解码 1 帧；macOS 的 AVFoundation 重复库提示仍存在，但未阻断解码。
- 补充 `internal/ws/registry_test.go`，并再次通过 `go test ./...`、`go vet ./...`、`git diff --check`。
- 最终回归另通过 `go test -race ./internal/ws ./internal/action ./internal/framehub`、前端/脚本语法检查和中文文档规则检查（仅设计稿已有弯引号风格建议，无 error）。

### 10. Vision 降采样（2026-09-07）

- Vision 客户端现在可在连接后发送 `{"type":"hello","max_fps":5}`；Go 返回 `hello.ack`，只对 delta 帧按上限采样，config/key/session 始终发送。
- `max_fps=0` 表示不额外采样，服务端将上限限制为 30；补充归一化和采样单测。
- 发现并修复静止画面被视频读超时误判为断流的问题；`ReadFrame` 不再设置固定读超时，由 socket 错误或显式关闭结束，正常静止画面不再触发周期性重连。
- Python 演示脚本现在默认请求 `max_fps=5`，在无视频包的静止阶段每 5 秒发送 heartbeat，便于长期运行的 Vision 客户端保持通道状态。
- 为 Vision hello/frame/result/stats 补齐 `session_id`；收到非当前 Session 的结果时返回 `session_mismatch`，避免重连后旧检测结果串线。
- 增加检测框服务端校验：标签、置信度和归一化边界不合法时返回 `invalid_detection`，不广播到前端。
- 使用 `10.0.0.30:5555` 复测：越界框返回 `invalid_detection`，合法框仍通过 `/api/vision/results` 广播；stats 同时返回当前 `session_id`。

### 11. 关联调试事件（2026-09-07）

- 新增 `internal/debug` 有界环形缓冲，记录 Session 启动/断流、检测拒绝和 Action 结果，带 `device_id`、`session_id`、`request_id`，不记录媒体内容。
- 新增 `GET /api/vision/debug?serial=...&limit=...` 只读查询接口，默认返回最近 100 条，最多 500 条。
- 使用两台在线设备 `10.0.0.30:5555`、`10.0.0.244:5555` 并发连接 Vision，hello 返回不同 `session_id` 且设备标识保持隔离；debug 查询成功返回 Session 启动和检测拒绝事件。
- 增加检测结果 `expires_ms` 时效校验，过期结果返回 `expired_result` 并记录 `detection.expired` 事件。
- 播放器和 dashboard overlay 现在按当前 `session_id` 过滤结果，并在断线/新 Session 时清空旧检测框。
- ActionQueue 增加过期动作和队列满的自动化测试，确认过期动作不会执行、满载时会返回 `queue_full`。
- SessionRegistry 增加慢消费者测试：不读取的订阅者会丢弃旧帧、保留最新帧，并累计丢帧计数，不阻塞共享读帧循环。

### 12. 旋转/编码器重启回归（2026-09-07）

- 在独立端口对 `10.0.0.30:5555` 记录初始 `user_rotation=1`，连接 Vision 后调用 `/api/rotate` 两次。
- 两次旧连接均收到 WebSocket 关闭；每次重连都生成新的 `session_id`，hello 恢复成功；第二次后设备方向恢复为 `1`。

### 13. 项目入口与协议版本校验（2026-09-07）

- README 增加 Python Vision 对接入口和可直接运行的演示命令，避免新用户只看到内部接口而找不到使用说明。
- Vision 通道对显式 `protocol_version` 做白名单校验：当前支持 `1`，未知版本返回 `unsupported_protocol` 并写入 `protocol.rejected` 调试事件；省略版本号继续兼容旧客户端。
- 真实设备 `10.0.0.30:5555` 冒烟通过：未知版本被拒绝，版本 `1` 和省略版本号均能收到 `hello.ack`。
- 端到端复测通过：Vision 收到真实 binary 帧后回传 `detection.result`，结果通道收到相同 `session_id` 和演示框；播放器 HTTP 入口返回 200。
- 新增只读 `scripts/vision/benchmark_stream.py`，使用项目虚拟环境在 `10.0.0.30:5555` 运行 5 秒基线：收到 8 个编码包、解码 6 帧，平均解码 3.187 ms/帧，Python 侧 JPEG 编码 1.545 ms/帧；该数据不冒充 Go 端 JPEG 推流结论。
- ActionQueue 补齐 `swipe`、`key`、`text`：增加起止坐标、keycode、文本字段及对应 scrcpy 控制消息编码；真实设备仅验证非法参数返回 `invalid_coordinate`、`missing_keycode`、`missing_text`，未发送有效动作。
- Session 回收时现在显式关闭 ActionQueue；关闭后的新请求返回 `queue_closed`，并增加并发关闭/提交测试，避免设备离线后残留队列 worker。
- 进一步修复关闭竞态：worker 不会在关闭后继续执行已排队动作，所有 pending 请求都会收到 `queue_closed`；重复运行关闭测试通过。
- 补充 scrcpy `TouchEvent`、`KeyCodeEvent`、`TextEvent` 编码单测，确认结构化动作使用的 control 消息字段和长度正确。
- 修复静止画面下浏览器 watchdog 误重连：Go 浏览器视频通道每 2 秒发送 `stream.heartbeat`，前端按心跳/数据包判断流活跃，不再要求 5 秒内必须有新解码帧；实际浏览器页面连续观察 8 秒保持连接，控制台无 error/warn。
- 在稳定页面上向 `10.0.0.244:5555` 回传 `overlay-check 97%` 结果，浏览器实际显示绿色检测框和标签，确认 overlay 从 Vision 到前端的可见闭环。
- 修复后再次观察同一页面 20 秒（超过原先约 6 秒重连周期），连接保持、帧率显示正常，浏览器控制台无 error/warn，服务日志未出现重复连接。
- 前端 overlay 增加 `expires_ms` 自动清除；真实页面发送 1 秒有效期的 `expiry-check` 后等待 2 秒，检测框已从画面消失，避免显示过期识别结果。
- 修复 overlay 定时器竞态：新结果到达时会取消旧结果的清除定时器；真实页面先发 1 秒旧框、再发 5 秒新框，等待 2 秒后新框仍可见。
- 在 dashboard 临时页面中验证 `10.0.0.244:5555` 的 `dashboard-check 96%` 绿色检测框实际反显，多设备卡片页面闭环通过。
- 基准脚本增加跨 macOS/Linux 的峰值 RSS 增量字段；在 `10.0.0.30:5555` 重新运行 5 秒样本得到 2 个解码帧、9.506 ms/帧解码、4.280 ms/帧 JPEG、峰值 RSS 增量 11.665 MB（网络和设备状态会影响结果）。
- 新增 `scripts/vision/requirements.txt`，demo/benchmark 的依赖安装和缺依赖提示统一指向该清单；Python 语法与中文文档检查通过。
- 基准脚本增加传输延迟统计；`10.0.0.30:5555` 3 秒样本测得平均 0.4 ms、最大 1 ms（本机网络环境，仅作基线）。
