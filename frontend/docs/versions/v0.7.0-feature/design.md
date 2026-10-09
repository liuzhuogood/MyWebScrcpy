# v0.7.0-feature 设计文档

## 背景

- 录制/回放/截图均为“后端执行 + 前端编排”：录制复用 H.264 共享流不断投屏；重放切换视频源（经 `player:switch-stream` 语义）；截图分整图上传与选区导出两路。

## 目标与非目标

**目标：**

- 录制：时长滑杆（1~480 分钟默认 30）+ 声音开关 + 开始/停止 + 记录列表（下载/重放跳转/删除）。
- 重放：右侧面板（进度可拖拽、暂停/继续、停止、循环、自定义小段循环、录制列表、服务端恢复）。
- 截图：一键整图上传 + 记录（查看/下载/删除）。
- 选区截图：矩形/圆形双形态、拖拽移动 + 手柄缩放、键盘微调、BBOX JSON 应用、放大镜；保存下载 / 复制图片 / 复制 BBOX / 存为模板（名称 + 设备/全局 → 关闭 v0.6 预留）。

**非目标：**

- 选区手柄八向之外的旋转/多选；录制中实时码率显示（老版本也没有）。

## 关键决策

### 1. 重放与录制共享同一控制器实例

PlayerView 内 `useReplay(serial)` 单例分发给 RecordingPanel（重放跳转/录制互斥）与 ReplayPanel；取代老 `window.__replayStart/CustomEvent` 全局线。`usePlayer` 新增公开 `switchStream(msg)`（断开重连拿新源）。

### 2. 选区状态机收敛到 `useCapture` composable

rect（显示坐标）↔ bounds（画布像素）换算、拖拽/缩放/键盘/BBOX/导出，全部在 composable；CaptureLayer 只做渲染与指针转发。圆形以内切圆处理。

### 3. 存为模板复用模板创建接口

选区 blob + 名称 + serial/global + scene 尺寸 → `POST /api/templates`（与 v0.6 同一后端契约）。

## 风险与权衡

- [重放计时漂移] 沿用老逻辑：本地墙钟为主、后端 loop_count 对齐、暂停累积；短片绕回靠取模不依赖轮询。
- [选区与操控冲突] 选区层打开时画布操控需暂停：复用检查器 guard 思路，`captureActive` 为真时 usePlayer 放行（只 block 左键/触摸，右键返回保留？老代码选区层覆盖画布天然拦截全部；新版同样：层覆盖即拦截，键盘 Esc 关闭）。

## 迁移计划

1. API 层三件套 + `useRecordings` + `useReplay` + 三面板 + `useCapture` + CaptureLayer + 存模板 Dialog。
2. PlayerView 接四个入口 + 重放侧栏 + 选区层 + `switchStream` 联动。
3. 真机联调：短录制（1 分钟档）→ 停止 → 下载/删除；重放开始/暂停/停止；整图截图；选区保存下载 + 存模板（测完删模板）。

## 回滚计划

1. 四个入口隐藏即回滚；新增 `src/replay/`、`src/capture/` 独立目录。

## 待确认问题

- 无。
