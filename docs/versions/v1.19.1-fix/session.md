# v1.19.1-fix 会话记录

## 2026-10-12

**用户**：

> https://github.com/liuzhuogood/MyWebScrcpy/actions/runs/38154146156
>
> 看一下，我编译报错了，请你帮我解决，重新 github 编译吧。

**Codex**：

- GitHub Actions 的 Windows 构建报错：任务管理器直接使用 Unix 专属的 `syscall.SysProcAttr.Setpgid` 和 `syscall.Kill`。前端构建已通过；其他平台部分任务被矩阵提前取消，Release 和 Docker 被跳过。
- 本地运行 `GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build` 及任务管理器测试交叉编译，复现相同错误，作为修复前红灯。
- 按平台拆分进程管理：macOS/Linux 保留进程组终止与已登记 PID 清理流程；Windows 通过系统 `taskkill /PID <pid> /T /F` 终止任务与子进程，调用限时 5 秒并记录失败。
- 增加手动停止进程树、设备锁释放和另一设备任务隔离的回归用例；现有超时测试改为各平台原生进程状态检查。
- 增加 GitHub Windows 原生测试任务，覆盖任务管理、Markdown 执行和调度，测试通过后才允许 Release 和 Docker 发布。
- 全量回归复现已有回放用例的时序断言问题：订阅发生在初始化缓存建立前时，合法的会话帧会先于配置帧送达。测试允许这个会话帧，仍严格检查配置帧、关键帧顺序与回放标志；回放业务逻辑保持原样。
- 工作流静态检查发现 Release Action v1 使用过期运行时；核对官方 v3 的输入与 Node 24 运行时后升级两个发布分支。
- 使用补丁版本 `v1.19.1` 重新触发发布构建，保留原 `v1.19.0` 标签。正式版本同步至 `VERSION`、前端包和 OpenAPI。

## 验证进度

- ✅ 修复后 Windows 程序与任务管理器测试交叉编译通过。
- ✅ 本地任务管理、Markdown 执行与调度测试通过。
- ✅ `go test -count=1 ./...`、`go vet ./...`、任务管理/Markdown/调度/回放的 race 检查通过。
- ✅ 回放状态机用例连续执行 100 次通过；新增手动进程树终止覆盖进程表与已登记 PID 两条路径。
- ✅ 五个平台交叉编译通过：Linux amd64/arm64、macOS amd64/arm64、Windows amd64。
- ✅ 锁定依赖安装、前端生产构建、17 个前端测试、原型类型检查通过。
- ✅ 独立 Python 模板匹配测试通过，使用已有 `yolo_wepork/.venv` 的依赖；默认 Python 未安装 OpenCV。
- ✅ Release Action 更新后的 `actionlint` 工作流静态检查、Go 完整回归与版本一致性检查通过。
- ⬜ 提交并推送 `v1.19.1` 标签，核对 GitHub Actions、Release 文件与 Docker 结果。
