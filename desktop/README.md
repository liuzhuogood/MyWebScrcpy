# MyWebScrcpy 桌面版

Tauri 管理原生窗口、菜单、托盘和单实例，配套 Go 程序承载现有 Vue 与 HTTP/WebSocket。Pi、ADB 与搜索工具不捆绑，scrcpy-server 保留内置配套版本。首页系统配置支持内置依赖按需安装与卸载，也可选择系统程序；Python 沿用已有配置。

## 构建

需要 Go（版本见根目录 go.mod）、Rust 1.90+、Node/pnpm 和 Tauri 对应平台构建依赖。先在项目根目录执行：

```sh
pnpm --dir frontend install --frozen-lockfile
pnpm --dir desktop install --frozen-lockfile
node scripts/build-desktop.mjs
```

输出在 desktop/target/release/bundle/ 和 desktop/target/portable/<系统>-<架构>/。只构建可执行文件：`node scripts/build-desktop.mjs --no-bundle`；已有最新 web/dist 时可加 `--skip-frontend`。构建使用当前本机目标，不假定跨系统 Tauri 可交叉编译。

## 启动模式

便携目录中的 Go 入口：

```sh
./mywebscrcpy            # 有配套外壳和可连接桌面时打开 GUI
./mywebscrcpy --gui      # 强制 GUI，失败报错
./mywebscrcpy --no-gui   # 只运行服务，默认监听 0.0.0.0:8080
./mywebscrcpy --no-gui --listen 127.0.0.1:8080 --data-dir /absolute/runtime/path
```

Windows 使用 mywebscrcpy.exe。macOS 可双击 MyWebScrcpy.app，或运行包内 Contents/MacOS/mywebscrcpy --no-gui。Linux 无图形库的机器必须使用 Go 入口，不直接运行 mywebscrcpy-gui。只有 Go 程序的既有服务分发保持默认服务行为。

Linux 自动检测 Wayland/X11 socket，包括 SSH X11 转发；环境变量存在但 socket 不可连接时运行服务。socket 是预检查，实际图形初始化失败且尚未就绪时自动模式转入服务，显式 --gui 则报错。Linux 托盘取决于桌面和 AppIndicator 支持，不能保证所有桌面显示托盘。

## 窗口和数据

菜单包含设备列表、大屏监控、脚本与任务、依赖管理和浏览器打开。可用托盘下关闭窗口仅隐藏，托盘菜单恢复窗口或退出；没有托盘时关闭窗口退出。退出只停止当前 App 拥有的服务和托管任务。

桌面服务默认监听 127.0.0.1:1989（可在系统配置中修改，或通过 MYWEBSCRCPY_DESKTOP_PORT / PORT 环境变量指定），数据位于系统应用数据目录 com.mywebscrcpy.desktop 下，日志为 desktop.log；就绪文件包含私有控制令牌，不要对外分享，同时生成公开端点文件 desktop-endpoint.json 供外部调用读取。服务端模式的数据目录与工作方式保留。

## 独立依赖

从“依赖管理”查看 ADB/Python 的实际检测结果与官方安装链接。安装后确保程序可被 App 找到，或在启动前指定绝对路径：

```sh
MYWEBSCRCPY_ADB=/absolute/path/adb MYWEBSCRCPY_PYTHON=/absolute/path/python3 ./mywebscrcpy
```

macOS Finder 启动不会继承终端临时环境变量。首页“系统配置 → 依赖”可按需安装 Pi/ADB，或配置系统 Pi、ADB 的绝对路径；fd/rg 自动识别 Pi 工具缓存和 PATH；保存后新操作生效，ADB 忙时拒绝切换。配置保存在桌面数据目录的 dependencies/config.json。既有桌面“依赖管理”入口及 Python 路径设置保留；MYWEBSCRCPY_ADB 在系统来源下仍优先，之后按明确路径、ANDROID_HOME、PATH 查找。Python 脚本库按脚本要求安装。

桌面 App 使用完整安装包更新，不允许沿用服务端单文件替换升级。系统签名、公证和三平台真机视频/音频验证不因本机编译成功而完成。

## 验证

`node scripts/verification/desktop-service.mjs` 验证当前目标配套 Go 程序的回环端口、就绪令牌、依赖配置、升级阻止和受控退出。

需要验证原生 WebView/窗口行为时，运行 `node scripts/build-desktop.mjs --verify-native --no-bundle`，再在启动 GUI 时设置 `MYWEBSCRCPY_NATIVE_REPORT` 为报告的绝对路径。该构建会通过原生菜单处理器进入依赖管理，验证关闭隐藏和恢复，并输出不含设备信息/令牌的 JSON。Go 的 `desktopverify` 与 Rust 的 `native-verify` 都仅存在于明确指定的验证构建；交付前重新运行普通构建，移除验证入口。

## 更新发布

完整更新包、运行方式识别、数据备份、回滚与 Actions 凭据见 [更新文档](../docs/updating.md)。签名更新构建使用 `node scripts/build-desktop.mjs --signed`，签名私钥从环境传入，公钥保存在 `updater.pub`。原生验证构建使用独立应用标识，可运行 `node scripts/verification/desktop-native.mjs`，验证完成后退出 App。

## v1.20.1 Release 下载

Linux 发布 CLI，macOS 发布 CLI 和 Intel/Apple Silicon 两种 GUI DMG，Windows 发布 CLI、独立 GUI exe 和 GUI 安装包。CLI 文件默认只启动服务，`--gui` 请使用 GUI 分发。Windows GUI 内嵌 Go 服务，启动时释放到 `%LOCALAPPDATA%/com.mywebscrcpy.desktop/runtime/<SHA-256>/`，无需旁边的服务 exe；安装包也使用同一个内嵌服务。独立 exe 需要系统 WebView2，安装包会自动安装缺失的 WebView2。

Release 保留 9 个程序与 `desktop-latest.json` 自动更新索引。macOS GUI 安装包为 `mywebscrcpy-v1.20.1-macos-amd64-gui.dmg` 和 `mywebscrcpy-v1.20.1-macos-arm64-gui.dmg`，打开后将 App 拖到 Applications 安装。Windows CLI 统一为 `mywebscrcpy-windows-cli-amd64.exe`，GUI 为 `mywebscrcpy-windows-gui-amd64.exe`，安装包为 `mywebscrcpy-v1.20.1-windows-x86_64-gui-setup.exe`。Linux 不发布 GUI 包。此前 CLI 版本若不能匹配新文件名，请手工下载一次新 CLI，之后使用新名称自动更新。

Windows GUI 安装版使用索引自动更新；独立 GUI exe 使用完整安装包更新，避免安装器更新后仍重新打开旧便携文件。

macOS 使用 DMG 手动更新；本次未增加 `.app.tar.gz` 自动更新资产，索引仍只包含 Windows。DMG 构建包含配套 Go 服务和 scrcpy-server，Pi/ADB 在系统配置中按需安装。Apple 正式签名与公证取决于仓库 Apple 凭据，缺少凭据时使用 ad-hoc 签名。

标签发布会自动调用 `macOS DMG` 工作流构建两种 DMG，Release 和 Docker 等待两个 macOS 架构均验证成功。所有产物使用同一发布标签源码。

## macOS 首次打开

将 DMG 中的 App 拖到“应用程序”后，若提示“Apple could not verify”，是因为当前版本尚未经过 Apple 公证。确认应用来自本项目 [Release](https://github.com/liuzhuogood/MyWebScrcpy/releases) 并信任它后，在终端执行：

```bash
sudo xattr -rd com.apple.quarantine "/Applications/MyWebScrcpy.app"
```
