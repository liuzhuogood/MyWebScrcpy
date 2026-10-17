# v1.27.1-feature 实施任务清单

## 1. 版本基线与文档对齐
- [x] 核对当前工作区、Git 分支 `lz/v1.27.1-feature` 与基线 Commit `c237887`
- [x] 建立 `docs/versions/v1.27.1-feature/`（design.md、session.md、tasks.md、version.md）
- [x] 同步术语字典 `docs/glossary.md`（新增任务运行、设备总状态、总状态及决策、通用操作技能）

## 2. 任务中心与 AI 设置可交互本地原型
- [x] 在 `prototypes/cases/task-workspace-v1/` 中新增聚焦原型（任务中心主动扫描/产出检查、系统设置 AI 配置）
- [x] 运行 `pnpm proto:check` 与 `pnpm proto:build`（无错误，构建退出码 0）
- [x] 记录原型验证结果（无独立浏览器环境，标记由当前会话独立验收）

## 3. 任务数据迁移与工作目录主动扫描
- [x] 数据库定向升级：更新 `internal/db/db.go` SchemaVersion 至 `12710`，定向删除旧任务表（`task_runs`, `task_run_logs`），新库不建旧表，移除旧读写逻辑，隔离测试验证非任务数据保留
- [x] 后端任务工作目录管理：默认 `~/.mywebscrcppy/tasks/`，解析绝对路径，支持系统配置读取，不可写时明确报错
- [x] 实现 `POST /api/task-runs/scan`：用户手动主动扫描、内存索引更新、扫描时间与异常统计、防重复提交（`ErrScanInProgress` 返回 409）
- [x] 实现任务列表与详情读取：`GET /api/task-runs`（按内存索引与筛选分页）、`GET /api/task-runs/{id}`（只读指定目录，包含 task.json/state.json/actions.jsonl/产出读取）
- [x] 产出读取与安全防护：`GET /api/task-runs/{id}/outputs/{output_id}` 防路径穿越、符号链接越界、拦截不安全媒体（如 SVG），真实校验报告/日志/截图/视频，区分业务成功与产出完整性
- [x] 清理旧任务注册、心跳、日志上传、结束上报接口与 Python SDK `TaskReporter`（37 个 SDK 既有单测全部通过）

## 4. 统一设备状态与技能模板匹配
- [x] 实现设备总状态采集服务：单轮采集、截图与 OCR/XML/页面信息绑定同一观测标识（`observation_id`）、前后页面上下文稳定性校验（`context_stable`）
- [x] 技能模板加载与隔离匹配：读取 `task.json` 声明技能目录下的 `templates/<id>/`，三类模板（全局、设备、技能）合并匹配，同名来源隔离，不污染全局库
- [x] 实现 `GET /api/device-state/{serial}`：支持 JSON 与 HTML（Accept 区分），关联任务或冲突处理
- [x] 并发与超时控制：基础采集 20s 预算，同设备串行采集，单项失败部分容错降级

## 5. AI 服务配置与总状态及决策接口
- [x] 系统设置持久化扩展：Chat Completions 地址、显式模型名、密钥（受控服务端存储、脱敏返回、日志不记）、超时
- [x] 实现 `POST /api/device-state/{serial}/decision`：复用本轮采集、官方视觉协议（base64 data URL）、目标分析、AI 预算 30s、全局最多 2 并发
- [x] 支持 Accept: text/html 返回决策结果 HTML，支持 JSON 响应，决策结果不触发设备操作
- [x] HTTP fixture 隔离测试：完整请求结构、取消、超时、鉴权失败、并发限制与脱敏（`internal/devicestate/devicestate_test.go` 8 个用例全部通过）

## 6. 通用技能 my-scrcpy-common 与应用技能精简
- [x] 创建通用操作技能 `skills/my-scrcpy-common`（rules、wiki、共享操作脚本：`observe.py`、`watch_video.py`、`try_return.py`、`android_back.py`、`open_app.py`、`restart_app.py`）
- [x] 更新技能市场（`skillsmarket`）校验与白名单，支持 `my-scrcpy-common` 及 `rules`/`wiki`/`templates` 子目录白名单
- [x] 精简现有应用技能（快手等），移除专用业务脚本与 dryrun，统一为规则与通用脚本复用
- [x] 更新 Python SDK、文档及提示词（移除 TaskReporter，更新 task workspace 与产出规范）

## 7. 前端正式界面与联调
- [x] 任务运行页改造：`frontend/src/views/TasksView.vue` 增加手动扫描刷新按钮、扫描状态与时间展示、产出完整性独立呈现、报告/日志/媒体查看与安全下载
- [x] 系统配置弹窗扩展：`frontend/src/components/system/SystemConfigDialog.vue` 增加任务工作目录与 AI 视觉决策模型配置，脱敏展示与安全保存
- [x] 前端构建 `pnpm --dir frontend build` 验证（退出码 0，类型检查通过）

## 8. 全量验证与交付准备
- [x] Go 全量测试 `go test ./...`（全部 PASS）、`go vet ./...`（退出码 0）及竞态测试 `go test -race`（PASS）
- [x] 数据库迁移与安全验证测试：`internal/db/schema_reset_test.go`
- [x] 任务扫描、产出安全与设备状态单测：`internal/taskrun/taskrun_test.go`
- [x] AI 决策 fixture 与配置测试：`internal/devicestate/devicestate_test.go`
- [x] Python SDK 单元测试：Ran 37 tests, OK
- [x] 技能市场集成与安装测试：`internal/skillsmarket/installer_test.go`、`routes_test.go`
- [x] OpenAPI 规范与同步测试：`api_docs_test.go`、`api_artifacts_docs_test.go`
- [x] 版本一致性验证：`node scripts/sync-version.mjs --check`（1.27.1）
- [x] `git diff --check`（无冲突格式问题）与单文件行数限制检查（所有文件 ≤1000 行，方法 ≤400 行）
- [x] 最终文档更新与交付报告完成


## 用户新增界面验收要求（2026-10-17）

- [x] 正式任务中心沿用 `prototypes/cases/task-workspace-v1/` 原型：桌面左侧任务卡片列表，右侧常驻详情，点击任务直接切换右侧，移除任务详情弹窗。
- [x] 右侧保留刷新详情、目标/进度/结果及产出/报告/日志/操作记录切换；使用真实接口，无自动扫描/轮询，窄屏可上下排列。
- [x] 主管在 15272 正式页面验证任务切换、详情刷新、图片缩略图与页内原图预览，保存左右分栏和原图预览截图。视频播放未在最终页面验收。

此项来自用户在对照 15272 正式页和 15271 原型后明确提出的布局修订，图片交互已通过主管浏览器验收；整个版本仍有下述功能缺口。完整要求登记于批准稿 PLAN.md 与项目外 user-layout-request.md。

## 主管最终验收（2026-10-18）

- [x] 目录图片直接按缩略图网格排列；点击缩略图或“查看原图”在当前页面打开大图。
- [x] 原图适配视口；ESC 和右上角关闭可退出，所选任务保持不变；独立下载返回原始图片。
- [ ] 通用操作的总截止时间约束覆盖每次 HTTP 和动作发起。
- [ ] ESC 判断忽略 XML 动态文字，避免将文字更新判为返回成功。
- [ ] AI 配置使用明确密钥写入字段并原子保存。

前述实施勾选不代表整体已验收。详见 [acceptance.md](acceptance.md)。未提交、未推送、未合并；15272 为隔离本地预览，共享 8080 未部署。
