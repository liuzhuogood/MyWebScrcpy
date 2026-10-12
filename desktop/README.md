# MyWebScrcpy 桌面版

Tauri 管理原生窗口、菜单、托盘和单实例，配套 Go 程序承载现有 Vue 与 HTTP/WebSocket。ADB、Python 不捆绑，scrcpy-server 保留内置配套版本。当前外部 Pi Agent 尚未接入。

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

桌面服务监听 127.0.0.1 的动态端口，数据位于系统应用数据目录 com.mywebscrcpy.desktop 下，日志为 desktop.log；就绪文件包含私有控制令牌，不要对外分享。服务端模式的数据目录与工作方式保留。

## 独立依赖

从“依赖管理”查看 ADB/Python 的实际检测结果与官方安装链接。安装后确保程序可被 App 找到，或在启动前指定绝对路径：

```sh
MYWEBSCRCPY_ADB=/absolute/path/adb MYWEBSCRCPY_PYTHON=/absolute/path/python3 ./mywebscrcpy
```

macOS Finder 启动不会继承终端中临时设置的环境变量；可在桌面“依赖管理”中填写 ADB/Python 可执行文件的绝对路径，保存后重启 App。配置保存在桌面数据目录的 dependencies.json；显式环境变量优先于已保存路径。Python 脚本库按脚本自身要求在专用环境中安装，App 不执行未经确认的 Pi 安装命令。

桌面 App 使用完整安装包更新，不允许沿用服务端单文件替换升级。系统签名、公证和三平台真机视频/音频验证不因本机编译成功而完成。

## 验证

`node scripts/verification/desktop-service.mjs` 验证当前目标配套 Go 程序的回环端口、就绪令牌、依赖配置、升级阻止和受控退出。

需要验证原生 WebView/窗口行为时，运行 `node scripts/build-desktop.mjs --verify-native --no-bundle`，再在启动 GUI 时设置 `MYWEBSCRCPY_NATIVE_REPORT` 为报告的绝对路径。该构建会通过原生菜单处理器进入依赖管理，验证关闭隐藏和恢复，并输出不含设备信息/令牌的 JSON。Go 的 `desktopverify` 与 Rust 的 `native-verify` 都仅存在于明确指定的验证构建；交付前重新运行普通构建，移除验证入口。

## 更新发布

完整更新包、运行方式识别、数据备份、回滚与 Actions 凭据见 [更新文档](../docs/updating.md)。签名更新构建使用 `node scripts/build-desktop.mjs --signed`，签名私钥从环境传入，公钥保存在 `updater.pub`。原生验证构建使用独立应用标识，可运行 `node scripts/verification/desktop-native.mjs`，验证完成后退出 App。
