# 原型边界

- 遵循 `../frontend/AGENTS.md`、`../frontend/docs/rules/coding.md` 和 `../frontend/docs/rules/ui.md`。
- 原型任务默认只修改本目录；`../frontend/src/` 的组件、主题和业务代码只读复用。
- 每个方案位于 `cases/<功能名>-<版本>/`，包含 `meta.ts` 和 `Prototype.vue`；预览入口自动发现，不修改正式路由。
- 使用已有 UI 组件与主题变量。普通结构元素可以使用 Vue 模板，不另造按钮、表单、弹窗等基础控件。
- 状态与数据在原型内闭环，不初始化正式应用、业务 Store、真实请求、设备控制或持久化操作。有副作用的业务组件须提供明确的模拟依赖后再使用。
- 定稿接入正式业务属于用户另行授权的实现任务；保留原型版本，不自动删除。
- 原型源码入库；构建和验证产物放入已忽略的 `dist/`、`artifacts/`。

## 验证与门禁规范

- **门禁精确分级**：纯原型或文档修改提交前，仅需运行 `pnpm --dir frontend proto:check`（必要时 `proto:build`）；严禁在纯原型任务中执行全量后端 `go test` 或正式前端生产打包 `frontend build`。
- **固化截图工具**：界面验证复用 `scripts/capture.mjs`，严禁临时手写复杂 WebSocket / CDP 脚本。
