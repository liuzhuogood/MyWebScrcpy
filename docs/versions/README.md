# 版本状态总览

2026-10-20 规划补充：当前源码基线 `ec0019b`、正式 `VERSION` 为 `v1.29.0`；新增 [v1.32 首页手机视图与批量操作规划](v1.32.0-feature/design.md)。下方原核对说明与历史状态保留为当时记录，不作为当前发版/部署证明。

核对日期：2026-10-09。覆盖主项目 21 个版本记录与 Vue 前端 7 个迁移版本。

## 状态说明

- **已完成**：本版实施已有代码、任务或会话证据；不表示所有补充验收、GitHub Release 或 Actions 均已完成。
- **待验收**：实施已完成，仍缺指定设备、浏览器或专项场景的验证。
- **延期**：原记录明确推迟、可选未做或尚未安排，不与本版核心实现混为一谈。
- **已被后续版本替代**：历史功能、入口或决定已由后续版本修正或移除，原勾选只表示当时实施。
- **待实施**：仅有方案或文档，当前未完成代码实现。

版本目录名、提交标题、正式 `VERSION` 和部署状态分别核对。当前正式 `VERSION` 为 `v1.27.2`；`v1.26.1` 已发布，`v1.27.0`、`v1.27.1` 标签保留但都未生成 Release（构建被 Windows 回归测试拦截）。v1.12 于 2026-10-09 调整为 Web 端重构；旧 `v1.12` 提交只有原方案文档；本次重构发布使用 `v1.12.0` 标签，提交和远端指向以 Git 记录为准。

## 主项目

