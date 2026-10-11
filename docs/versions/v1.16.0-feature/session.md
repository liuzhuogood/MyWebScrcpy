# v1.16.0-feature 会话记录

## 记录原则

- 用户原话完整记录。
- 实际工具回复精简记录，只保留结论、承诺、关键问题和关键决策。
- 每次用户对话都要追加记录。

## 会话

### 1. 用户

> lz-vlog 我想做一个功能：支持新增 Markdown 脚本。
> 这个 Markdown 脚本可以：
> 1. 存在服务端的 SQLite 数据库里
> 2. 也可以选择存在手机本地存储上
> 脚本生成逻辑是这样的：
> 1. 首次会根据 Markdown 里的正则提取版本号
> 2. 如果提取不到版本号，就使用 Markdown 文件的时间作为版本号
> 3. 然后生成稳定的 Python 脚本
> 4. 如果版本号一样（也就是没有修改过），就不用重新生成
> Markdown 里面会有文字和图片，到时候会专门做一个技能来生成脚本：
> 1. 文字：生成具体的流程
> 2. 图片：一般用来做模板匹配
> 另外，我需要在界面上可以管理这些模板。

### 1. OpenCode

- 一句话复述理解：梳理基于 Markdown 的自动化脚本体系，支持服务端 SQLite 存储与手机端存储，设计基于正则提取版本号（时间戳兜底）的防重复编译决策引擎，文字转 ADB 控制流、图片转模板匹配，并在 Web 端提供模板可视化管理。
- 输出系统架构图、Markdown 规范（YAML Front-Matter）、正则提取模式设计及 SQLite 表结构设计。
- 提示手机端存储的时间戳变动与模板离线风险，提出服务端作为主元数据中心、手机端作为镜像同步的建议方案。

### 2. 用户

> 你先帮我设计一下，不用直接修改代码

### 2. OpenCode

- 确认按纯设计与原型规划推进，不改动生产业务代码。

### 3. 用户

> 另外，Markdown 文本里也要指定这个脚本定时运行的周期，可以通过正则把它提取出来。还有一些其他设置也要设计成可以通过正则提取，并且在 Markdown 里能很直观地看到这些设置。

### 3. OpenCode

- 细化 YAML Front-Matter 正则提取规格，补充 `schedule`（Cron 周期）、`target_device`（目标设备）、`timeout_sec`（超时保护）、`enabled`（启用开关）的正则表达式与兜底策略。
- 梳理文字流程与图片模板引用的 Markdown 语法规范（`![名称](template://id?threshold=0.85)`）。

### 4. 用户

> 设计还要再详细一些，要利用项目里圆形设计的技巧，尽量做得更细。
> 主要界面上我想看到：
> 1. 脚本
> 2. Markdown 预览（还可以修改）

### 4. OpenCode

- 加载 `ui-prototype` 技能，进入独立原型沙盒环境。
- 新建原型方案 `prototypes/cases/markdown-script-v1/`，打造「Markdown 源码编辑（左）」+「实时交互渲染预览（中）」+「稳定 Python 产物与缓存判定（右）」的三栏工作台。
- 原型通过 `proto:check` 和 `proto:build` 验证，启动本地预览服务 `http://127.0.0.1:5174/#markdown-script-v1`。

### 5. 用户

> 把原型截图给我看一下，然后把原型的功能列一下吧

### 5. OpenCode

- 通过浏览器自动化捕获原型实机运行截图与模板资产管理弹窗截图。
- 列出原型的五大核心功能清单（顶部状态、源码编辑、渲染提取、Python 缓存决策、模板资产库）。

### 6. 用户

> 增加一个设计：自动生成文档。需要确认在哪个地方加入“自动生成文档”和“调整文档”的功能

### 6. OpenCode

- 分析文档生命周期（需求到文档 vs 文档迭代），对比三种入口布局方案，推荐方案 1（在 Markdown 编辑区就地集成）。

### 7. 用户

> 就像那种代码生成一样。一个栏，就是有一个输入框，然后有一个对话，像一个对话一样的
> 请先做圆形的设计

### 7. OpenCode

- 升级原型为四栏联动架构（新增左侧独立的 `AI 文档助手` 对话栏）。
- 支持输入自然语言需求触发「自动生成文档」与「就地调整/润色文档」，对话指令一键回填源码、同步触发中栏渲染与右栏版本决策。
- 原型验证通过并完成实机截图交付。

### 8. 用户

> 截图给我看一下

### 8. OpenCode

- 捕获并交付带有 AI 交互对话、Markdown 联动刷新与状态更新的完整实机截图。

### 9. 用户

> 使用pi作为引擎 修改文档 转换脚本 稳定执行（拖管执行） 每个功能对就一个pi环境，环境里有不同的系统提示词，技能

### 9. OpenCode

- 输出 Pi 三环境解耦架构：
  1. **Pi 1 (Doc Agent)**：修改/生成文档，专长 Markdown 策划与模板引用；
  2. **Pi 2 (Code Agent)**：转换稳态脚本，专长 Python/ADB 映射与语法校验；
  3. **Pi 3 (Runner Agent)**：托管稳定执行，专长进程生命周期、心跳守护与执行日志推流。

### 10. 用户

> 还有一个，先帮我记录一下，不需要现在就实现：生成稳定化脚本的时候要有日志、截图；如果 Markdown 里约定了要有视频或截图，就要生成对应的截图和视频，最好再生成一个产物

### 10. OpenCode

- 在 `docs/designs/deferred-artifacts-and-logs.md` 完整归档记录远期产物捕获需求（基础日志与步骤截图、Markdown 显式声明、执行产物归档包 Artifact Bundle）。

### 11. 用户

> 还有没有什么没考虑到的？

### 11. OpenCode

