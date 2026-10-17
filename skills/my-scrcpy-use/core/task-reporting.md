# 外部任务工作目录与记录约定

1. **工作目录规范**：
   - 任务在宿主机本地目录 `~/.mywebscrcppy/tasks/<task-id>/` 中执行与记录，不通过 HTTP 接口上报心跳、日志或结果。
   - 每个任务独立目录，包含 `task.json`、`state.json`、`actions.jsonl`、`report.md`、`logs/task.log`、`screenshots/`、`videos/`。
2. **状态文件原子更新**：
   - `state.json` 保存当前阶段、进度与结果，通过临时文件加原子重命名（rename）更新。
   - `actions.jsonl` 以 JSON Lines 格式追加记录操作事件。
3. **产出约定**：
   - 在 `task.json.outputs` 中显式声明产出标识、类别、格式与相对路径。
   - 业务结果与产出完整性分别记录；服务通过用户主动“扫描刷新”发现并展示任务。