| 版本 | 当前状态 | 范围与证据 |
| --- | --- | --- |
| [v1.0.0-feature](v1.0.0-feature/version.md) | 已完成；补充验收待验收 | 首版核心功能已发布。原任务中的未来计划不属于本版未完成项；逐浏览器、15 FPS 与长时间稳定性清单缺少逐项证据，保留待验收。 |
| [v1.1.0-feature](v1.1.0-feature/version.md) | 已完成 | 大屏实施、验证和发布在原记录中已完成。旧 HTML 入口已由 Vue `/dashboard` 替代，历史文件路径只用于追溯。 |
| [v1.2.0](v1.2.0/version.md) | 已完成；部分延期/已替代 | 脚本管理实施已完成，提交为 `b50cd07`。可选并发保护延期；旧脚本页面已随 Vue 替换移除，当前保留后端脚本接口，无新版页面。未找到 `v1.2.0` 标签，不将该版单独发布标为完成。 |
| [v1.3.0-feature](v1.3.0-feature/version.md) | 已完成；已被后续修正 | 文件管理已实现并提交，正式版本及远端 `v1.3.0` 标签已确认。v1.3.1 将管理对象改为手机文件并移除令牌；本版服务端本地文件和鉴权设计属于历史实现。 |
| [v1.3.1-fix](v1.3.1-fix/version.md) | 已完成 | 手机文件管理修正已完成。横竖屏仍受前台应用方向策略限制，原记录中的验证边界保留；旧弹窗实现已迁移至 Vue。 |
| [v1.3.2-fix](v1.3.2-fix/version.md) | 已完成 | README 对齐和播放器恢复修复已完成；旧前端文件路径保留为历史记录。 |
| [v1.4.0-feature](v1.4.0-feature/version.md) | 已完成；部分延期 | 共享会话、Vision 和 ActionQueue 的本版实施已收口。原 `[~]` 项为延期范围：鉴权、互斥租约、尺寸/质量协商、Go JPEG 推流对比及持续链路指标等，不能勾为已实现。 |
| [v1.4.1-fix](v1.4.1-fix/version.md) | 已完成 | Vision 超时修正与说明已完成；旧原始尺寸窗口后来由显示尺寸功能替代。 |
| [v1.4.2-fix](v1.4.2-fix/version.md) | 已完成 | 共享订阅关键帧保留修复及回归测试已完成。 |
| [v1.5.0-feature](v1.5.0-feature/version.md) | 已完成 | 录屏、ADB 命令、组合键和区域截图已实现、验证并部署。正式版本及远端 `v1.5.0` 标签已确认，原待发布条目已补齐。历史播放器验证不替代新版 Vue 的 v0.7 真机验收。 |
| [v1.5.1-feature](v1.5.1-feature/version.md) | 已完成 | 显示尺寸、入口和录制目录修复已完成并有提交与部署记录。显示尺寸移入设置属于原 v1.12 方案，现已延期。 |
| [v1.5.2-feature](v1.5.2-feature/version.md) | 已实现；待验收 | 检查器实施、提交和部署已完成。空/不完整 XML、XPath 多命中、旋转及坐标映射仍按原记录保留专项回归，不能以构建通过代替。 |
| [v1.5.3-feature](v1.5.3-feature/version.md) | 已实现；待验收 | 设备音频和有声录制已实现、部署并完成已记录的 AAC/MP4 验证。人工听感、Android 11 解锁后启动、断流重连与更多浏览器覆盖仍待验收。 |
| [v1.5.4-feature](v1.5.4-feature/version.md) | 已完成 | 属性 XPath 自动测试、画中画及坐标读数实施已完成。浏览器画中画能力仍按原实现的运行条件使用。 |
| [v1.6.0-feature](v1.6.0-feature/version.md) | 已完成 | API 文档与技能提示词已完成，提交 `86766ae`，正式版本及远端 `v1.6.0` 标签已确认。原「待提交与标签发布」已过时。 |
| [v1.7.0-feature](v1.7.0-feature/version.md) | 已完成 | 页面信息接口与检查器展示已完成，原任务清单已勾选；当前 Vue 接入对应能力。 |
| [v1.8.0-feature](v1.8.0-feature/version.md) | 已被后续版本替代（匹配部分） | 模板管理与匹配曾实现。v1.11 明确移除内嵌匹配、检测、匹配状态及结果接口，保留模板资产管理；本版匹配任务是历史完成记录，不作为恢复功能的待办。 |
| [v1.9.0-feature](v1.9.0-feature/version.md) | 已完成；部分已被替代 | 动作 `mode`/`duration_ms` 与 ADB 执行能力保留。模板自动定位点击部分已由 v1.11 移除，相应任务、接口和测试描述仅作历史记录。 |
| [v1.10.0-feature](v1.10.0-feature/version.md) | 已实现；专项交互待验收 | 终端实施、提交和部署已有证据，远端 `v1.10.0` 标签已确认。原全空清单已更新；Ctrl+C、Tab、方向键、进程回收和 resize 的逐项真机验收缺少明确结果，保留待验收。旧 HTML/CDN 实现已由 Vue 与本地 xterm 依赖替代。 |
| [v1.11.0-feature](v1.11.0-feature/version.md) | 已实现并部署；待验收/待同步 | 后端匹配移除已提交为 `9cef468`，当前 10.0.0.6 部署包含该改动。移除前后坏帧/CPU/帧连续性对比尚未验收；合入的 Vue 仍保留 matches/click 调用及匹配控件，需按本版仅管理模板的决定同步。 |
| [v1.12.0-feature](v1.12.0-feature/version.md) | Web 端重构主体已实现并部署；遗留待处理 | Vue 已并入主仓库，直接替换旧页面，统一构建并部署。三项遗留：分割线占不满、图标未统一、模板匹配不可用。原显示尺寸、边框与录制反馈方案已延期；正式版本已更新为 `v1.12.0`，发布使用同名标签；工作流验收单独记录。 |
| [v1.13.0-feature](v1.13.0-feature/version.md) | 已实现 | 自动更新机制：GitHub Release 检查、多平台二进制匹配、SSE 进度流推流、原地替换与前端下载进度展示。 |
| [v1.14.0-feature](v1.14.0-feature/version.md) | 已实现并验证 | 引入 SQLite 数据库持久化手动添加设备；自动 ADB 重连守护；区分展示「自动发现」与「手动添加」；前台添加/重连/删除设备。 |
| [v1.15.0-feature](v1.15.0-feature/version.md) | 已实现并验证 | 独立模板匹配工程（Python + PyAV + OpenCV）与实时绿框展示；解耦主服务投屏流，HTTP API 在线同步模板，前端 Canvas 订阅 WebSocket 广播实时渲染目标绿框与置信度。 |
| [v1.16.0-feature](v1.16.0-feature/version.md) | 已实现并验证 | Markdown 驱动自动化脚本、Pi 三环境引擎（文档对话/代码编译/托管执行）、版本提取防重复生成，以及基于进程 PID 的任务管理器与设备排他锁。 |
| [v1.17.0-feature](v1.17.0-feature/version.md) | 已实现并部署 | 内嵌式 Cron 调度器（robfig/cron）、动态热重载、设备忙防并发跳过保护（SkipIfStillRunning）、自动前置编译与前端排期倒计时看板。已在 10.0.0.6 部署并验证每分钟 Cron 触发。 |
| [v1.18.0-feature](v1.18.0-feature/version.md) | 已完成；真实手机动作另行验收 | 三栏工作台、底部标签、多选设备、所选设备与公共模板合集、明确设备迁移及独立进程管理。2026-10-11 正式构建、Go 全量/race 检查和隔离浏览器验收通过；原动作编译器仍为占位，未升级用户数据库或发布。 |
| [v1.19.0-feature](v1.19.0-feature/version.md) | 已实现并完成真机压测 | 投屏启动加速与会话并发优化：`cleanup=false` 保留设备端 Jar、按设备缓存 SDK 与 Jar 状态、移除 1500ms 硬编码等待与冗余校验、按 serial 单飞建连并延长保活至 60s。真机冷启动首帧由 2.66s 降至 0.53s~0.89s（提速 3.0x~3.9x）。 |
| [v1.19.1-fix](v1.19.1-fix/version.md) | 五平台编译成功；发布被运行测试拦截 | Windows 进程树清理测试通过；原生测试发现中文输出编码与调度测试文件追加问题，转 `v1.19.2-fix` 处理。 |
| [v1.19.2-fix](v1.19.2-fix/version.md) | 已修复并发布 | 2026-10-12 [GitHub Actions](https://github.com/liuzhuogood/MyWebScrcpy/actions/runs/38155571572) 的五平台编译、Windows 原生测试、Release 和 Docker 全部成功；[Release](https://github.com/liuzhuogood/MyWebScrcpy/releases/tag/v1.19.2) 已发布五个程序文件。修复提交 `b6d2077`，保留前序标签。 |
| [v1.19.5-feature](v1.19.5-feature/version.md) | 已实现；纳入 v1.20.1 | 首页系统配置、三主题、Pi/ADB 独立安装与来源配置；scrcpy-server 继续内嵌。本次正式版本为 v1.20.1。 |

| [v1.20.0-feature](v1.20.0-feature/version.md) | 进行中 | Tauri 桌面窗口、原生菜单、托盘、GUI/服务模式与独立依赖管理；三平台验证分别记录。 |
| [v1.20.1-feature](v1.20.1-feature/version.md) | 正式版本已同步；发布包另行验收 | 系统配置、主题、Pi/ADB 按需安装与卸载、系统路径自动检测、主程序依赖拆包；保留 v1.21.0 设计记录。 |
| [v1.21.0-feature](v1.21.0-feature/version.md) | 第一阶段已实现；补充验收待完成 | Pi 技能生成 Python、可安装 SDK 基础能力、来源校验与有效产物执行；真机及其他平台原生链路未测。 |
| [v1.23.0-feature](v1.23.0-feature/version.md) | 已发布 v1.23.0 | 系统设置全局手动匹配服务、Python 依赖检测、多设备 SDK 任务与模板面板提示。 |
| [v1.22.0-feature](v1.22.0-feature/version.md) | 已合入 main，部分验收待完成 | SDK 封装模板资产、Vision 与进程内匹配；接入 Pi sdk-v2，软件测试及只读真机通过，点击、浏览器与原生跨平台验收待完成。 |
| [v1.24.0-feature](v1.24.0-feature/version.md) | 已合入本地 main；待桌面验收 | GUI 小窗透明度 0%～100% 与鼠标穿透，已实现快捷键、主窗口与托盘恢复入口。 |
| [v1.25.0-feature](v1.25.0-feature/version.md) | 已发布 SDK；CI 补充修复转 v1.25.1 | 外部任务运行与上报、小窗透明度与鼠标穿透、移除 Pi/内置脚本并清理遗留模块。 |
| [v1.25.1-fix](v1.25.1-fix/version.md) | 已发布 v1.25.1 | 修复 Windows CI 下 OCR 测试夹具，全平台回归测试、Release 资产、Docker 镜像与 SDK 均成功发布。 |
| [v1.25.2-fix](v1.25.2-fix/version.md) | 已发布 v1.25.2 | 原生投屏小窗无边框与顶栏恢复、Indra OCR 接入及运行环境统一管理。 |
| [v1.26.0-feature](v1.26.0-feature/version.md) | 已发布 v1.26.0 | HID 硬件模拟通道、点击注入模式、Python 环境平滑切换与并发保护、AI 集成与自动化技能市场规划原型。 |
| [v1.26.1-fix](v1.26.1-fix/version.md) | 已发布 v1.26.1 | 修复首页添加网络设备的反馈与即时呈现；同版包含 OCR 文字排版接口与快手极速版短视频任务技能。2026-10-17 [GitHub Actions](https://github.com/liuzhuogood/MyWebScrcpy/actions/runs/39015414870) 的五个平台构建、Windows 桌面任务、Release 与 Docker 全部成功；[Release](https://github.com/liuzhuogood/MyWebScrcpy/releases/tag/v1.26.1) 已发布 10 个资产。 |
| [v1.27.0-feature](v1.27.0-feature/version.md) | 已打标签；发布被 Windows 回归测试拦截，转 v1.27.1 | AI 集成接入指引与技能市场正式实现（后端 `skillsmarket`、前端页面与路由、技能包 ZIP 下载与多助手原子安装）。标签已推送但 `desktop (windows-latest)` 的 Go 回归失败，未生成 Release；助手客户端加载、真机业务与 Windows/Linux 原生运行未验收。 |
| [v1.27.1-fix](v1.27.1-fix/version.md) | 已打标签；发布被 Windows 模板监听互锁拦截，转 v1.27.2 | 修复技能包 `request_client.py` 在 Windows 默认代码页（cp1252）下打印中文帮助抛 `UnicodeEncodeError` 的问题（该修复在 Windows 上已生效）；同一次任务里 `internal/template` 因既有 fsnotify 互锁测试超时，未生成 Release。 |
| [v1.27.1-feature](v1.27.1-feature/version.md) | 界面验收通过；三项功能缺口待修复 | 任务目录手动扫描、总状态及决策、通用技能和应用规则精简；左右分栏、图片网格与页内原图预览已验证。按用户授权本地合入 main，沿用当前 1.27.2 版本；超时、ESC 动态 XML、AI 配置保存仍待修复，见验收记录。 |
| [v1.27.2-fix](v1.27.2-fix/version.md) | 已发布 v1.27.2 | 修复 `DirWatcher.Close` 在 Windows 上先关 fsnotify、后等事件循环造成的互锁（关闭时目录事件在途会永久阻塞），并让 `removeWatch` 只处理登记过的目录；新增关闭场景回归测试。其余内容与 v1.27.0 相同。2026-10-17 [GitHub Actions](https://github.com/liuzhuogood/MyWebScrcpy/actions/runs/39018862813) 全部成功；[Release](https://github.com/liuzhuogood/MyWebScrcpy/releases/tag/v1.27.2) 已发布 10 个资产。 |
| [v1.28.0-feature](v1.28.0-feature/version.md) | 已合本地 main；待真机验收 | OCR 接口新增 stabilization_ms（0～5000ms），在指定窗口内多次识别并综合；保留 auto 串行逻辑，新增独立 parallel 模式，API、SDK 与引擎选择入口同步。 |
| [v1.30.0-feature](v1.30.0-feature/version.md) | 待实施；原型已验证 | 投屏拖拽安装 APK；原型与公网预览已验证，已调整计划并更新四件套文档。 |
| [v1.31.0-feature](v1.31.0-feature/version.md) | 已实现并通过验证 | 首页全新全息空间甲板（Holo Spatial Deck）设计重构，全宽展开，隐藏帧率，设备列表与纯蓝 #406BDB 投屏键，前台应用卡片；支持 0%~100% 主窗口无级全透明穿透，修复 macOS 浅色系统下黑暗模式泛白问题。 |
| [v1.32.0-feature](v1.32.0-feature/version.md) | 已合入 main；浏览器/真机/桌面验收待完成 | 竞品与源码对照；首页新增手机视图、迁入大屏能力并移除独立入口；右侧批量操作面板、每页台数和基础批量动作。后续标签/实时群控、短语/文件分发、动作录制分版推进。 |
| [v2026.10.21-feature](v2026.10.21-feature/version.md) | 正式发布版本 | 全息空间甲板视觉、AppHeader 顶栏化与投屏拖拽安装 APK/文件能力；同步 Python 3.9 兼容性修复并按北京时间 tag 发布。 |
| [v2026.10.22-feature](v2026.10.22-feature/version.md) | 正式发布版本 | 按北京时间日期的例行重新发布，代码内容与 v2026.10.21 一致（无新增提交），仅全量同步版本号。 |
| [v2026.10.23-feature](v2026.10.23-feature/version.md) | 正式发布版本 | 合入 v1.32.0-feature：首页实时手机视图（第三视图）与右侧批量操作面板体系、编号小方块显隐、有界并发批量调度、默认仅显示在线设备；全量同步版本号。 |
| [v2026.10.24-feature](v2026.10.24-feature/version.md) | 正式发布版本 | 按北京时间日期的例行发布，代码内容与 v2026.10.23 一致（无新增提交），仅全量同步版本号。 |
| [v2026.10.25-feature](v2026.10.25-feature/version.md) | 正式发布版本 | 按北京时间日期的例行发布，代码内容与 v2026.10.24 一致（无新增提交），仅全量同步版本号。 |
| [v2026.10.26-feature](v2026.10.26-feature/version.md) | 正式发布版本 | 按北京时间日期的例行发布，代码内容与 v2026.10.25 一致（无新增提交），仅全量同步版本号。 |
| [v2026.10.27-feature](v2026.10.27-feature/version.md) | 正式发布版本 | 修复投屏自动亮屏；新增空闲自动熄屏（系统设置 → 通用设置 → 空闲自动熄屏，后端连续空闲10分钟自动休眠）；全量同步版本号。 |
| [v2026.10.28-feature](v2026.10.28-feature/version.md) | 正式发布版本 | 按北京时间日期的例行发布，代码内容与 v2026.10.27 一致（无新增提交），仅全量同步版本号。 |
| [v2026.10.29-feature](v2026.10.29-feature/version.md) | 构建失败保留 | 按北京时间日期的例行发布，Windows 桌面任务回归测试遇文件占用竞态未生成 Release，保留标签。 |
| [v2026.10.29-1-fix](v2026.10.29-1-fix/version.md) | 二进制发布成功 / PyPI 构建失败 | 修复模板监听在 Windows 上由于 fsnotify 句柄占用导致测试删除失败的问题；全量同步版本号；全平台客户端与 Release 资产发布成功，但 PyPI 工作流因缺少 packaging 失败。 |
| [v2026.10.29-2-fix](v2026.10.29-2-fix/version.md) | 正式发布版本 | 修复 Python SDK 发布工作流中的 packaging 依赖与版本规范比较；全量同步版本号并触发全链路发布。 |
| [v2026.10.30-feature](v2026.10.30-feature/version.md) | 正式发布版本 | 按北京时间日期的例行发布，包含 v2026.10.29-2 的 Python SDK 发布依赖与测试加固，全量同步版本号。 |
| [v2026.10.31-feature](v2026.10.31-feature/version.md) | 正式发布版本 | 按北京时间日期的例行发布，包含 v2026.10.30 的全量能力与发布加固，全量同步版本号。 |


## Vue 前端迁移版本

前端现位于主仓库 `frontend/`。以下版本描述其迁移过程，原独立仓库路径与旧 HTML 对照用于追溯。

| 版本 | 当前状态 | 范围与证据 |
| --- | --- | --- |
| [v0.1.0-feature](../../frontend/docs/versions/v0.1.0-feature/version.md) | 已完成 | 首页实施与历史联调已完成，v0.2 已开工并交付，旧开工确认待办已关闭。现已并入主仓库 `frontend/`，生产页不显示 Mock 开关。 |
| [v0.2.0-feature](../../frontend/docs/versions/v0.2.0-feature/version.md) | 已实现；待验收 | 投屏核心实施与已记录的真机冒烟完成。键盘、滚轮、断线重连、非锁定应用真旋转和真实浏览器全屏仍待专项验收。 |
| [v0.3.0-feature](../../frontend/docs/versions/v0.3.0-feature/version.md) | 已完成；部分延期 | 双设备大屏、操控和尺寸切换已完成历史联调，版本摘要已存在。原 Vision 绿框接入延期；v0.6 的模板命中轮询不能作为当前 Vision 广播验收证据。 |
| [v0.4.0-feature](../../frontend/docs/versions/v0.4.0-feature/version.md) | 已实现；待验收 | tasks.md 与本文件已记载文件操作闭环，session.md 停留在早期网络中断阶段，不能把其中临时阻塞当作当前故障。全文搜索与上传队列 UI 未覆盖，仍待专项验收；本次未重做文件操作。 |
| [v0.5.0-feature](../../frontend/docs/versions/v0.5.0-feature/version.md) | 已实现；验收记录待核实 | 检查器历史联调已记录。tasks.md 勾选放大镜目视确认通过，但 session.md 与本文件仍写待确认；缺少明确确认过程，本次不据此断言已验收，保留待核实。 |
| [v0.6.0-feature](../../frontend/docs/versions/v0.6.0-feature/version.md) | 已被后续版本替代；前端待同步 | 历史管理、请求和错误显示验证保留。主项目 v1.11 已删除 matches/click 等匹配接口，当前不能称模板匹配可用；Vue 尚存对应调用、控件及轮询，需要同步为仅模板管理。 |
| [v0.7.0-feature](../../frontend/docs/versions/v0.7.0-feature/version.md) | 已实现；待真机验收 | 录制、回放、截图和区域截图代码已接入；本次合并流程的 vue-tsc 与生产构建已通过。真机录制/回放、截图与存模板完整闭环未验收，不能标为全部完成。 |

## 尚未收口的事项

- **v1.12 遗留待处理**：分割线总是占不满、图标未统一、模板匹配当前不可用；处理方向和验收见 [v1.12 任务清单](v1.12.0-feature/tasks.md)。
- **延期方案**：原 v1.12 的显示尺寸入设置、手机边框、录制边框/点击反馈 UI、参数、触控日志与后端合成，见 [延期方案](v1.12.0-feature/deferred-plan.md)。
- **待同步**：Vue v0.6 模板面板仍含已被主项目 v1.11 删除的匹配/点击调用、控件及轮询；按仅模板资产管理的最终决定收口，不恢复旧匹配引擎。
- **待验收**：Vue v0.7 的真机录制/回放/截图/存模板完整闭环；v0.2 键盘、滚轮、重连、旋转与全屏；v0.4 全文搜索及上传队列 UI；v0.5 放大镜确认记录冲突。
- **待专项验收**：v1.10 终端逐项交互与进程回收；v1.11 移除前后坏帧/CPU/帧连续性对比；v1.5.2 XML/旋转回归；v1.5.3 听感、Android 11 和断流重连。
- **延期**：v1.0 的认证、分组、连接历史等后续计划，v1.2 的可选并发保护/独立发布，v1.4 的鉴权与完整链路优化，v1.10 跨面板终端历史恢复。

## 本次核对依据与边界

- 对照各版 `tasks.md`、`version.md`、`session.md` 与当前代码，不用「已有版本目录」推断完成。
- Git 历史确认 `b65e877` 仅新增原 v1.12 四份文档。原方案尚未实施，已归入延期记录；当前 v1.12 的重构范围按本次用户调整记录。
- 已只读核对远端 `v1.3.0`、`v1.5.0`、`v1.6.0`、`v1.10.0` 标签；未据此断言 GitHub Release 或 Actions 成功。
- 当前统一前端已部署至 `10.0.0.6`，根入口为 `/`，旧 `/app/` 与 HTML 页面不再提供。合并流程已通过 Vue 构建、Go 测试与静态检查、页面及资源冒烟；未重新执行全部历史设备操作。
- 本次只读复查服务为 active/running，PID `897759`，重启计数 `0`；部署二进制 SHA-256 为 `efa4cabef021c6ed6ccb7079bc6d8f51873078cdf7ad2d9f74705c9d767b1685`，与本次部署产物一致。
- 用户已授权提交、推送并打标签。正式版本、OpenAPI 与前端包版本同步为 `v1.12.0`（JSON 中为 `1.12.0`）；发布检查、Git 远端与 Actions 结果分别核对。

## 本次会话

**用户**：「看一下 versions下的版本记录，我感觉应该都已完成了」；核对后：「按你的建继续」。

**处理**：补齐有证据的任务与摘要，保留真实未实施项及验收边界，并用以上总览统一说明历史版本与当前部署的关系。

**2026-10-09 范围调整**：用户将 v1.12 改为 Web 端重构，并指定分割线、图标统一与模板匹配不可用三项遗留问题。已同步四件套和本总览，原方案保留为延期记录。

**2026-10-09 发布验收**：重构提交 `a7bb1e6` 与标签 `v1.12.0` 已推送，远端提交指向一致。[GitHub Actions](https://github.com/liuzhuogood/MyWebScrcpy/actions/runs/37914234053) 的五个平台构建、Release 和 Docker 任务全部成功；[v1.12.0 Release](https://github.com/liuzhuogood/MyWebScrcpy/releases/tag/v1.12.0) 已发布五个程序文件。三项遗留问题保持待处理。
