# 外部任务工作目录与产出规范 (v1.27.1)

编排、调度和执行由外部助手（如 Codex、WorkBuddy、Claude Code）负责。服务负责观察任务工作目录、提供设备状态与操作能力。

## 1. 任务工作目录约定

默认工作目录为 `~/.mywebscrcppy/tasks/`（`~` 为宿主机用户主目录）。可在系统设置的服务区域修改。

每个任务使用独立目录：

```text
~/.mywebscrcppy/tasks/<task-id>/
├── task.json          # 任务定义与产出声明
├── state.json         # 最新进度、业务结果（原子更新）
├── actions.jsonl      # 追加式操作记录
├── report.md          # 默认报告文件
├── logs/task.log      # 默认日志文件
├── screenshots/       # 截图目录
└── videos/            # 视频文件
```

- 状态文件 `state.json` 采用临时文件加原子重命名（rename）写入。
- 操作日志 `actions.jsonl` 以 JSON Lines 格式追加写入。
- 任务执行中断后可从当前目录和实际设备状态继续。

## 2. 产出契约与访问

任务在 `task.json.outputs` 中显式声明各项产出：

| 产出类别 | 默认路径 | 格式 | 要求 |
|---|---|---|---|
| 报告文件 | `report.md` | Markdown | 必需 |
| 日志文件 | `logs/task.log` | UTF-8 文本 | 必需 |
| 截图目录 | `screenshots/` | PNG/JPEG | 可选 |
| 视频文件 | `videos/recording.mp4` | MP4 | 可选 |

- 路径严格限制在本次任务目录内，禁止路径穿越或符号链接越界。
- 业务结果与产出完整性分别呈现；仅在业务成功且所有必需产出均完整时才展示为“全部完成”。

## 3. 扫描与查询接口

服务不做后台轮询、文件监听或定时自动扫描。

| 接口 | 方法 | 说明 |
|---|---|---|
| `/api/task-runs/scan` | `POST` | 用户手动主动扫描当前根目录并重建内存索引 |
| `/api/task-runs` | `GET` | 查询最近一次扫描的任务列表（支持过滤与分页） |
| `/api/task-runs/{id}` | `GET` | 直接读取指定任务目录详情与产出状态，不隐式扫描根目录 |
| `/api/task-runs/{id}/outputs/{output_id}` | `GET` | 安全读取、预览或下载已声明的产出文件 |
