# 系统配置

首页顶端进入“系统配置”，包含“主题”和“依赖”两页签。

主题提供系统、明亮、黑暗，立即生效并保存在当前客户端；系统模式跟随浏览器的系统配色偏好。

| 依赖 | 内置来源 | 系统来源 |
| --- | --- | --- |
| scrcpy-server.jar | 随应用内嵌 4.0 | 不提供切换或卸载 |
| pi-agent | 按需安装 Pi 0.86.1 / fd 10.3.0 / ripgrep 15.2.0 | 自动检测 pi，找不到时填写执行路径 |
| ADB | 官方 Platform-Tools 36.0.2，安装前确认 Android SDK 许可 | 保留 MYWEBSCRCPY_ADB、明确配置、ANDROID_HOME、PATH 优先顺序 |

内置依赖文件位于数据根目录的 `dependencies/pi-agent/` 和 `dependencies/adb/`。服务使用 `--data-dir /absolute/path`，桌面沿用其用户数据目录。下载地址、版本和 SHA-256 由程序内清单限定，用户不能提交任意安装 URL。下载校验和解包成功后原子发布；关闭弹窗不会取消任务，安装成功不修改已选来源。

Pi 内置安装覆盖 Windows AMD64、macOS AMD64/ARM64、Linux AMD64/ARM64。ADB 覆盖 Windows AMD64、macOS AMD64/ARM64、Linux AMD64；Linux ARM64 使用系统依赖。

“系统”会在后端进程的 PATH 中查找执行程序，相当于 `which pi`；随后运行 `pi --help` 并检查所需 RPC 能力。手动绝对路径优先于 PATH；ADB 既有环境变量覆盖仍优先于手动路径。错误路径不会回退到内置副本。Pi 使用的 fd/rg 由后台按 Pi 的工具目录（PI_CODING_AGENT_DIR 或 ~/.pi/agent 下的 bin）与 PATH 自动查找，界面不要求分别配置；缺少必需的搜索工具时检测会说明原因。保存前检查程序版本和所需能力，原子写入 `dependencies/config.json`，新操作使用新配置。ADB 来源变更需等待投屏、后台会话、设备命令与任务空闲。

卸载只删除本应用管理的文件，不删除系统程序、用户技能、脚本、模型设置或任务。使用中拒绝卸载；卸载后保留来源选择，内置来源需重新安装。删除失败保留清理状态，可重试。

## 配置入口

配置弹窗无需令牌或解锁。服务模式直接提供依赖配置，不检查访问地址、Host、Origin 或 Sec-Fetch-Site；桌面沿用自动完成的既有内部控制。

既有 `/dependencies` 页面、桌面菜单及 Python 设置继续保留，旧 `dependencies.json` 的明确 adb/python 字段照常读取。首次新配置保留旧 ADB 路径与系统查找顺序。旧 `.pi/runtime/` 不自动删除或猜测登记，可通过明确执行路径继续使用。

## 开发验证

```sh
pnpm --dir frontend build
go test ./...
python3 scripts/prepare-pi.py
MYWEBSCRCPY_PI_TEST_PAYLOAD="$PWD/.cache/pi-test" go test -count=1 ./internal/pibundle ./internal/aiprovider
MYWEBSCRCPY_PI_ARCHIVE_CACHE="$PWD/.cache/pi" go test -count=1 ./internal/dependencies
```

主程序/桌面构建不下载 Pi；测试资源不会被 go:embed 嵌入产物。
