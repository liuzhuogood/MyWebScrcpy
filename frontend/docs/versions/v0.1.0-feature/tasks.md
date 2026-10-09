# v0.1.0-feature 任务清单

> 2026-10-09 状态核对：**已完成**。详见 [版本摘要](version.md) 与 [统一状态总览](../../../../docs/versions/README.md)；历史验收不等于本次重新验收。

## 1. 规格与设计

- ☑ 1.1 完成 session/design/tasks/version 初稿。
- ☑ 1.2 确认影响范围（纯前端，不碰后端）和回滚策略（独立仓库逐版回滚）。

## 2. 实现

- ☑ 2.1 Vite + Vue3 + TS + Tailwind v4 + shadcn-vue（手动接入：components.json、cn、主题 token、6 个基础组件）。
- ☑ 2.2 路由（`/`、`/player`、`/dashboard` 占位）+ Pinia 设备 store + `/api` 代理。
- ☑ 2.3 首页：设备墙、搜索、别名重命名、加载/失败/空三态、3s 轮询。
- ☑ 2.4 首页 Mock 开关（`src/mocks/devices.ts` + store `mockEnabled` 持久化 + 工具栏 Switch）。

## 3. 验证

- ☑ 3.1 `pnpm exec vue-tsc -b` 通过。
- ☑ 3.2 `pnpm build` 通过。
- ☑ 3.3 浏览器实看首页失败态截图（后端未启动，502 + 重试，符合预期）。
- ☑ 3.4 真机联调下验证设备墙（2 台真机正常显示）；Mock 开/持久化/搜索/关全链路验证。
- ☑ 3.5 别名重命名回归通过：打开编辑 → 自动聚焦无报错（顺手修复 `Input` ref 取 `$el` 的 bug）→ 保存别名出徽标 → 刷新持久化 → 改回原名清除别名。

## 4. 发布

- ☑ 4.1 更新版本摘要（version.md）。
- ☑ 4.2 v0.2.0 已开工并完成投屏核心实施、真机联调，原开工确认事项已关闭。
