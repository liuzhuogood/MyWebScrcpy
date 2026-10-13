# v1.20.1 交付记录

2026-10-13，用户要求：“现在打 tag v1.20.1，然后提交推送吧”，并补充“所有改动都提交推送”。

开发分支 `codex/v1.19.5-system-config`，基线 `b5144cf07b8415966991638a7fb51eaab707e43a`，远端 origin 为 `git@github.com:liuzhuogood/MyWebScrcpy.git`。交付前 fetch 后 main 与该基线一致，同名 tag 不存在。

沿用项目 `VERSION` 和 `node scripts/sync-version.mjs` 同步前端、桌面、Cargo 与 Tauri 版本。提交包含全部 Git 可见改动，包括系统配置正式实现、构建链、独立原型与已有 v1.21.0-feature 文档；被忽略的运行依赖缓存、临时服务数据和构建产物不入库。

交付步骤：最终全测通过后提交开发分支；按版本工作流合回 main、复测并提交合并；创建附注标签 `git tag -a v1.20.1`，普通推送 main、开发分支和指定标签，并核对远端 commit/tag。发布工作流由标签自动触发，不将推送成功视为发布包或真实设备验收完成。

本机最终验证：前端 build、独立原型 proto:build、Go 全量测试、dependencies/updater/main race、真实官方 Pi/ADB 归档安装回归、外置 Pi 助手与技能测试、版本同步测试与 Release manifest 契约全部通过。其他平台原生运行及标签远程工作流不由这些本机结果替代。

用户补充要求精简 Release、区分 CLI/GUI，并确认 Windows CLI 统一名称及保留 GUI 自动更新索引。标签工作流改为 5 个 CLI 目标与 Windows GUI；按明确清单组装 7 个程序及 1 个索引，不上传历史 GUI、独立签名及重复 CLI。Windows GUI 服务嵌入外壳并按内容摘要缓存释放，Windows CI 在仅有单个 GUI exe 的目录测试服务启动与退出。完整 UI 原生运行仍需 Windows 验收。

发布改造本机验证通过：5 个 CLI 交叉编译（Linux AMD64/ARM64、macOS AMD64/ARM64、Windows AMD64）；本机 macOS ARM64 CLI 实测默认服务、拒绝 GUI 模式、缺依赖启动与桌面服务生命周期；Release 清单精确包含 7 程序 + 1 索引，并拒绝缺失程序。Rust 缓存释放测试 2 项通过，覆盖损坏修复、不同版本共存和 8 线程并发；Cargo check 使用已安装 SDK26.5 通过。Windows 独立 GUI exe 的原生服务测试在标签 CI 执行，本机不冒充 Windows 验证。

开发提交 `712a56d07b08ffffacdb0ac465e70104b5b45661` 合回 main，合并树与开发提交完全一致，无冲突；主分支合并状态下 Go 全量、前端及原型构建、Rust 缓存测试/clippy、版本同步与发布契约复测通过。补写此记录后再次完整校验通过，随后完成合并提交及附注标签推送。
