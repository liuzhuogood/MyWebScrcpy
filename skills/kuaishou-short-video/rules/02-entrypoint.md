# 快手任务入口与生命周期规范

本文档定义快手日常任务的执行入口、工作目录约定与状态更新规则。

## 1. 任务工作目录与初始化

1. **工作目录定位**：
   - 任务在系统统一任务根目录（默认为 `~/.mywebscrcppy/tasks/<task-id>/`）下创建与执行，不使用不可发现的临时目录。
   - 外部编排助手在启动任务前，先在任务目录下写入 `task.json` 声明任务定义与产出列表。

2. **产出声明示例 (`task.json`)**：
   ```json
   {
     "name": "快手极速版日常签到与视频任务",
     "device_serial": "<SERIAL>",
     "goal": "完成快手日常签到并浏览短视频获取奖励",
     "success_condition": "成功完成签到且生成有效结果报告与日志",
     "timeout_seconds": 300,
     "source": "kuaishou-skill",
     "outputs": [
       {"id": "report", "category": "report", "format": "markdown", "path": "report.md", "required": true},
       {"id": "log", "category": "log", "format": "text/utf-8", "path": "logs/task.log", "required": true},
       {"id": "shots", "category": "screenshot", "format": "image/png", "path": "screenshots/", "required": false, "allowed_types": [".png", ".jpg"]},
       {"id": "video", "category": "video", "format": "video/mp4", "path": "videos/recording.mp4", "required": false}
     ]
   }
   ```

## 2. 状态原子更新与操作流水

1. **最新状态 (`state.json`)**：
   - 阶段（stage）与进度（progress）变化时，写入临时文件并原子替换至 `state.json`。
   - 状态包括：`pending`（待执行）、`running`（运行中）、`paused`（暂停）、`completed`（完成）、`failed`（失败）、`cancelled`（取消）。
   - 结束时注明 `ended_reason`（如 `completed`、`timeout`、`device_offline` 等）。

2. **操作流水 (`actions.jsonl`)**：
   - 每一个动作（如点击、滑动、按键）执行后，向 `actions.jsonl` 追加一行 JSON 记录：
     `{"timestamp": 1792226733000, "action": "click_sign_in", "result": "success"}`

3. **结果报告 (`report.md`)**：
   - 任务结束时生成完整的 Markdown 执行报告，记录签到结果、视频浏览统计、耗时及产出文件链接。
   - 任务中心通过用户主动“扫描刷新”发现并展示该任务及其产出。不强制要求生成独立可执行 Python 块或上报旧接口。
