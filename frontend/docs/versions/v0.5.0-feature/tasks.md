# v0.5.0-feature 任务清单

> 2026-10-09 状态核对：**已实现；验收记录待核实**。详见 [版本摘要](version.md) 与 [统一状态总览](../../../../docs/versions/README.md)；历史验收不等于本次重新验收。

## 1. 规格与设计

- ☑ 1.1 完成 session/design/tasks/version 初稿。
- ☑ 1.2 研读 ui-inspector.js + op-log.js，范围收敛（不含拖拽调宽）。

## 2. 实现

- ☑ 2.1 `src/api/ui.ts` + `src/inspector/guard.ts` + `opLog.ts`。
- ☑ 2.2 检查器 Panel + 4 Tab 组件（取色/XML/页面信息/操作日志）。
- ☑ 2.3 `usePlayer` 接 `onOp` + guard；PlayerView 加检查入口与右侧面板。

## 3. 验证（真机 `10.0.0.30`）

- ☑ 3.1 取色读数 + 固定；检查期间点击不注入（日志无 tap 记录）。
- ☑ 3.2 XML 获取 + 树渲染 + XPath 测试（4 匹配）+ 页面信息 + 复制按钮。
- ☑ 3.3 操作日志：HOME 记录 + 描述/ADB 切换 + 复制/清空。
- ☑ 3.4 `vue-tsc + build` 通过；解码 canvas 加 `willReadFrequently`。
- ⬜ 3.5 【待核实】原清单勾选放大镜目视确认通过，但 session.md 与 version.md 均记录待确认；补充明确确认结果后再关闭。

## 4. 发布

- ☑ 4.1 版本摘要已更新；2026-10-09 已补充当前状态与验收边界。
