# v1.16.0-feature 任务清单：Markdown 脚本与任务管理器

**当前状态：设计定稿，原型及公网交付已完成，待实施正式编码。**

## 1. 数据库与数据访问层 (`internal/db/`)

- [x] 1.1 在 `internal/db/db.go` 中新增建表迁移：
  - `markdown_scripts`（存储 Markdown 原文、正则提取元数据、编译版本、稳定 Python 产物）；
  - `running_tasks`（存储活跃任务、设备序列号、操作系统 PID、运行状态、启停时间戳）。
- [x] 1.2 实现 `markdown_scripts` 的 CRUD 数据访问方法（增删改查、按 ID/名称查询）。
- [x] 1.3 实现 `running_tasks` 的任务注册、状态更新、活跃任务列表查询与根据设备判断活跃任务方法。
- [x] 1.4 编写 `internal/db/` 单元测试，确保建表迁移与并发访问无误。

---

## 2. 正则解析器与版本决策引擎 (`internal/mdscript/`)

- [x] 2.1 创建 `internal/mdscript/parser.go`：
  - 正则提取 `version`（提取不到时以当前更新时间戳 `ts_YYYYMMDDHHmmss` 兜底）；
  - 正则提取 `schedule`（Cron 周期）、`target_device`、`timeout_sec`、`enabled`；
  - 正则提取模板标记 `![alt](template://id?threshold=0.85)`。
- [x] 2.2 创建 `internal/mdscript/compiler.go`：
  - 对比 `compiled_version` 与 `extracted_version`，实现版本一致跳过重新生成的缓存决策；
  - 接入 Pi 环境 2 规则生成符合规范的稳态 Python 执行代码。
- [x] 2.3 编写 `internal/mdscript/` 单测，覆盖有版本、无版本、单双引号及各种 Cron 表达式的提取边界。

---

## 3. 任务管理器与进程锁服务 (`internal/taskmanager/`)

- [x] 3.1 创建 `internal/taskmanager/manager.go`：
  - 实现基于设备序列号的排他锁判定（若目标设备有 `running` 任务则拒绝启动并告警）；
  - 实现托管运行：调用 `exec.Command` 启动 Python 隔离进程，捕获并登记宿主机真实 PID 至数据库；
  - 实现生命周期监听：监听子进程退出码并更新任务为 `completed` 或 `failed`，释放设备锁；
  - 实现强制结束进程（kill PID 并更新状态为 `killed`），立即解除设备锁。
- [x] 3.2 编写 `internal/taskmanager/` 单测，验证多设备并发互斥锁与强杀进程释放锁逻辑。

---

## 4. 后端 HTTP API 接口封装 (`internal/mdscript/handlers.go`, `internal/taskmanager/handlers.go`)

- [x] 4.1 提供 Markdown 脚本管理接口：
  - `GET /api/md-scripts`：列出所有 Markdown 脚本；
  - `GET /api/md-scripts/{id}`：获取单个脚本详情；
  - `POST /api/md-scripts`：新建 Markdown 脚本（自动触发正则提取）；
  - `PUT /api/md-scripts/{id}`：更新 Markdown 脚本；
  - `DELETE /api/md-scripts/{id}`：删除脚本；
  - `POST /api/md-scripts/{id}/compile`：触发编译为 Python 脚本；
  - `POST /api/md-scripts/{id}/run`：请求托管运行（进入任务管理器）。
- [x] 4.2 提供任务管理器接口：
  - `GET /api/tasks/active`：查询当前活跃的执行任务（包含 PID 与耗时）；
  - `POST /api/tasks/{id}/kill`：强制结束指定任务并释放设备锁。
- [x] 4.3 在 `main.go` 注册上述路由并联调测试。

---

## 5. 前端 Web 端四栏工作台与任务管理器接入 (`frontend/src/`)

- [x] 5.1 在 `frontend/src/api/` 新增 `mdScripts.ts` 与 `tasks.ts` 接口客户端封装。
- [x] 5.2 按照已验证的原型 `markdown-script-v1` 实现正式界面：
  - 左侧：AI 文档助手对话栏（支持自动生成与调整文档）；
  - 中左：Markdown 源码编辑器与快捷步骤插入；
  - 中右：实时渲染预览、提取指标卡与模板高亮卡片；
  - 右侧：稳定 Python 产物预览与版本缓存状态展示。
- [x] 5.3 接入任务管理器弹窗与顶栏活跃任务角标，打通「立即运行」设备锁冲突拦截与「结束进程」释放锁。
- [x] 5.4 联动现有模板资产库（`useTemplates.ts`），支持快速插入视觉匹配标记。

---

## 6. 全链路验证与交付

- [x] 6.1 执行前端静态类型检查与构建：`pnpm --dir frontend build`。
- [x] 6.2 执行后端单元测试：`go test -v ./...`。
- [x] 6.3 运行打包全流程：`./scripts/build.sh`。
- [x] 6.4 连接在线真机 `10.0.0.30:5555`，完成真机端到端验收（创建 Markdown ➔ 提取版本 ➔ 编译 Python ➔ 启动任务管理器 ➔ 验证排他锁 ➔ 强杀结束任务）。
- [x] 6.5 更新版本总览与发布记录。
