# 快手任务中心弹层与页面特征知识 (Wiki)

## 页面特征与渲染模式

1. **React Native (RN) 渲染特性**：
   - 快手“任务中心/去赚钱”主页面采用 RN 框架渲染。
   - Android 原生 Accessibility / UIAutomator XML 树通常仅反映下层原生容器，XML 中获取的 `bounds` 与实际触摸区可能存在垂直偏移。
   - **关键准则**：在 RN 页面中，优先使用 OCR 画面文字框中心坐标执行点击，结合 `my-scrcpy-common/scripts/observe.py` 的 OCR 结果。

2. **弹层轮播与返回键穿透**：
   - 每次进入任务中心时，系统可能轮播弹出多重活动弹窗（如签到提醒、打卡红包、翻倍奖励、好友推荐等）。
   - 普通 `KEYCODE_BACK` 经常被 RN 弹层拦截或失效。
   - **分级处理策略**：
     1. 识别并点击弹窗关闭文字/图标（如“关闭”、“去领取”、“立即打卡”）。
     2. 调用 `my-scrcpy-common/scripts/try_return.py` 发送 `KEYCODE_ESCAPE` (111) 并对比页面前后状态。
     3. 若仍无法退出弹窗，调用 `my-scrcpy-common/scripts/restart_app.py` 强制冷启回首页。
