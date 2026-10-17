# v1.27.2：修复模板目录监听在 Windows 上关闭时的互锁

本版接续 `v1.27.1`。`v1.27.0` 与 `v1.27.1` 的标签都保留，两次构建均因 Windows 桌面任务的回归测试失败而未生成 Release，本版继续修复并重新发布。

## 问题

- `v1.27.1` 的 `Build and Release` 仍在 `desktop (windows-latest, windows-x86_64, nsis)` 的 `Go regression` 失败，`release` 与 `docker` 任务被跳过，未生成 Release。上一版的技能包编码修复本身已生效（该任务日志中 `internal/skillsmarket` 为 `ok`）。
- 该次失败是 `internal/template` 被 10 分钟测试超时：`panic: test timed out after 10m0s`，停在 `TestTemplateAssetCRUD` 的清理阶段。`Service.Stop` → `Storage.Close` → `DirWatcher.Close` 等待事件循环退出，而事件循环正阻塞在 `fsnotify` 的 `Remove` 上：
  - 原 `Close` 先调用 `watcher.Close()`（关闭 fsnotify 的内部读取协程），之后才 `wg.Wait()`；
  - 若此刻事件循环正在处理目录删除/改名事件并调用 `Remove`，`Remove` 会一直等待内部读取协程应答，而读取协程已被上一步关闭，应答永远不会到达 → 双方互等。
  - `fsnotify` v1.10.1 的 Windows 后端 `Remove` 就是"投递请求 + `return <-in.reply`"（`backend_windows.go:166`），关闭之后没有协程再进行应答。
- 该竞态自 `59a6782`（2026-09-15，模板全自动热重载）就已存在，属既有缺陷：Windows 上只要关闭时恰有目录增删事件在途就会挂住，同时意味着程序退出时可能不再返回。macOS/Linux 的 fsnotify 后端不走这条应答通道，因此本地与其它平台不复现。

## 修复

1. `internal/template/watcher.go` 的 `Close` 调整顺序：先 `close(done)` 让事件循环退出，`wg.Wait()` 确认其结束，最后才 `watcher.Close()`，保证关闭 fsnotify 时没有 Add/Remove 在途。
2. `removeWatch` 增加存在性判断，只对登记过的目录调用 `fsnotify.Remove`，去掉文件增删触发的无关阻塞调用。
3. 新增 `TestDirWatcherCloseWhileEventsPending`：在仍有待处理事件（子目录创建、写入、删除）时关闭监听，要求 5 秒内返回。

## 本地验证

- [x] `go test ./internal/template/ -count=5 -race` 通过，含新增的关闭场景用例与既有热重载用例。
- [x] `go build ./...`、`go test ./... -count=1` 全量通过；`gofmt -l internal/template` 无输出。
- [ ] 未在 Windows 原生环境复现该互锁后再验证：本地无 Windows，且缺陷只出现在 fsnotify 的 Windows 后端。本版结论依据是 fsnotify v1.10.1 `backend_windows.go` 中 `Remove`（166 行 `return <-in.reply`）与 `Close` 的交互分析，确定性结论以 `v1.27.2` 的 Windows 桌面任务结果为准。

## 未验证范围

- 修复的 Windows 实机验证依赖推送后的 CI；同样未在原生 Windows 手动制造"关闭时目录事件在途"的场景。
- 仓库内其他技能目录（`skills/kuaishou-short-video`、`skills/my-scrcpy-use`、`skills/dingtalk-punch`）的 Python 入口仍未统一 UTF-8 输出，与 v1.27.1 记录一致，本版未改动。

## 发布状态

- `VERSION` 已同步为 `v1.27.2`，标签 `v1.27.2` 指向本次发布提交；推送后由 GitHub Actions 触发多平台 CLI、Windows GUI、macOS DMG 与 Docker 镜像的构建与发布。
- `v1.27.0` 与 `v1.27.1` 标签保留，但都没有 Release；`v1.26.1` 已发布。
