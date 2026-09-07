# v1.4.0-feature 任务清单

## 1. 现状与契约

- [x] 1.1 核查 Go 入口、设备管理、scrcpy 启动/握手/读帧/控制写入和现有路由。
- [x] 1.2 核查前端投屏、WebCodecs、Canvas 尺寸/坐标映射及大屏复用情况。
- [x] 1.3 记录当前无 Vue、无 Vision 订阅、无 ActionQueue、无共享 Session/FrameHub 的事实。
- [~] 1.4 已确认 Python 以独立 Vision 客户端接入、MVP 使用 scrcpy-frame；鉴权和更细的动作安全策略按用户决定延期。
- [x] 1.5 明确 MVP 使用双向 WebSocket：FrameHub 发帧，VisionGateway/ResultHub 收检测结果；冻结职责边界。
- [x] 1.6 冻结 `protocol_version`、device/session/frame_id、坐标和错误码契约，并补充 `docs/vision-integration.md`。
- [~] 1.7 已记录 Python ADB 仅用于只读诊断/安装/环境准备，控制动作优先走 Go 队列；互斥租约留后续。

## 2. MVP 实现

- [x] 2.1 为设备抽取可复用 Session 生命周期，保留旧 `/ws` 兼容适配器；同一 serial 共享 scrcpy 进程。
- [x] 2.2 实现有界 latest-frame FrameHub、frame_id/时间戳/尺寸元数据和慢消费者丢帧基础能力；浏览器 WS 已接入共享 SessionRegistry。
- [x] 2.3 实现 Python Vision scrcpy-frame 订阅与检测结果回传协议，默认本机/显式开关。
- [x] 2.4 实现设备级 DetectionEvent 广播，隔离不同 serial/session，并校验检测框边界。
- [x] 2.5 实现 Action 类型、坐标校验、设备级串行队列和结果响应；浏览器 raw control 与 Vision 的 tap/swipe/key/text 共用队列。
- [x] 2.6 在播放器增加独立 overlay Canvas；同步显示尺寸和页面缩放。
- [x] 2.7 增加 `scripts/vision/demo_vision.py` 演示客户端：接收 scrcpy 帧、PyAV/OpenCV 本地画框并回传 `detection.result`；不发送动作。
- [x] 2.8 dashboard 接入同一结果广播 WebSocket，增加检测框 overlay，并按当前 `session_id` 过滤旧结果。

## 3. 低延迟优化

- [~] 3.1 已接通 Vision `max_fps` delta 降采样，并保留 config/key/session；尺寸/质量协商仍留后续，Session stats 已提供背压/丢帧指标。
- [~] 3.2 增加只读 `benchmark_stream.py`，可在真实设备上测量 H.264 接收/解码和 Python 侧 JPEG 转码基线；Go 端 JPEG 推流与共享解码帧的完整对比仍待后续实现。
- [~] 3.3 已提供 heartbeat、队列深度、最近帧时间、检测结果 `expires_ms` 校验和基准传输延迟；持续运行时的端到端链路指标仍留后续。
- [x] 3.4 已修复静止画面读超时和浏览器 watchdog 误判导致的周期性重连；通过真实设备旋转/重启 scrcpy 验证旧 Session 断开、新 Session 恢复和方向状态恢复，并连续观察静止页面 8 秒保持连接。

## 4. 多设备与调试

- [x] 4.1 实现基础 SessionRegistry：按 serial 复用、引用计数、10 秒重连保留窗口和资源回收；离线事件细化留待后续。
- [~] 4.2 Vision/Action 鉴权、来源限制、设备授权和只读订阅模式延期；当前仅用于本机/可信内网测试。
- [x] 4.3 增加 `/api/vision/stats`，提供订阅数、帧数、丢帧数、动作队列长度和最近帧时间；播放器 UI 指标仍可后续补充。
- [x] 4.4 增加按 device_id/session_id/request_id 的结构化关联事件和有界环形调试缓冲，提供只读查询接口。

## 5. 测试与发布

- [x] 5.1 已为 FrameHub、SessionRegistry 缓存/慢订阅、binary 消息编解码、坐标边界和 ActionQueue 增加 Go 单元测试。
- [x] 5.2 已覆盖共享订阅、慢消费者丢旧帧、Vision 断线、过期结果和两台设备隔离；结合 Go 单测与 `10.0.0.30:5555`/`10.0.0.244:5555` 运行冒烟完成。
- [x] 5.3 运行 `go test ./...`、`go vet ./...`、前端 `node --check`、`git diff --check`。
- [x] 5.4 使用 `10.0.0.30:5555` 验证设备列表、Vision hello、真实 scrcpy 帧、检测结果广播和越界动作拒绝；未发送有效点击。
- [x] 5.5 完成 Python 演示脚本语法检查；运行窗口需用户安装 `websocket-client`、`av`、`opencv-python` 后现场确认。
- [~] 5.6 已确认无敏感值/视频内容进入日志，结构化动作有白名单和过期校验；鉴权、来源限制和公网暴露防护按用户决定延期，当前只适合本机/可信内网。
- [x] 5.7 已按用户指示更新正式版本、提交并推送代码与标签。
