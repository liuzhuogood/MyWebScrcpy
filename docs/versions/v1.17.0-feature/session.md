# v1.17.0-feature 会话与决策演进记录

更新日期：2026-10-11。

---

## 1. 用户提问与背景

> 任务怎么做到排期定时执行？

---

## 2. 核心架构与方案决策

1. **内嵌式 vs 外部系统 Crontab**：
   - 决策：采用 Go 内嵌调度器（`robfig/cron/v3`）。
   - 理由：避免对宿主机环境的 Linux crontab 产生配置侵入与权限要求；脚本在 Web 页面修改后可立即在内存中进行**热重载（Hot-Reload）**，无须重启后端服务。
2. **防重叠与设备排他锁机制**：
   - 决策：定时触发不盲目启动，必须前置通过 `TaskManager.GetActiveTaskByDevice` 检查目标设备。
   - 理由：每台手机同一时间仅允许跑一个任务。若设备正忙，自动记录 `skipped` 跳过本轮，天然杜绝并发冲突。
3. **版本编译自动护航**：
   - 决策：定时触发时自动核验 `compiled_version` 与 `extracted_version`，必要时自动编译，确保定时执行的一定是稳态代码。

---

## 3. 会话记录

### 1. 用户

> 任务怎么做到排期定时执行？

### 1. OpenCode

- 分析现代自动化任务调度架构，提出「内嵌 Cron 调度引擎（robfig/cron/v3）+ 任务管理器设备排他锁检查 + 自动热重载」四步闭环方案。
- 给出 4 大核心细节：内嵌无需系统 cron、防重叠天然复用排他锁、自动编译保障、前端倒计时展示。

### 2. 用户

> 先进行版本设计

### 2. OpenCode

- 输出 `v1.17.0-feature` 完整版本设计提案，包括架构图、时序状态机、数据模型、API 与前端指标卡交互。

### 3. 用户

> 1

### 3. OpenCode

- 正式创建 `v1.17.0-feature` 版本四件套文档（`version.md`、`design.md`、`tasks.md`、`session.md`）并更新项目版本状态总览。

### 4. 用户

> 1

### 4. OpenCode

- **阶段 1 核心调度器实施**：引入 `robfig/cron/v3`，实现 `Scheduler` 守护、单脚本热重载 `ReloadScript` 与注销 `RemoveScript`；实现到点自动版本转译校验与设备排他锁检查。单测覆盖增删停用与忙跳过逻辑。
- **阶段 2 数据库字段迁移**：在 `internal/db/db.go` 与 `scripts.go` 扩展 `last_run_at` 与 `last_run_status` 渐进式列迁移与状态更新方法。
- **阶段 3 API 与路由联动**：暴露 `GET /api/scheduler/entries` 与 `POST /api/md-scripts/{id}/toggle-schedule`，将调度器热重载通知器（`ScriptChangeNotifier`）注入 Markdown CRUD 处理层。
- **阶段 4 前端排期看板**：封装 `api/scheduler.ts`，在 `ScriptsView.vue` 指标区增加「下次执行倒计时」与启停开关，任务管理器弹窗内增加「排期大盘」总览。
- **阶段 5 全链路验证与生产部署**：前端编译通过，全量 Go 单测通过，打包并发布至 `10.0.0.6`，实测每分钟 Cron 自动触发、自动版本转译、分配真实 PID 执行与执行完毕自动释放设备锁。全部任务收口。
