# v0.7.0-feature 任务清单

> 2026-10-09 状态核对：**已实现；待真机验收**。详见 [版本摘要](version.md) 与 [统一状态总览](../../../../docs/versions/README.md)；历史验收不等于本次重新验收。

## 1. 规格与设计

- ☑ 1.1 完成 session/design/tasks/version 初稿。
- ☑ 1.2 研读 recording/replay/screenshot/选区逻辑。

## 2. 实现

- ☑ 2.1 API 层：recordings/replay/screenshots。
- ☑ 2.2 `useRecordings` + 录制面板；`useReplay` + 重放面板。
- ☑ 2.3 截图面板（一键上传 + 记录）。
- ☑ 2.4 选区截图（useCapture + CaptureLayer + 存模板）。
- ☑ 2.5 PlayerView 接线（四入口 + switchStream 联动）。

## 3. 验证（真机）

- ⬜ 3.1 短录制→停止→下载/删除；重放→暂停→停止。
- ⬜ 3.2 整图截图→查看/下载/删除；选区保存下载 + 存模板（测完清理）。
- ☑ 3.3 `vue-tsc + build` 已在 2026-10-09 前端合并与部署流程通过；不代替 3.1/3.2 真机验收。

## 4. 发布

- ☑ 4.1 版本摘要已更新；2026-10-09 已补充当前状态与验收边界。
