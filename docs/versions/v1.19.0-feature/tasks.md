# v1.19.0-feature 任务清单：投屏启动加速与会话并发优化

日期：2026-10-11。当前状态：正式实现、真机性能测试与回归验证全部完成。

## 需求与设计

- ✅ 排查 `internal/ws/hub.go`、`internal/ws/registry.go` 与 `internal/scrcpy/server.go` 的投屏启动耗时卡点。
- ✅ 创建 `v1.19.0-feature` 版本四件套，并在 `docs/versions/README.md` 登记。

## 基准测试与单元测试（测试先行）

- ✅ 采集优化前真机（`10.0.0.244:5555`）冷启动、二次冷启动与热复用耗时基准数据。
- ✅ 编写单元测试覆盖：Jar 缓存与免推复用、Jar 缺失时自动兜底重推、SDK 版本缓存、多设备并发 `acquireSession` 互不阻塞。

## 功能实现

- ✅ 优化 `internal/scrcpy/server.go`：移除 `Server.Push` 内部多余的 `adb shell ls -la` 校验；传入 `cleanup=false` 防止 `scrcpy-server` 自删手机端 Jar 包；通过 `exited` 通道安全追踪进程提前退出。
- ✅ 优化 `internal/ws/hub.go`：实现按 `serial` 的 SDK 与 Jar 状态单次探测/内存缓存、跳过重复 `adb push`、失败自愈重推，以及 `40ms + 50ms/100ms` 快速握手探测（移除 `1500ms` 硬编码 `Sleep`）。
- ✅ 优化 `internal/ws/registry.go`：引入 `inflightSession` 实现按 `serial` 单飞建连（释放全局锁 `h.sessionMu`），并将默认空闲保活时间延长至 60 秒。

## 性能对比与回归验证

- ✅ 在真机（`10.0.0.244:5555`）与基准测试中执行优化后性能测试，输出优化前后各阶段与总耗时对比表（冷启动从 `2.66s` 降至 `0.53s~0.89s`，提速 `3.0x~3.9x`）。
- ✅ 运行 `go test ./internal/ws/... ./internal/scrcpy/...`、`go vet ./...` 及 `go test -race ./internal/ws/... ./internal/scrcpy/...` 回归验证通过。
- ✅ 更新 `session.md`、`design.md`、`tasks.md` 与 `version.md` 最终结论。
- ✅ 用户回复「2」授权提交并推送本版本改动，隔离工作区其他未提交内容。
