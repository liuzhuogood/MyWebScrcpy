---
name: dingtalk-punch
description: 钉钉自动化打卡业务技能：结合模拟定位与界面 OCR 识别，自动化完成考勤打卡。通用设备操作与视觉决策由 my-scrcpy-common 提供。
---

# 钉钉自动化打卡

本技能维护钉钉打卡的业务目标、流程判断、定位要求与成功标准。通用设备操控与状态观察由 `my-scrcpy-common` 统一提供。

## 规则与知识库

按需读取：

1. [打卡流程与边界](rules/01-flow.md)
2. [验证与结果记录](rules/02-verify.md)
3. [组件特征与常见提示 (Wiki)](wiki/locators.md)

## 通用操作调用约定

执行外部任务时，通过已安装的 `my-scrcpy-common` 脚本执行原子动作与观察：

```bash
# 1. 观察当前设备状态或获取 AI 视觉决策建议
python3 "${COMMON_SKILL_DIR}/scripts/observe.py" --serial "${SERIAL}" --decision --goal "识别钉钉打卡按钮或异常提示"

# 2. 检查前台应用或启动钉钉
python3 "${COMMON_SKILL_DIR}/scripts/open_app.py" --serial "${SERIAL}" --package com.alibaba.android.rimet

# 3. 尝试退出异常弹窗
python3 "${COMMON_SKILL_DIR}/scripts/try_return.py" --serial "${SERIAL}"
```

## 任务工作目录与产出

所有任务定义、执行进度、操作记录与最终产出均保存在本地任务目录 `~/.mywebscrcppy/tasks/<task-id>/` 中。
按 `task.json` 的产出约定声明生成 `report.md`、`logs/task.log` 及现场截图。
