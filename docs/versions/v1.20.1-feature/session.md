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

第二次运行 38347272584：5 个 CLI、Windows 安装包和签名校验成功；Windows Go 全量回归暴露既有多设备测试的 3 秒 Python 启动期限过短。测试改为共享 15 秒启动期限，读取设备参数标记并检查任务仍运行，提前退出立即失败；测试脚本保持运行直至显式停止，独立停止不受 Python 启动耗时及 10 秒自然结束影响。60 秒任务超时及测试清理仍限制生命周期。用户已明确授权更新 v1.20.1 标签并持续修复发布。

## 发布验收完成

第三次运行 [38348468105](https://github.com/liuzhuogood/MyWebScrcpy/actions/runs/38348468105) 全部成功，发布源码与标签提交为 `4c3cf5f582d5845af842d250c5fc44d49a3d355b`。5 个 CLI 目标、Windows 桌面、Release 和 Docker 共 8 个作业均成功。Windows Go 全量回归、多设备启动/独立停止重复 3 次、服务缓存、独立 GUI 服务启动/退出、桌面生命周期、发布清单和更新签名校验全部通过。

[Release v1.20.1](https://github.com/liuzhuogood/MyWebScrcpy/releases/tag/v1.20.1) 实际包含约定的 7 个程序与 `desktop-latest.json`，无重复 CLI、macOS/Linux GUI 或独立签名资产。全部 8 个文件已真实下载，大小及 SHA-256 与 GitHub 资产 digest 一致；5 个 CLI 的 Go 构建信息均指向上述源码提交。更新索引只包含 Windows，指向同版本 GUI 安装包；独立验签通过且篡改安装包被拒绝。发布的 macOS ARM64 CLI 实测默认服务、GUI 模式拒绝、缺依赖启动、配置及受控退出通过。

Docker Hub 的 `liuzhuogood/mywebscrcpy:v1.20.1` 和 `latest` 均可读取 Linux AMD64 清单，镜像摘要一致：`sha256:3c5cf027a17ca9e86f0a78937041cf1ffecb193557e8a22397bd01e994f44e3d`。本轮核对构建、推送与远端清单，不包含本机 Docker 容器运行或真实手机验收。Windows GUI 验证包含独立 exe 的内嵌服务运行，不包含窗口/手机交互验收。

原 v1.20.1 标签更新已获用户明确授权，使用精确旧标签对象的 force-with-lease 推送并核对远端。最终验收记录在发布后补写，仅普通推送 main，标签继续保留在已通过 CI 的源码提交，不因文档收尾再次触发发布。

## macOS DMG 补充

用户追加要求增加 macOS DMG。下载清单增加 Intel AMD64 和 Apple Silicon ARM64 两个 GUI 安装包，共 9 个程序与原 Windows 更新索引。采用 Tauri 原生 DMG 打包，原生 macOS runner 分别构建，并验证桌面服务、挂载磁盘镜像、App 版本、GUI/Go 服务架构和代码签名。未增加 macOS 自动更新归档，DMG 使用手动安装更新。

新增可复用 `macOS DMG` 工作流，正式 Release 与 Docker 等待两种 DMG 验证成功。用户明确要求在 GitHub Actions 中加入两个产物后重新打标签发布，因此本次提交推送后更新 v1.20.1 标签，完整重新构建，所有产物使用同一标签源码。远程构建与实际资产验收结果待构建完成后补充。

同时核查用户询问的目录清理：根目录无 `piundle`，`internal/pibundle` 仍被技能安装、独立依赖解压校验与测试引用，不能整目录删除。可再生成的旧 payload、Pi 下载/测试缓存、桌面编译目标及生成的 sidecar 可清理；数据库、已安装依赖、技能与源码保留。本次未执行删除。
