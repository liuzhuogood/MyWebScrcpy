# v0.3.0-feature 设计文档

## 背景

- v0.2.0 投屏单设备链路已验证；`ScrcpyDecoder` / `ControlPacker` 可复用。
- 老大屏：每设备独立 WS + 解码 + 触控，卡片工具栏（Home/任务/电源/全屏），横屏自动转框，小/中/大三档，5s 轮询增删。
- 真机 2 台（`10.0.0.11`、`10.0.0.30`），正好验证多卡同屏。

## 目标与非目标

**目标：**

- `/dashboard` 多设备同屏直播，卡片含画面、名称、状态、工具栏（Home/任务/电源/全屏/播放器打开）。
- 三档尺寸切换并记忆；设备上下线自动增删卡片；横屏卡片自动转框。
- 点击/拖拽/滚轮/右键返回可用。

**非目标：**

- Vision 绿框叠加、fps 显示、音频（老版本同样没有；v0.6/v0.7 再议）。
- Mock 设备不接入大屏（假 serial 建连必红屏无意义）。

## 关键决策

### 1. 一卡一组件，生命周期自管理

`DashTile.vue`（props: device）内部建 WS + decoder + packer + 监听，`onUnmounted` 全量销毁；`DashboardView` 只做设备轮询与增删，主从清晰。

### 2. 老代码的电源按钮是死按钮，本版接上

老 `bindToolbar` 只处理了 home/recents/fullscreen，`power` 无分支；新版直接发送 POWER 键（与播放器一致）。

### 3. 尺寸用 Tailwind 档位 + fitCanvas

small/medium/large 对应卡片宽度，高度由画面比例 + fitCanvas 自适应；`dash_size` key 沿用，保证与老页面偏好互通。

## 风险与权衡

- [多路解码] 2 台 H.264 同解对浏览器无压力；设备多了以后由用户自行取舍。
- [WS 数量] 每卡一条 `/ws`，与老行为一致，后端已有实践。

## 迁移计划

1. 新增 `DashTile.vue` + `DashboardView.vue`，路由 `/dashboard` 替换 Stub。
2. 真机联调：双卡出画面 → 点击操控 → 尺寸切换 → 下线一台看自动移除（不断后端，用真机在线状态自然验证；必要时后端重启观察重连）。

## 回滚计划

1. `/dashboard` 出问题指回 Stub；新增两个视图文件独立，删除即回滚。

## 待确认问题

- 无。
