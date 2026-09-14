# v1.9.0-feature 版本摘要

1. **点击/滑动接口扩展 `mode` 参数**：
   - 动作请求（`action.Request` 及 Vision WebSocket / HTTP 上行协议）新增 `mode` 字段，支持两种执行通道：
     - `sdk`（默认）：复用现有 scrcpy 控制 Socket 注入触摸事件，保持极低延迟与拟人化手势能力。
     - `adb`：通过系统级 `adb shell input tap` 与 `adb shell input swipe` 执行控制，穿透部分限制辅助注入的特殊页面与系统弹窗。
   - 配合新增 `duration_ms` 参数，支持精确控制滑动过程的持续毫秒数。

2. **保持 100% 向后兼容**：
   - 既有客户端及自动化脚本未携带 `mode` 字段时，缺省自动降级为 `sdk` 模式。
   - 既有拟人化控制（`humanize=true`：高斯落点随机化、缓动贝塞尔曲线、微抖动节奏）在 `sdk` 模式下完全不受影响。

3. **智能物理坐标转换**：
   - `adb` 模式无缝兼容现有的 0.0~1.0 归一化浮点坐标系统。
   - 后端根据设备当前屏幕物理分辨率（`Width` 与 `Height`）自动完成像素转换与边界安全夹取（Clamping），避免手动计算像素偏移。

4. **复用已有 ADB 进程管理器**：
   - 后端统一复用 `internal/adbcommand/manager.go` 执行底层命令，避免重复创建未受控的子进程与开销。
   - 严格纳入 `devicegate` 串行网关管控，杜绝多通道并发引发的设备端指令乱序与争抢。
   - ADB 模式执行成功后，同样联动广播 `touch.event` 投屏可视化事件，保证前端涟漪动画一致性。

5. **明确剔除项及技术结论**：
   - **剔除 `aoa/otg`**：在单 USB 物理连接场景下，切换 AOA（Android Open Accessory）协议会强制触发 USB 描述符重配置与总线重枚举（Bus Re-enumeration），导致正在推流的 scrcpy 视频流和控制通道瞬间断开崩溃；另起独立进程也无法规避单物理总线独占限制。
   - **剔除 `uhid`**：scrcpy 的 UHID 模式对鼠标仅支持相对坐标位移，无法支持绝对点位点击与滑动；若模拟触控屏 Digitizer HID 报文，则协议极其繁琐且在不同 Android 内核版本上存在兼容性裂痕。
   - **剔除 `sendevent`**：直接写入 `/dev/input/event*` 必须依赖 Root 权限，且多点触控 ABS 轴极值量程在不同设备硬件间高度碎片化，缺乏可移植性与商用稳定性。
