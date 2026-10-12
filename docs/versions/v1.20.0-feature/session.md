# v1.20.0-feature 会话记录

## 2026-10-12

**用户**：评估 Windows、Linux 和 macOS 编译程序是否只能命令行启动，能否打包成普通桌面 App。

**Codex**：当前为 Go HTTP 服务，Vue 资源和 scrcpy-server 已嵌入，未提供桌面外壳。

**用户**：默认启动 GUI，加 noGUI 参数时只启动服务。

**用户**：ADB、scrcpy/JAR、Python 和 Pi AI / Pi Agent 可以做成独立依赖，让用户后续安装，以减小 App 体积。

**用户**：不采用较重的 Electron，选择 T 开头的轻量框架。

**Codex**：确定采用 Tauri 系统 WebView，并保留现有 Go 后端；需要实际验证 WebCodecs。

**用户**：Linux 是否能自动判断 GUI 环境。

**Codex**：由 Go 入口判断当前可连接的桌面会话，无桌面时只运行服务，避免加载 Tauri 图形库。

**用户**：帮我创建一个新的版本事件号，然后开始做这个事情吧。要有著名的，著名的GUI，菜单，菜单，头盘，头盘菜单。

**Codex**：按语音上下文解释为桌面 GUI、原生菜单、系统托盘及托盘菜单，建立 v1.20.0-feature 并开始实施。当前工作区干净，正式版本为 v1.19.2。

## 已核实的实现约束

- 原服务与发布流程保留，新桌面构建另设入口；不删除历史逻辑。
- 当前 Pi Engine 是内置 Markdown 编译器名称，代码没有调用外部 Pi Agent；不编造外部包名或安装命令。
- 当前 ADB、Python 并未捆绑；scrcpy-server 保留内置版本，外置资源必须匹配协议版本。
- 本机有 Rust/Go，前端依赖已安装；全局 pnpm 包管理器损坏，使用 Codex 自带 pnpm。cloudflared 当前不在 PATH，公网原型交付条件待核实。

## 本地实施与验证结果

- Go 入口新增 --gui、--no-gui、--print-mode、--listen、--data-dir；Linux 探测 Wayland/X11 可连接会话。只含 Go 的原分发默认服务行为保留。
- Tauri 外壳与配套 Go 程序，原生菜单、系统托盘、单实例、关闭隐藏及恢复；Linux 仅在标准托盘宿主报告可用时启用关闭隐藏，避免隐藏后无法恢复。
- 桌面服务采用 127.0.0.1 随机端口、私有令牌、应用数据目录与 stdin 父进程退出检测。退出清理自有任务、录制、投屏；既有服务模式生命周期保留。
- ADB/Python 检测、官方安装链接、绝对路径配置与重启生效；未安装依赖不被捆绑或猜测替代。scrcpy-server 仍内置。
- 修复桌面导航发现的 /scripts、/dependencies 服务端 SPA 白名单遗漏；保留其他 API/资源的 404 行为。
- 桌面阻止单文件自更新，新增三平台构建工作流，仅生成检查/安装产物，不自动发 Release。
- Go 全量测试、全量 race、go vet、前端正式构建、原型构建/检查、Rust check/clippy（含验证 feature）通过。Go Windows amd64、Linux amd64/arm64、macOS amd64/arm64 编译通过。
- 后台集成验证：回环端口、私有 health/shutdown、依赖配置鉴权及错误路径、SPA 页面、桌面升级阻止、正常退出均通过。GUI 父进程退出后配套 Go 服务退出；重复 GUI 启动没有第二个实例。
- 原生 WebView 验证构建通过菜单处理器进入依赖管理，确认真实 DOM 挂载、正确标题、窗口关闭隐藏及恢复；VideoDecoder/AudioDecoder 接口存在，不等于真机解码/音频通过。
- 本机缺少辅助功能权限；System Events 菜单查询未完成，停止了本次查询进程。窗口截图只得到空白/黑色，未作为正式界面通过证据保留。原型实拍和原生 DOM 报告分开记录，不把 Chrome 验证冒充 WKWebView 验证。
- 验证逻辑用 Go desktopverify 和 Rust native-verify 双重构建开关隔离，普通构建已重新生成并确认不存在原生报告入口。未改正式 VERSION，未创建标签、提交推送或部署远端。
- Rust 首次下载官方索引超时，本机使用已忽略的 desktop/.cargo/config.toml 临时镜像配置；Cargo.lock 保留包校验和，不把本机网络配置带入仓库。

## 提交授权

**用户**：提交推送；所有变动提交推送。

**Codex**：提交当前全部未忽略变动，推送 main；保留未验收项，不创建发布标签。提交前复测 Go 全量测试/vet、前端构建/原型检查、Rust clippy。
