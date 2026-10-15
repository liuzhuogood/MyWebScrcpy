# v1.25.1-fix：Windows CI 回归夹具修复与发布

状态：等待 Actions 发布。

### 核心变更
1. **Windows CI 跨平台可执行测试夹具修复**：修复 `internal/ocr/service_test.go` 中原硬编码 `#!/bin/sh` 的模拟 Python 脚本在 Windows runner 下因缺少可执行后缀及 Shell 解释器导致找不到文件 (`executable file not found`) 的问题，替换为跨平台编译的 Go 二进制夹具，确保 Windows/macOS/Linux 回归测试全绿。
2. **版本对齐**：正式版本推进至 `v1.25.1`，同步前端、桌面 Tauri/Cargo 及 Python SDK 版本。

### 范围与保护
- 严禁包含未完成且未授权的 OCR/匹配服务新功能实现。
- 严禁篡改 `AGENTS.md` 已有暂存区内容。
- 保留 `player_windows_test.cjs` 与 `roi_test.cjs` 测试。
