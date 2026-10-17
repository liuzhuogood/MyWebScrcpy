---
name: kuaishou-short-video
description: 快手极速版自动化业务技能：监测红包弹窗、执行连续打卡白拿手机任务，并按连续签到入口观看指定时长的视频。通用设备操作与视觉决策由 my-scrcpy-common 提供。
---

# 快手极速版短视频任务

本技能维护快手极速版的业务目标、页面判断、规则与经验知识。通用设备操作、页面观察与动作脚本由 `my-scrcpy-common` 统一提供。

## 规则与知识库

按需读取：

1. [流程与边界](rules/00-overview.md)
2. [前置条件](rules/01-preconditions.md)
3. [页面与定位](rules/02-pages-and-locators.md)
4. [动作与生命周期](rules/03-actions.md)
5. [验证与报告](rules/04-verify-and-report.md)
6. [弹层与RN页面经验 (Wiki)](wiki/rn-dialogs.md)
7. [金币与收益核对 (Wiki)](wiki/coin-rewards.md)

## 通用操作调用约定

执行外部任务时，通过已安装的 `my-scrcpy-common` 脚本执行原子动作与观察：

```bash
# 1. 观察当前页面状态或获取 AI 视觉决策建议
python3 "${COMMON_SKILL_DIR}/scripts/observe.py" --serial "${SERIAL}" --decision --goal "监测红包弹窗或定位去赚钱入口"

# 2. 刷短视频并执行拟人化随机上滑
python3 "${COMMON_SKILL_DIR}/scripts/watch_video.py" --serial "${SERIAL}" --min-stay 5 --max-stay 15 --count 5 --expected-package com.kuaishou.nebula

# 3. 尝试退出弹层并对比前后页面变化
python3 "${COMMON_SKILL_DIR}/scripts/try_return.py" --serial "${SERIAL}"

# 4. 无法通过返回键退出时冷启回首页
python3 "${COMMON_SKILL_DIR}/scripts/restart_app.py" --serial "${SERIAL}" --package com.kuaishou.nebula
```

## 任务工作目录与产出

所有任务定义、执行进度、操作记录与最终产出均保存在本地任务目录 `~/.mywebscrcppy/tasks/<task-id>/` 中。
按 `task.json` 的产出约定声明生成 `report.md`、`logs/task.log` 及截图。