- 深入复盘提出 6 大工程隐患与应对方案：手机存储图片断层、任务并发与设备互斥锁、息屏锁屏状态、文件时间戳误判重编译、跨分辨率适配、僵尸进程超时回收。

### 12. 用户

> 为什么文件的修改日期会变来变去？文件的修改日期不是放在文件的属性上面的吗

### 12. OpenCode

- 从操作系统底层与安卓实际场景解答：`adb push` 默认重置当前传输时间、安卓 `/sdcard/` FUSE/sdcardfs 虚拟文件系统抽象、现代编辑器原子覆盖（Atomic Save / rename）机制及 Git 不追踪 mtime 的设计原理。

### 13. 用户

> 那修改一下规则吧，就是脚本不存在手机里面了，只存在服务端，写简单一点

### 13. OpenCode

- 规则简化调整：彻底废弃手机端本地存储，所有 Markdown 脚本纯存放在服务端的 SQLite 中。
- 同步精简 Markdown YAML 规范（移除 `storage` 字段）与数据库表结构，原型代码同步更新并完成构建。

### 14. 用户

> 我想再设计一个类似任务管理器的东西。每台手机同一时间只会执行一个任务，每个任务都有一个进程 PID。这个 PID 会注册到任务管理器表里，并且有生命周期；在任务管理器里也可以直接结束任务。其他脚本执行时，先去任务管理器里判断有没有脚本在执行就行了。这样就实现了一个锁机制，之前那个锁机制就不用了，直接用任务管理器的锁机制就行了

### 14. OpenCode

- 吸收任务管理器设计：废弃抽象锁协议，改为基于系统真实进程 PID 登记的「任务管理器（Task Manager）」设备排他锁机制。
- 设计 `running_tasks` 持久化表与生命周期时序图。
- 在原型中增加任务管理器弹窗、活跃任务角标、设备占用锁冲突拦截与「结束进程」释放锁交互链路，完成实机截图验证。

### 15. 用户

> 重新加载一下原型技能

### 15. OpenCode

- 重新加载 `ui-prototype` 技能，核实其新增加的临时公网预览（Cloudflare Quick Tunnel）交付规范。

### 16. 用户

> 然后打原型的链接发布到外网

### 16. OpenCode

- 启动脱离会话的独立本地开发进程与 cloudflared 临时隧道。
- 在 `frontend/vite.prototype.config.ts` 精确放行临时公网 Hostname。
- 交付可点击公网链接（`https://examining-remedy-layer-auditor.trycloudflare.com/#markdown-script-v1`）与 `session.json` / `stop.sh` 归档。

### 17. 用户

> 重成版本四件套吧

### 17. OpenCode

- 一句话复述理解：按版本工作流规范正式初始化 `v1.16.0-feature` 版本四件套文档（`version.md`、`design.md`、`tasks.md`、`session.md`）并更新总览。

### 18. 用户

> 把这个版本实施完成吧

### 18. OpenCode

- **复述理解**：正式实施 `v1.16.0-feature`，落地后端 SQLite 数据持久化、正则提取与版本编译器、任务管理器生命周期与设备排他锁，以及前端四栏工作台与任务管理器交互，并完成全链路单元测试与真机端到端验收。
- **阶段 1 数据库与数据访问**：
  - 在 `internal/db/db.go` 中建表迁移 `markdown_scripts` 与 `running_tasks`；
  - 编写 `internal/db/scripts.go` 与单测 `scripts_test.go`，验证 CRUD 与设备占用锁判定，单测 100% 通过。
- **阶段 2 正则解析与版本决策**：
  - 编写 `internal/mdscript/parser.go` 正则提取版本号、Cron、设备、超时及模板图片宏，无版本时回退时间戳 `ts_YYYYMMDDHHmmss`；
  - 编写 `internal/mdscript/compiler.go`，实现版本一致防重复生成及符合规范的稳定 Python 代码生成；
  - 编写 `parser_test.go` 单测，覆盖全部提取边界并测试通过。
- **阶段 3 任务管理器与进程锁**：
  - 编写 `internal/taskmanager/manager.go`：设备互斥判定 ➔ 创建临时隔离执行环境 ➔ 启动 Python 子进程 ➔ 注册真实 PID ➔ 监听退出更新状态 ➔ 支持主动强杀释放锁；
  - 编写 `manager_test.go` 单测，验证多设备并发互斥锁与强杀进程释放锁逻辑，单测 100% 通过。
- **阶段 4 HTTP API 接口封装**：
  - 编写 `internal/mdscript/handlers.go` 与 `internal/taskmanager/handlers.go`，暴露 `/api/md-scripts` 与 `/api/tasks` 相关 REST 接口；
  - 在 `main.go` 注册上述路由。
- **阶段 5 前端 Web 工作台接入**：
  - 在 `frontend/src/api/mdScripts.ts` 封装脚本与任务管理器 API；
  - 在 `frontend/src/views/ScriptsView.vue` 落地正式四栏交互界面与任务管理器弹窗；
  - 在 `frontend/src/router/routes.ts` 与 `frontend/src/components/layout/AppHeader.vue` 注册 `/scripts` 入口与导航按钮。
- **阶段 6 全链路构建与真机验收**：
  - 执行 `vue-tsc -b && vite build`，前端编译通过；
  - 执行 `go test ./...`，全量单元测试通过；
  - 执行 `./scripts/build.sh`，Go 二进制构建成功；
  - 在在线真机 `10.0.0.30:5555` 上完成端到端验证：新建脚本 ➔ 自动正则提取 ➔ 编译生成 Python ➔ 启动任务管理器 ➔ 拦截同设备重复运行 ➔ 运行完毕自动释放锁；
  - 停止并清理临时原型公网预览隧道。更新版本文档。
