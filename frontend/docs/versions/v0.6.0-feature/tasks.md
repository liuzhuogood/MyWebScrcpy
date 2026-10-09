# v0.6.0-feature 任务清单

> 2026-10-09 状态核对：**已被后续版本替代；前端待同步**。详见 [版本摘要](version.md) 与 [统一状态总览](../../../../docs/versions/README.md)；历史验收不等于本次重新验收。

## 1. 规格与设计

- ☑ 1.1 完成 session/design/tasks/version 初稿。
- ☑ 1.2 研读 template-match.js + 实测线上契约。

## 2. 实现

- ☑ 2.1 `src/api/templates.ts`：CRUD + images + import/export + click + matches。
- ☑ 2.2 `TemplatePanel.vue`：列表/筛选/上传/导入导出/开关/阈值/作用域/参考图/删除/定位点击。
- ☑ 2.3 `useTemplateMatches.ts` + PlayerView 命中 overlay + 绿框开关 + 模板入口。

## 3. 验证（真机）

- ☑ 3.1 定位点击链路通（后端报 screencap 受限，属后端环境问题；前端请求/错误呈现 E2E 通过）。
- ☑ 3.2 开关/作用域/阈值步进/删除 E2E 通过；存量 `chreome` 恢复原状（全局/启用/0.67）；测试模板已清。
- ☑ 3.3 绿框开关切换正常（当前 0 命中，绘制走空路径无报错）。
- ☑ 3.4 `vue-tsc + build` 通过。

## 4. 发布

- ☑ 4.1 版本摘要已更新；2026-10-09 已补充当前状态与验收边界。
