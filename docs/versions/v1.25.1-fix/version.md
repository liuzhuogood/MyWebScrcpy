# v1.25.1-fix：Windows CI 回归夹具修复与发布

状态：已发布 v1.25.1。

### 核心变更
1. **Windows CI 跨平台可执行测试夹具修复**：修复 `internal/ocr/service_test.go` 中原硬编码 `#!/bin/sh` 的模拟 Python 脚本在 Windows runner 下因缺少可执行后缀及 Shell 解释器导致找不到文件 (`executable file not found`) 的问题，替换为跨平台编译的 Go 二进制夹具，确保 Windows/macOS/Linux 回归测试全绿。
2. **版本对齐与全链路发布**：正式版本推进至 `v1.25.1`，同步前端、桌面 Tauri/Cargo 及 Python SDK 版本。GitHub Actions 全量 Job（Windows NSIS 打包与 Go regression、macOS Intel/arm64 DMG、Linux CLI、Docker 镜像构建与 Python SDK 发布）全部成功。

### 验证证据
- **Git Commit**: `5c61f39` (`fix(ocr): 修复 Windows CI 下 OCR 测试夹具并发布 v1.25.1`)
- **Git Tag**: `v1.25.1`
- **Build and Release**: [Run 38772002243](https://github.com/liuzhuogood/MyWebScrcpy/actions/runs/38772002243) (10/10 Jobs 全部 Success)
- **Publish Python SDK**: [Run 38771975375](https://github.com/liuzhuogood/MyWebScrcpy/actions/runs/38771975375) / [Run 38772001467](https://github.com/liuzhuogood/MyWebScrcpy/actions/runs/38772001467)
- **GitHub Release**: [v1.25.1](https://github.com/liuzhuogood/MyWebScrcpy/releases/tag/v1.25.1) (非草稿，含 9 个多平台二进制程序及 `desktop-latest.json`)
- **Docker Hub**: 多架构镜像 `v1.25.1` 成功构建并推送
- **PyPI**: `mywebscrcpy-sdk 1.25.1` 成功发布并完成干净环境安装校验

### 范围与保护
- 严格隔离未提交的 OCR/服务配置新改动，未将其带入发布。
- 保留 `AGENTS.md` 既有暂存内容未做任何篡改。
- 保留并验证了 `player_windows_test.cjs` 与 `roi_test.cjs`。
