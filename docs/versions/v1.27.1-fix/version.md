# v1.27.1：修复技能包请求脚本在 Windows 默认代码页下的中文输出异常

本版接续 `v1.27.0`。`v1.27.0` 标签已推送，但构建失败、未生成 GitHub Release，本版为同一批功能的修复发布。

## 问题

- `v1.27.0` 的 `Build and Release` 在 `desktop (windows-latest, windows-x86_64, nsis)` 任务失败，失败步骤为 `Go regression`；`release` 与 `docker` 任务因依赖该任务被跳过，因此没有生成 `v1.27.0` Release（标签本身保留）。
- 唯一失败用例是 `internal/skillsmarket.TestExtractedZipPythonClientExecutable`：测试解压技能包后执行 `scripts/request_client.py --help`，Windows 控制台默认代码页 `cp1252` 无法编码帮助文本中的中文，`argparse` 打印帮助时抛 `UnicodeEncodeError: 'charmap' codec can't encode characters`，脚本以退出码 1 结束。

## 修复

1. 三个技能包（`mywebscrcpy-device-base`、`mywebscrcpy-kuaishou-daily`、`mywebscrcpy-wechat-helper`）的 `scripts/request_client.py` 在 CLI 入口把标准输出与标准错误重配为 UTF-8，与 `v1.19.2` 对生成脚本的既有处理一致；三份脚本保持内容完全一致。
2. `TestExtractedZipPythonClientExecutable` 为被执行的子进程固定 `PYTHONIOENCODING=cp1252`，使该编码缺陷在非 Windows 平台也能复现，防止回归。

## 本地验证

- [x] 修复前复现：`PYTHONIOENCODING=cp1252 python3 scripts/request_client.py --help` 退出码 1，报 `UnicodeEncodeError: 'charmap' codec`，与 CI 失败位置一致。
- [x] 修复后同一命令退出码 0，帮助文本中的英文标识与中文说明均完整输出。
- [x] `go test ./... -count=1` 全量通过；`go test ./internal/skillsmarket/ -count=1 -run TestExtractedZipPythonClientExecutable -v` 通过。
- [x] 三份 `request_client.py` 内容一致（同一 MD5 `a23227ab90587412976a601272e8e383`），`py_compile` 通过。
- [x] `gofmt -l internal/skillsmarket` 仅剩本版之前就已存在的 `types.go`，与本次改动无关。

## 未验证范围

- 未在原生 Windows 控制台人工执行脚本，只固定 `cp1252` 复现同一编码路径；Linux/macOS 与 Windows 各平台由推送后的 Actions 重新构建验证。
- 仓库内其他技能目录（`skills/kuaishou-short-video`、`skills/my-scrcpy-use`、`skills/dingtalk-punch`）的 Python 入口尚未统一 UTF-8 输出，在 cp1252 控制台下存在同类风险，本版未改动。

## 发布状态

- `VERSION` 已同步为 `v1.27.1`，标签 `v1.27.1` 已推送；本版的技能包编码修复在 Windows 上生效（该任务日志中 `internal/skillsmarket` 为 `ok`）。
- 但同一次 Windows 桌面任务里 `internal/template` 因既有的 fsnotify 互锁被 10 分钟超时，`release` 与 `docker` 被跳过，**未生成 Release**；标签保留，转 [v1.27.2-fix](../v1.27.2-fix/version.md)。
- `v1.27.0` 标签保留但无 Release；`v1.26.1` 已发布。
