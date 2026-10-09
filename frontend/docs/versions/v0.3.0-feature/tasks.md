# v0.3.0-feature 任务清单

> 2026-10-09 状态核对：**已完成；部分延期**。详见 [版本摘要](version.md) 与 [统一状态总览](../../../../docs/versions/README.md)；历史验收不等于本次重新验收。

## 1. 规格与设计

- ☑ 1.1 完成 session/design/tasks/version 初稿。
- ☑ 1.2 研读 dashboard.js，范围收敛（不含 vision/fps/音频）。

## 2. 实现

- ☑ 2.1 `DashTile.vue`：单卡 WS + 解码 + 触控 + 工具栏 + 全屏 + 横竖转框（电源按钮按播放器语义接上，老代码是死按钮）。
- ☑ 2.2 `DashboardView.vue`：设备轮询增删 + 三档尺寸 + 空/失败态（只接真实设备）。
- ☑ 2.3 路由 `/dashboard` 替换 Stub。

## 3. 验证（真机双设备）

- ☑ 3.1 双卡同屏出画面（2 台设备直播中，无状态徽标即直播态）。
- ☑ 3.2 卡片 Home 操控 E2E（11 号机 page-info 落到 NexusLauncher）。
- ☑ 3.3 尺寸切换生效（小→中，卡片几何拉开；`dash_size` 持久化沿用老 key）；设备增删走 5s 轮询 diff（代码同老逻辑）。
- ☑ 3.4 `vue-tsc + build` 通过。

## 4. 发布

- ☑ 4.1 版本摘要已更新；2026-10-09 已补充当前状态与验收边界。
