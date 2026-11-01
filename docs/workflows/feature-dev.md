# 全栈特性迭代工作流 (feature-dev)

状态：可执行  
用途：指导在 MyWebScrcpy 项目中执行端到端全栈特性开发，涵盖需求设计、后端领域逻辑/接口开发、前端交互联调、全量自动化门禁与文档资产同步，保证在项目内闭环交付。  

## 输入与规范

- **目标项目**：`/Users/liuzhuo/code/MyWebScrcpy`
- **特性主题**：`<feature-name>`（如 `device-batch-control`）
- **目标版本**：`<version>`（如 `v1.33.0-feature`，对应 `docs/versions/<version>/`）
- **特性分支**：`feature/<feature-name>` 或版本分支
- **工程门禁命令**：
  - 后端单元测试：`go test ./internal/...`
  - 前端类型校验与生产构建：`pnpm --dir frontend build`（`vue-tsc -b && vite build`）
  - 原型套件隔离校验：`pnpm --dir frontend proto:check`
  - 服务启停与状态验证：`bash scripts/services.sh status`

---

## 阶段步骤与判定标准

| 编号 | 操作 | 通过条件 | 失败处理 |
| --- | --- | --- | --- |
| **FD1** | **需求拆解与术语/版本对齐**<br>1. 明确特性范围与验收标准。<br>2. 初始化或更新 `docs/versions/<version>/` 下的 `version.md`、`design.md` 和 `tasks.md`。<br>3. 核对 `docs/glossary.md`，若新增路由、页面或核心概念，同步登记术语字典。 | 任务拆解清晰且具备可验证标准；版本设计文档完备；术语字典已对齐。 | 针对歧义项进行需求澄清，不带未定义范围进入编码。 |
| **FD2** | **后端模型与接口契约实现**<br>1. 在 `internal/` 对应模块中实现核心逻辑、状态流转与数据契约。<br>2. 暴露 HTTP REST API 或 WebSocket 通道，遵从并发保护（如 `devicegate` 协调）。<br>3. 编写配套 `*_test.go` 单元测试。 | 运行 `go test ./internal/<module>/...` 单测 100% 通过；关键分支与边界异常均有测试覆盖；无并发数据竞争。 | 根据测试失败栈排查代码，严禁通过注释测试或吞异常制造通过。 |
| **FD3** | **前端页面与组件交互实现**<br>1. 在 `frontend/src/` 中实现视图、组件与数据流（优先使用 `src/components/ui/` 下的 shadcn-vue 组件与 Lucide 图标）。<br>2. 对接后端 API / WS，完善加载态、空状态与错误提示。<br>3. 支持亮色/暗黑主题自适应及响应式布局。 | 界面与交互逻辑完整，无 Vue 运行时 warning，交互反馈即时可靠。 | 检查组件 Props/Emits、Pinia 响应式状态与 Tailwind CSS 类名冲突。 |
| **FD4** | **全栈门禁复测与文档同步**<br>1. 后端全包回归：`go test ./internal/...`<br>2. 前端构建与静态检查：`pnpm --dir frontend build`<br>3. 原型隔离复核：`pnpm --dir frontend proto:check`<br>4. 接口文档同步：若变动自动化或公开接口，核对 OpenAPI 配置并运行 `go test ./...`。 | 后端测试全部通过；前端 TypeScript 及 Vite 构建零报错；接口文档与代码一致；无未跟踪或未隔离的构建废件。 | 拦截构建与测试报错并针对性修复，红灯时绝对禁止进入提交阶段。 |
| **FD5** | **原子提交与版本资产归档**<br>1. 按 Conventional Commit 精确暂存相关源码与文档变更（`git add -- <明确路径>`）。<br>2. 归纳中文语义提交（如 `feat(模块@版本): 核心改动`）。<br>3. 更新 `docs/versions/<version>/tasks.md` 任务进展并完成版本记录收口。 | 工作区干净（`working tree clean`）；提交粒度精准、信息可追溯；版本设计与任务记录已归档。 | 严禁 `git add -A`；遇未跟踪脏文件按规则清理或保护，严格遵循推送授权。 |

---

## 启动与运行方式

- **启动工作流**：在会话中说明 `使用 feature-dev 工作流，任务是 <特性描述>`。
- **进展与资产归档**：执行进展与生命周期数据统一记录并闭环于 `docs/versions/<version>/`（包含 `version.md`、`design.md`、`tasks.md`），不额外引入独立运行日志文件。
- **门禁要求**：流程结束前必须全部跑通 FD4 阶段的 3 项核心构建与测试命令。
