# v0.4.0-feature 任务清单

> 2026-10-09 状态核对：**已实现；待验收**。详见 [版本摘要](version.md) 与 [统一状态总览](../../../../docs/versions/README.md)；历史验收不等于本次重新验收。

## 1. 规格与设计

- ☑ 1.1 完成 session/design/tasks/version 初稿。
- ☑ 1.2 研读 files.js/html，接口与交互范围收敛。

## 2. 实现

- ☑ 2.1 `src/api/files.ts`：7 组接口 + XHR 上传 + 下载 helper。
- ☑ 2.2 `src/files/useFiles.ts` + `fileKind.ts`：列表/选择/分页/上传队列/撤销 + 类型徽标/预览判定。
- ☑ 2.3 `FilesView.vue`：面包屑/搜索/筛选/表格/批量条/预览/上传队列（Dialog/AlertDialog/Sonner/Progress 全 shadcn）。
- ☑ 2.4 路由 `/files` + 投屏更多菜单入口 + App 全局 Toaster。

## 3. 验证（真机 `10.0.0.30`）☑ 闭环，测试残留已清（总数回到 55）

- ☑ 3.1 列表/容量/类型徽标/行操作/面包屑/文件夹进入；全文搜索因后端全盘扫描慢未测。
- ☑ 3.2 建文件夹/重命名/移动（搬进 `__vtest__`，API 核实）/文本预览（内容正确）/下载（无报错）/上传契约（curl 直调 `path/conflict/file` 通过；面板无法选文件，上传队列 UI 仅代码层）。
- ☑ 3.3 删除 + 撤销 E2E 通过（删后空态 + Sonner 撤销 → 文件恢复，API 核实）。
- ☑ 3.4 `vue-tsc + build` 通过；顺手补预览 Dialog 的 sr-only Description（消 Reka a11y 警告）。

## 4. 发布

- ☑ 4.1 版本摘要已更新；2026-10-09 已补充当前状态与验收边界。
