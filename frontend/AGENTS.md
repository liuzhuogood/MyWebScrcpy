## 项目规范加载

- 普通任务无需读取 `docs/rules/`。
- 修改、审查或生成代码前，读取 `docs/rules/coding.md`（存在时）。
- 设计、实现或审查界面前，读取 `docs/rules/ui.md`（存在时）。
- 界面任务同时涉及代码时，两份规范都读取。
- 冲突优先级：用户当前指令 > 项目现有约定 > `docs/rules/` 通用规范。

## 组件规范

- 界面优先使用 shadcn-vue 组件（`src/components/ui/*`，Reka UI + Lucide 图标）；缺失才用 `pnpm dlx shadcn-vue@latest add <组件>` 按需添加，不自造基础组件、不全局改主题 token。
