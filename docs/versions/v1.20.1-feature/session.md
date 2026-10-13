# v1.20.1 交付记录

2026-10-13，用户要求：“现在打 tag v1.20.1，然后提交推送吧”，并补充“所有改动都提交推送”。

开发分支 `codex/v1.19.5-system-config`，基线 `b5144cf07b8415966991638a7fb51eaab707e43a`，远端 origin 为 `git@github.com:liuzhuogood/MyWebScrcpy.git`。交付前 fetch 后 main 与该基线一致，同名 tag 不存在。

沿用项目 `VERSION` 和 `node scripts/sync-version.mjs` 同步前端、桌面、Cargo 与 Tauri 版本。提交包含全部 Git 可见改动，包括系统配置正式实现、构建链、独立原型与已有 v1.21.0-feature 文档；被忽略的运行依赖缓存、临时服务数据和构建产物不入库。

交付步骤：最终全测通过后提交开发分支；按版本工作流合回 main、复测并提交合并；创建附注标签 `git tag -a v1.20.1`，普通推送 main、开发分支和指定标签，并核对远端 commit/tag。发布工作流由标签自动触发，不将推送成功视为发布包或真实设备验收完成。

本机最终验证：前端 build、独立原型 proto:build、Go 全量测试、dependencies/updater/main race、真实官方 Pi/ADB 归档安装回归、外置 Pi 助手与技能测试、版本同步测试与 Release manifest 契约全部通过。其他平台原生运行及标签远程工作流不由这些本机结果替代。

用户补充要求精简 Release、区分 CLI/GUI，并确认 Windows CLI 统一名称及保留 GUI 自动更新索引。标签工作流改为 5 个 CLI 目标与 Windows GUI；按明确清单组装 7 个程序及 1 个索引，不上传历史 GUI、独立签名及重复 CLI。Windows GUI 服务嵌入外壳并按内容摘要缓存释放，Windows CI 在仅有单个 GUI exe 的目录测试服务启动与退出。完整 UI 原生运行仍需 Windows 验收。

发布改造本机验证通过：5 个 CLI 交叉编译（Linux AMD64/ARM64、macOS AMD64/ARM64、Windows AMD64）；本机 macOS ARM64 CLI 实测默认服务、拒绝 GUI 模式、缺依赖启动与桌面服务生命周期；Release 清单精确包含 7 程序 + 1 索引，并拒绝缺失程序。Rust 缓存释放测试 2 项通过，覆盖损坏修复、不同版本共存和 8 线程并发；Cargo check 使用已安装 SDK26.5 通过。Windows 独立 GUI exe 的原生服务测试在标签 CI 执行，本机不冒充 Windows 验证。

开发提交 `712a56d07b08ffffacdb0ac465e70104b5b45661` 合回 main，合并树与开发提交完全一致，无冲突；主分支合并状态下 Go 全量、前端及原型构建、Rust 缓存测试/clippy、版本同步与发布契约复测通过。补写此记录后再次完整校验通过，随后完成合并提交及附注标签推送。

## 发布失败修复

首次标签运行 38343073747：5 个 CLI 构建、5 个外置 Pi 原生回归、Windows 任务回归、Docker 推送均成功；Windows GUI 安装包构建、签名验证、Go 全量回归、服务缓存测试、独立 GUI 服务运行也成功。桌面服务生命周期脚本在 Windows 启动后立即改 ADB 路径，后台设备轮询持有 ADB 租约，返回 409，而脚本错误地期待相对路径校验的 400，导致 Release 被阻止。

修复验证脚本：指定临时目录中的缺失 ADB 路径，隔离 runner 的真实 ADB；仅对明确“依赖正在使用，请稍后重试”的 409 做 5 秒有界等待，其他响应仍按原断言严格检查，保留 400/204 及认证、受控关闭验收。

用户要求精简截图中的测试作业：删除发布流程的独立 Pi 五平台矩阵与重复 Windows 任务作业，保留 Windows GUI 作业内 Go 全量回归（覆盖任务、Markdown 和调度）及独立 GUI 的原生服务验证。Release 与 Docker 都等待 CLI 和 Windows 桌面作业成功。外置 Pi 专项仍可本机执行，桌面独立 CI 保留相关回归。

修复后本机验收：桌面服务脚本并行启动 10 次全通过；Go 全量、前端/原型构建、Rust 服务缓存测试及 clippy、版本同步与发布清单契约通过。工作流结构检查确认只剩 5 目标 CLI、Windows 桌面、Release 与 Docker，后两者均等待前两项成功。
