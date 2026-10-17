# 项目术语字典(Glossary)

统一人与 AI 的表达:口语说法 → 项目内精确指代(路由 / 文件 / 接口 / 概念)。

## 使用规则

1. 用户说法命中"别名"列任一词,按同行"指向"理解,直接执行,不再确认。
2. 字典未覆盖、或一个说法对应多个条目的,先与用户确认指代再动手。
3. 方案、提交说明等输出优先使用"术语"列标准词。
4. 新增路由、大面板或核心功能时,在同一改动中登记本字典;重命名先改字典再改代码。

## 页面与路由

| 术语 | 别名(口语) | 指向 |
|---|---|---|
| 设备列表页 | 首页、主页、设备页 | 路由 `/` · `frontend/src/views/HomeView.vue` |
| 投屏详情页 | 投屏页、投屏操控页、播放器页、控制页 | 路由 `/player?serial=<序列号>` · `frontend/src/views/PlayerView.vue` |
| 大屏监控页 | 大屏、监控墙、dashboard | 路由 `/dashboard` · `frontend/src/views/DashboardView.vue` |
| 文件管理页 | 文件页、文件管理器 | 路由 `/files?serial=<序列号>` · `frontend/src/views/FilesView.vue`;亦内嵌于投屏详情页抽屉(`DevicePlayer.vue`,embedded) |
| 任务运行页 | 任务面板、任务页、Task Runs | 路由 `/tasks` · `frontend/src/views/TasksView.vue` |
| 依赖管理页 | 依赖页 | 路由 `/dependencies` · `frontend/src/views/DependenciesView.vue` |

## 界面区域与面板

| 术语 | 别名 | 指向 |
|---|---|---|
| 设备卡片 | 卡片 | `frontend/src/components/device/DeviceCard.vue` |
| 顶栏 | header、顶部导航 | `frontend/src/components/layout/AppHeader.vue` |
| 小窗顶栏 | 小窗工具条、投屏工具栏 | `frontend/src/components/player/DevicePlayer.vue` 内的工具栏;隐藏后可用顶部把手、恢复小窗快捷键或菜单“视图 → 恢复所有小窗”恢复 |
| 小窗窗口设置 | 透明度菜单、鼠标穿透菜单 | `frontend/src/components/player/WindowEffectsControl.vue`;原生小窗的透明度、鼠标穿透与无边框开关 |
| 无边框 | 隐藏系统标题栏 | 原生投屏小窗的系统窗口装饰开关,独立于小窗顶栏;系统设置的“投屏设置”保存新小窗默认值,小窗窗口设置与投屏设置侧栏控制当前窗口 |
| 投屏设置 | 播放器设置 | 系统设置页签 `frontend/src/components/system/PlayerSettingsContent.vue` 保存默认配置;投屏详情页侧栏 `frontend/src/components/player/PlayerSettingsPanel.vue` 调整当前投屏 |
| 右侧抽屉 | 抽屉、侧栏、侧边栏 | 投屏详情页右侧自适应面板,收纳检查器、文件管理、模板管理、ADB 终端(`frontend/src/components/player/DevicePlayer.vue`) |
| 播放器检查器 | 检查器、取色器、放大镜 | 右侧抽屉内:取色坐标、像素放大镜、XML 树、XPath 与元素定位联动 |
| 检查模式 | 拾取模式、检查开关 | 检查器顶栏的显式开关,仅开启后拦截画面输入;打开面板默认可操控设备,完成取色、元素选择或 OCR 框选后自动退出 |
| ADB 终端 | Web Shell、交互式终端 | 右侧抽屉内 xterm.js 终端,真实 PTY 分配 |
| 系统配置 | 设置、系统设置、系统配置弹窗 | 首页入口弹窗 `frontend/src/components/system/SystemConfigDialog.vue`,页签:主题 / ADB / Python / 模板匹配 / OCR 服务 |
| 大屏磁贴 | 磁贴、tile | `frontend/src/components/dashboard/DashTile.vue` |

## 核心概念

| 术语 | 别名 | 含义与指向 |
|---|---|---|
| 序列号 serial | 设备号、设备 ID | ADB 设备序列号;路由与接口统一用 `serial` 参数定位设备 |
| 模板 / 模板匹配 | 识图、找图、模板图 | 选区截图存为模板,交 Python 匹配服务执行;不参与投屏视频处理,见 `docs/python-matching-service.md` |
| 页面信息 | UI Package、uipage | 前台应用包名、Activity、焦点窗口、分辨率、DPI、旋转;接口 `/api/ui/page-info`,后端 `internal/uipage` |
| UI XML | XML 树、UI dump | UIAutomator XML 快照;接口 `/api/ui/xml`,后端 `internal/uixml`,用于 XPath 与元素定位 |
| OCR | 识文字、文字识别 | 接口 `POST /api/vision/ocr`;引擎优先级 Indra OCR → macOCR → PaddleOCR,自动模式仅契约错误时降级,见 `docs/system-config.md` |
| 任务运行 | Task Runs、任务上报 | 外部自动化执行端上报生命周期、心跳、日志与 Markdown 报告,见 `docs/task-runs.md` |
| 动作执行 | 拟人化注入、input 执行 | 两种模式:SDK 拟人化注入(贝塞尔曲线/落点微抖动)与 ADB Shell input 系统级穿透 |
| 快手短视频技能 | 快手技能、快手极速版任务、观看视频模式 | `skills/kuaishou-short-video/SKILL.md`；红包守护、连续打卡白拿手机、连续签到的视频观看流程 |

## 后端模块速查(`internal/`)

| 模块 | 职责 |
|---|---|
| `device` / `scrcpy` / `ws` | 设备管理、scrcpy-server 启动与视频流、WebSocket 通道 |
| `devicegate` | 设备操作并发门控:默认串行协调,显式声明的调用方允许有界并行 |
| `framehub` | 有界"最新帧"扇出,向视觉消费方(OCR / 模板匹配)分发帧 |
| `adbcommand` / `adbshell` / `action` | ADB 命令、交互式 Shell、动作执行 |
| `uipage` / `uixml` | 页面信息、UI XML 快照 |
| `vision` / `ocr` / `template` / `matchingservice` / `automation` | 视觉接口、OCR 引擎调度、模板管理、模板匹配服务进程、Go 原生自动化与视觉条件判断 |
| `files` | 文件管理后端 |
| `taskrun` | 任务运行后端 |
| `operationlog` | 操作日志 |
| `dependencies` / `servicepython` / `serviceprefs` | ADB/Python 依赖管理、Python 服务进程管理、服务开关偏好持久化 |
| `updater` / `desktop` / `db` / `debug` | 应用更新、桌面模式、数据库访问、调试 |

## 原型设计中的页面

以下入口属于独立原型，尚未接入正式业务路由。

| 术语 | 别名(口语) | 指向 |
|---|---|---|
| AI 集成原型 | AI 集成菜单、与 AI 集成 | `prototypes/cases/ai-integration-v2/Prototype.vue`；独立预览 `#ai-integration-v2`，v1 保留用于对照 |
| 技能市场原型 | 技能市场、APP 自动化技能 | v2 首页中的 `components/SkillMarket.vue`；搜索、分类、已安装过滤和技能入口 |
| 技能详情原型页 | 查看详情、技能详情、技能二级页面 | `prototypes/cases/ai-integration-v2/components/SkillDetailPage.vue`；预览查询参数 `?skill=<SkillId>#ai-integration-v2`，支持返回市场 |
