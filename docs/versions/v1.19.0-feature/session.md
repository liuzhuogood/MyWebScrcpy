# v1.19.0-feature 会话记录：投屏启动加速与会话并发优化

日期：2026-10-11。当前状态：已完成实施、真机性能对比测试与回归验证。

## 用户

我有疑问是为什么有时打开手机，投屏会慢， 是因为重新scp了 jar包吗，有没有办法提高效率？

## Gemini

- 排查 `internal/ws/hub.go`、`internal/ws/registry.go` 与 `internal/scrcpy/server.go` 的投屏建连全链路，向用户说明“有时秒开、有时偏慢”的原因：
  - 10 秒内重连或已有活跃订阅时，直接复用 `managedSession` 并重放内存缓存的 `lastConfig` + `lastKey`，因此是秒开；
  - 超过 10 秒无订阅后会销毁 `scrcpy-server`，再次打开需走完整的 `startSession` 冷启动。
- 定位冷启动偏慢的 5 个核心瓶颈：
  1. `internal/ws/hub.go` 在 `server.Start()` 后硬编码 `time.Sleep(1500 * time.Millisecond)` 死等，重试间隔固定 `500ms`。
  2. `internal/scrcpy/server.go` 的 `Server.Push` 每次冷启动无条件执行 `adb push` 推送 715KB 的 `scrcpy-server.jar`，且推送成功后还额外执行一次 `adb shell ls -la` 校验。
  3. 每次冷启动未缓存 `ro.build.version.sdk`，串行触发多次独立 `adb` 子进程调用。
  4. `internal/ws/registry.go` 的 `acquireSession` 在持有全局锁 `h.sessionMu` 期间执行耗时的 `startSession`，导致多设备并发打开或单台慢设备阻塞全局其他设备。
  5. 空闲保活时间仅 10 秒，页面短暂切换即触发冷启动。
- 提出 5 项针对性提速方案并询问是否直接实施。

## 用户

1， 你要创建一个版本来做， lz-vlog
请继续完成， 并做性能测试，对比

## Gemini

- 按 `lz-vlog` 规范，基于当前最新记录 `v1.18.0-feature` 创建 `v1.19.0-feature` 版本四件套（不修改正式 `VERSION` 文件），并在 `docs/versions/README.md` 登记。
- 在真机 `10.0.0.244:5555`（vivo V2072A，Android 13 / SDK 33）上采集优化前基准数据：
  - 单步耗时：`getprop` `175.9ms`、`adb push`（715KB）`212.8ms`、`ls -la` `47.1ms`、`adb forward` `20.1ms`、硬编码等待 `1500ms`。
  - 端到端首帧耗时（`acquireSession` + 收到 `FrameConfig` & `FrameKey`）：首次冷启动 `2.664s`，二次/三次冷启动平均 `2.194s`（`2.306s` / `2.081s`）。
- 实测中发现关键隐蔽根因：`scrcpy-server 4.0` 默认开启 `cleanup=true`，会在启动后拉起 `com.genymobile.scrcpy.CleanUp` 进程自动 `unlink` 删除手机端的 `/data/local/tmp/scrcpy-server.jar`。若仅在 Go 侧跳过 `adb push` 而不传 `cleanup=false`，第二次启动会因手机端 Jar 已被 `CleanUp` 删除而触发 `app_process` 报错 `Aborted (exit status 134)`（此时自愈逻辑成功捕获 `exit status 134` 并自动重推恢复）。
- 在 `internal/scrcpy/server.go` 的 `Server.Start()` 中显式传入 `cleanup=false`，阻止 `scrcpy-server` 自删手机端 Jar 包，同时省去手机端额外拉起 `CleanUp` 进程的开销。
- 完成全部优化后再次在真机 `10.0.0.244:5555` 上执行相同基准测试：
  - 首次冷启动（手机需首次推包）：`1.232s`（较优化前 `2.664s` 降低 `53.8%`）；
  - 首次冷启动（服务重启但手机端已有匹配 Jar，免推）：`888ms`（较优化前 `2.664s` 降低 `66.7%`，提速 `3.0x`）；
  - 同进程重复冷启动（命中内存 SDK+Jar 缓存）：最快 `534ms`、平均 `644~670ms`（较优化前平均 `2.194s` 降低 `70.6%`，提速 `3.4x`）；
  - 60 秒保活窗口内热复用：`< 1ms`（`~100µs`）。
- 完成单元测试、并发单飞测试、`go vet ./...` 与 `go test -race ./internal/ws/... ./internal/scrcpy/...` 验证，更新版本四件套与总览。

## 用户授权提交推送

用户输入「2」，按会话约定授权提交并推送当前 `v1.19.0-feature` 版本代码与文档。
