# v0.1.0-feature 会话记录

## 记录原则

- 用户原话完整记录。
- 实际工具回复精简记录，只保留结论、承诺、关键问题和关键决策。
- 每次用户对话都要追加记录。

## 会话

### 1. 用户

> 请你参考 MyWebScrcpy 项目， 然后使用 /lz-act init  shadcn-vue 来搭建这个个项目的前端框架，这里只做前端
>
> 先实现 首页

### 1. OpenCode

- 确认 pnpm + 代理到本地后端（`localhost:8080`）。
- 按 shadcn-vue 官方 Vite 流程搭建：Tailwind v4、`components.json`、`cn()`、button/card/badge/input/skeleton/empty，`docs/rules` 按 lz-init 落入 shadcn 套别。
- 实现首页：`GET /api/devices` 轮询 3s、搜索、别名重命名（兼容老 `localStorage` key）、加载/失败/空三种状态；`/player`、`/dashboard` 先占位。
- `vue-tsc -b` 与 `pnpm build` 通过。

### 2. 用户

> 启动起来，我想看一下

### 2. OpenCode

- 启动 `pnpm dev --port 5173`，浏览器确认首页渲染正常。
- 当时后端未启动，页面处于失败态（502 + 重试入口），属预期；提出两个选项：启动老后端联调，或加 Mock 看设备墙。

### 3. 用户

> 继续实现其他功能，先列一下迁移 版本跟踪文档
>
> /lz-vlog

### 3. OpenCode

- 创建 `docs/versions/v0.1.0-feature/` 四件套，本次只记录需求与迁移清单，不推进实现。

### 4. 用户

> 要 Mock 开关

### 4. OpenCode

- 新增 `src/mocks/devices.ts`（5 台假设备，含 1 台 offline，与 Device 契约一致）。
- store 加 `mockEnabled`（`localStorage: mywebscrcpy.mock` 持久化）：开启时不请求后端，关闭恢复真实轮询。
- 工具栏加 shadcn `Switch` + “Mock” 标签；计数显示“x 台设备（Mock）”作状态区分。
- 踩坑：初版误用 `:checked` / `@update:checked`，本仓库 reka-ui 的 SwitchRoot 只认 `modelValue` / `update:modelValue`，导致开关空转；已改为 `:model-value` + `@update:model-value`。
- 验证：浏览器实测开（5 台 Mock 墙）→ 刷新持久化 → 搜索过滤 → 关（回 2 台真机），`vue-tsc + build` 通过。

### 5. OpenCode（目标模式继续推进）

- 重命名回归：实测发现真 bug——`DeviceCard` 把 `ref` 直接绑在 shadcn `Input` 组件上，`inputRef.value.focus is not a function`（组件实例非 DOM）。已修复：取 `inputRef.value.$el` 再 focus/select。
- 回归全过：打开编辑 → 自动聚焦无报错 → 保存别名出徽标 → 刷新持久化 → 改回原名清除别名；控制台干净。
- `vue-tsc + build` 通过，文档同步更新。
