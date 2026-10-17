---
name: my-scrcpy-common
description: MyWebScrcpy 通用操作技能：提供设备状态观察（含 AI 视觉决策）、拟人化看视频、尝试返回（ESC/BACK）、打开及重启 APP 等共享操作脚本与规则。应用技能专注业务规则与页面 Wiki。
---

# 通用操作技能 (my-scrcpy-common)

本技能提供 MyWebScrcpy 的通用设备观察与操作工具集。应用业务判断（如快手金币领取、钉钉打卡）留在各自技能的规则与外部助手决策中，底层通用请求、拟人动作与设备状态统一复用本技能。

## 核心操作能力

| 通用操作 | 脚本与入口 | 说明与验证 |
|---|---|---|
| **观察页面** | `scripts/observe.py` | 读取设备总状态 `GET /api/device-state/{serial}`；若指定 `--decision` 则调用 `POST /api/device-state/{serial}/decision` 获取 AI 视觉建议 |
| **看视频** | `scripts/watch_video.py` | 随机停留（默认 5～15 秒），执行贝塞尔拟人化上滑，总时限约束，定期交回助手判断页面 |
| **尝试返回** | `scripts/try_return.py` | 发送 `KEYCODE_ESCAPE` (111)，获取前后状态对比判断弹层是否消除；不可靠时标记“未知” |
| **Android 返回** | `scripts/android_back.py` | 发送标准 `KEYCODE_BACK` (4)，与 ESC 明确区分 |
| **打开 APP** | `scripts/open_app.py` | 根据包名与可选 Activity 启动应用，并验证前台包名与页面状态 |
| **重启 APP** | `scripts/restart_app.py` | 停止指定包名并重新启动，验证实际前台状态 |

## 使用规范

1. **观察优先**：在执行任何点击或滑动前，先调用 `observe.py` 获取当前屏幕快照、前台包名/Activity 及 OCR 内容。
2. **决策建议不自动执行**：`observe.py --decision` 返回的 AI 分析结果绑定单轮观测，仅供助手决策参考，脚本自身绝不自动触发设备动作。
3. **返回判断谨慎**：像素或画面变动只能作为辅助证据，不能证明返回成功。无法确认时记录“未知”，由助手重新观察。
4. **受控工作目录**：任务执行过程在独立工作目录维护（`~/.mywebscrcppy/tasks/<task-id>/`），不需要向服务端上报心跳或上报日志。
