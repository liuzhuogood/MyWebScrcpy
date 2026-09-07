# v1.4.0-feature 版本说明

1. 基于 MyWebScrcpy `v1.3.2` 现状，启动 Go 设备级 Session/FrameHub、Python Vision 订阅、检测结果广播、前端 Canvas overlay 和统一 ActionQueue 的渐进式改造。
2. MVP 优先复用现有 scrcpy 二进制帧格式与 JSON 元数据，新增 Python Vision 双向 WebSocket、检测结果广播、动作校验/队列和播放器 Canvas overlay；Go 侧 JPEG 转码留待后续优化。
3. 低延迟阶段再评估编码帧旁路、采样/背压、frame_id 时效和端到端指标；多设备阶段补充 SessionRegistry、鉴权、调试与结构化关联日志。
4. 已完成基础包测试、Go 静态检查、前端语法检查和真实设备冒烟；已补上 SessionRegistry、多设备 Session 隔离、调试事件缓冲，使浏览器与 Python 共享同一 scrcpy session，并让浏览器 raw control 与 Vision 结构化动作共用 ActionQueue。JPEG 转码/性能对比仍留后续；鉴权按用户决定延期。
5. 提供 `scripts/vision/demo_vision.py` 现场演示脚本，可在 Python 端显示画面、绘制演示框并把识别框回传到播放器；需要额外安装 PyAV、OpenCV 和 websocket-client。
6. 新增 `docs/vision-integration.md` Python 对接文档；浏览器 raw control 与 Vision 动作现在共用设备级 ActionQueue，减少控制并发冲突。
7. 修复 binary frame envelope 尾部填充导致的 PyAV 解码风险，补充 Session 缓存失效、断开读循环和 registry 单测；真实设备 PyAV 解码复测通过。
8. 增加 `session_id` 隔离、检测框与结果过期校验、`/api/vision/debug` 环形事件查询、前端旧结果过滤，以及旋转/编码器重启回归验证。
9. README 已补充 Python Vision 入口；Vision 显式协议版本限制为 `1`，未知版本会返回 `unsupported_protocol`，同时保留省略版本号的兼容路径。
10. 新增只读 `scripts/vision/benchmark_stream.py`，输出 H.264 接收/解码和 Python 侧 JPEG 转码基线；当前实测设备基线为约 1.04 decoded FPS、3.187 ms/帧解码和 1.545 ms/帧 JPEG 编码（5 秒短样本，结果随设备与网络变化）。
11. ActionQueue 已补齐 Vision 的 `swipe`、`key`、`text` 执行编码与字段校验；非法参数会在队列入口返回明确错误，不触碰设备。
12. Session 释放时显式关闭 ActionQueue，避免每次 scrcpy 会话回收后遗留 worker；关闭队列的新请求返回 `queue_closed`。
13. 修复 ActionQueue 关闭竞态，确保关闭后不再执行 pending 动作，并为 pending 请求返回 `queue_closed`。
14. 增加 scrcpy 触摸、按键和文本 control 消息编码单测，覆盖动作执行层的 wire 格式。
15. 为浏览器共享视频通道增加 `stream.heartbeat`，前端 watchdog 改按流活跃信号判断，修复静止画面每几秒自动重连的问题；真实页面连续观察 8 秒保持连接且控制台无错误。
16. 在真实浏览器页面上验证 Vision 结果可见反显：`10.0.0.244:5555` 显示绿色 `overlay-check 97%` 检测框。
17. 静止页面稳定性复测延长至 20 秒，超过原先重连周期仍保持单一连接，确认 `stream.heartbeat` 修复有效。
18. 播放器和 dashboard overlay 增加 `expires_ms` 到期自动清除，并通过真实页面 1 秒有效期结果验证。
22. 修复连续识别结果之间的 overlay 清除定时器竞态，避免旧结果提前清掉新检测框，并完成真实页面验证。
23. 在 dashboard 多设备页面验证 `dashboard-check 96%` 检测框实际反显，确认播放器和大屏使用同一结果广播协议。
19. `benchmark_stream.py` 增加 macOS/Linux 峰值 RSS 增量采样，低延迟基线现在覆盖带宽、帧率、解码/JPEG 耗时和内存。
20. 新增 `scripts/vision/requirements.txt`，统一 Python Vision 演示和基准脚本的依赖安装入口。
21. `benchmark_stream.py` 增加 frame 时间戳到 Python 收包的平均/最大传输延迟指标。
