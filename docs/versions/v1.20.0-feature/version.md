# v1.20.0-feature：轻量桌面 App（本地实施完成，跨平台验收中）

1. 新增 Tauri 原生窗口、菜单、系统托盘及托盘菜单，支持单实例和窗口恢复。
2. 桌面分发默认 GUI，支持 --gui / --no-gui；Linux 自动检测可连接图形会话，原服务分发保留。
3. 桌面后台使用回环随机端口、私有控制令牌、独立用户数据目录，退出清理自有进程和投屏会话。
4. ADB/Python 独立安装，新增依赖检测、官方安装入口与路径配置；保留内置 scrcpy-server，外部 Pi Agent 待确认。
5. 增加桌面构建与三平台 CI 配置，修复 /scripts /dependencies 的直接页面导航。

本地 Go 全量/race/vet、前端与原型构建、Rust check/clippy、后台集成与 macOS 原生 WebView/窗口生命周期验证通过。macOS arm64 App 已生成，约 25 MiB。Windows/Linux 原生 GUI/安装、macOS 物理菜单和视觉验收及真机投屏/音频仍待验收。正式版本、标签和部署未变更。
