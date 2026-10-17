# v1.27.0：AI 集成接入指引与技能市场正式实现

本版把 v1.26.0 规划并完成交互原型的「AI 集成 / 自动化技能市场」落成正式功能，代码来自版本分支 `lz/v1.26.0-feature`，本次合入 `main`。

## 核心变更

1. **AI 集成接入指引**：
   - 新增正式入口与路由（`/ai-integration`，`frontend/src/views/AiIntegrationView.vue`），页头导航加入「AI 集成」。
   - 覆盖 Codex、Claude Code、WorkBuddy 三种助手：接入步骤、任务示例提示词、官方文档链接与宿主机上的实际技能目录。
   - 安装目标始终是**运行 MyWebScrcpy 服务的宿主机**，不是通过浏览器访问的前端机器。

2. **技能市场**：
   - 技能列表支持搜索、分类、按助手已安装状态筛选；二级详情页可刷新、可返回（`/ai-integration/skills/:id`）。
   - 收录三个技能包：`mywebscrcpy-device-base`（手机操作基础）、`mywebscrcpy-kuaishou-daily`（快手日常助手）、`mywebscrcpy-wechat-helper`（微信消息助手，仅读取与草拟，不自动外发消息）。
   - 统一以 `mywebscrcpy-` 前缀命名；每个包含 `SKILL.md`（合法 frontmatter）、`scripts/request_client.py`（仅用标准库）与 `.mywebscrcpy-manifest.json`（SHA256 清单，标记管理权属）。

3. **下载与安装**：
   - 下载：生成以技能名为根目录的独立 ZIP，不改变任何安装状态。
   - 安装：多选目标助手一键原子安装，逐目标返回 `installed` / `already_installed` / `failed`，支持仅重试失败目标；一个目标失败不回滚也不伪报其他目标成功。

4. **安全保障与已有改动保护**：
   - 仅接受白名单技能 ID 与助手 ID，禁止任意路径输入或外部 URL 下载。
   - 写接口校验 Origin 与 Host，拒绝 `Sec-Fetch-Site: cross-site`。
   - 拒绝路径遍历，校验目标路径中的符号链接，不向符号链接写入。
   - 目标目录缺少本应用 manifest、内容被用户改过或含非本应用管理文件时判定为 `conflict`，明确拒绝覆盖并保留原文件，不做静默兜底。
   - 安装实现按平台分开：macOS/Linux 用受控父目录句柄与不覆盖式原子重命名，Windows 用目录句柄与 `NtSetInformationFile`（不设 `ReplaceIfExists`）；未实现安全发布的平台直接明确失败。

5. **接口与文档**：
   - 新增 `GET /api/ai-integrations`、`GET /api/skills`、`GET /api/skills/{id}`、`GET /api/skills/{id}/download`、`POST /api/skills/{id}/install`；OpenAPI 收录 5 个端点、8 个 schema 与「AI 集成与技能市场」标签。
   - 设计文档见 `docs/ai-integration.md`；术语字典登记正式页面与 `skillsmarket` 模块，v1/v2 原型改为历史原型说明。

## 合入 main 与冲突处理

- 该分支基于早于 `main` 的规范快照，`web/openapi.json` 与其存在 80 处字段级差异（如 `Template.created_at`、`FileItem.actions`、录像/截图列表的旧 schema 引用）。合并只追加分支新增的 5 个端点、8 个 schema 与 1 个标签声明，共新增 614 行、删除 0 行，**未采纳分支过期的字段描述**，`main` 已对齐的规范保持原样。
- `docs/glossary.md` 采用分支侧的 `skillsmarket` 条目与「正式页面对应条目、原型为历史原型」的说明。
- `docs/versions/v1.26.0-feature/{tasks,version}.md` 保留 `main` 已发布的版本记录，未被分支草稿替换。

## 本地验证

- [x] `go build ./...` 与 `go test ./... -count=1` 全量通过，含 `internal/skillsmarket` 的路由、目录、安装器、重命名与 ZIP 用例。
- [x] 前端 `vue-tsc` 类型检查与 `vite` 生产构建通过；`/ai-integration` 与 `/ai-integration/skills/:id` 路由已注册。
- [x] 接口文档一致性测试通过：期望的 5 个端点与 8 个 schema 全部存在，顶层未出现未声明的标签使用，OpenAPI 版本仍为 3.1.0。
- [x] 分支侧已有的验证作为本版证据继承：独立构建、Go 全仓竞态测试、Node/Python 回归、46 项接口与技能包检查、真实浏览器验证、临时助手目录的多目标安装与冲突保护、三个下载包内容与请求脚本核对。

## 未验证范围

- 助手客户端实际加载/发现技能、真实设备的业务执行结果、Windows/Linux 原生运行行为（Windows/Linux 已做交叉编译，未在原生系统运行）。
- 合入 `main` 之后未重新做浏览器实机复验：本次只验到类型检查、生产构建、Go 测试与接口文档一致性，不能据此断言页面交互已验收。

## 发布状态

- `VERSION` 已同步为 `v1.27.0`（`node scripts/sync-version.mjs --check` 通过），前端、桌面与 Python SDK 的版本号同步一致；`web/openapi.json` 中健康检查与自动更新接口的版本示例一并更新为 `1.27.0`。
- 标签 `v1.27.0` 已推送，同次推送的 `v1.26.1` 构建与 Release 成功。
- `v1.27.0` 的构建在 `desktop (windows-latest)` 的 Go 回归测试失败（`internal/skillsmarket.TestExtractedZipPythonClientExecutable` 在 cp1252 代码页下抛 `UnicodeEncodeError`），`release` 与 `docker` 任务被跳过，**未生成 Release**；标签保留，修复与重新发布见 [v1.27.1-fix](../v1.27.1-fix/version.md)。
