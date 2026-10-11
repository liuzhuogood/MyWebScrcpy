# v1.17.0-feature 任务清单：内嵌 Cron 调度器与排期执行系统

**当前状态：设计定稿，版本四件套初始化完毕，待实施正式编码。**

---

## 阶段 1：调度器核心服务 (`internal/scheduler/`)

- [x] 1.1 引入 Go 标准调度库 `github.com/robfig/cron/v3`。
- [x] 1.2 创建 `internal/scheduler/scheduler.go`：
  - 实现基于 `cron.New()` 的调度生命周期守护（`Start` / `Stop`）；
  - 实现全量从数据库加载有效脚本（`schedule_cron != ""` 且 `is_enabled = 1`）；
  - 实现单脚本动态热重载（`ReloadScript` / `RemoveScript`），维护内存中的 `cron.EntryID` 映射。
- [x] 1.3 实现定时触发工作单元（Worker）：
  - 触发时自动比对版本并调用 `mdscript.Compiler` 生成稳态 Python；
  - 检查目标设备当前锁状态，若忙则记录 `skipped` 审计日志，若闲则调用 `taskManager.StartTask`。
- [x] 1.4 编写 `internal/scheduler/` 单元测试，覆盖标准 Cron 解析、动态增删重载及防重叠跳过逻辑。

---

## 阶段 2：数据访问与状态追踪扩展 (`internal/db/`)

- [x] 2.1 在 `internal/db/db.go` 中建表迁移：
  - 为 `markdown_scripts` 补充 `last_run_at` 与 `last_run_status` 字段。
- [x] 2.2 增加数据访问方法：
  - `UpdateScriptRunStatus(id int64, status string, timestamp int64)`；
  - `ToggleScriptEnabled(id int64, enabled int)`。
- [x] 2.3 补充 `internal/db/` 单元测试。

---

## 阶段 3：HTTP API 接口与路由集成 (`internal/scheduler/handlers.go`, `main.go`)

- [x] 3.1 暴露调度器接口：
  - `GET /api/scheduler/entries`：查询当前活跃的 Cron 调度项列表及 `NextRunAt` 时间；
  - `POST /api/md-scripts/{id}/toggle-schedule`：快速切换脚本定时启用状态；
- [x] 3.2 在 `main.go` 中启动 Scheduler 实例并注册路由；在脚本 CRUD 更新与删除时触发调度器热重载。

---

## 阶段 4：前端 Web 工作台与排期看板接入 (`frontend/src/`)

- [x] 4.1 在 `frontend/src/api/` 新增 `scheduler.ts`，封装调度接口与模型类型。
- [x] 4.2 在 `ScriptsView.vue` 中接入排期状态：
  - 指标卡增加「下一次执行」倒计时（如：`下次执行：今日 19:30 (12分钟后)`）；
  - 增加定时调度启用/禁用快捷开关；
- [x] 4.3 在任务管理器弹窗内增加「排期任务大盘」Tab，一眼查看全设备排期日程。

---

## 阶段 5：全链路测试、构建与生产部署

- [x] 5.1 执行前端静态类型检查与构建：`pnpm --dir frontend build`。
- [x] 5.2 执行全量 Go 单元测试：`go test -v ./...`。
- [x] 5.3 运行全流程构建打包：`./scripts/build.sh`。
- [x] 5.4 部署至 `10.0.0.6` 生产服务器并进行真实真机 Cron 触发与排他锁互斥验收。
- [x] 5.5 更新版本文档总览与发布记录。
