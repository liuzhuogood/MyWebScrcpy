# 外部任务运行集成

编排、定时和执行由 Codex 等外部工具负责。MyWebScrcpy 提供设备 API，并在 `/tasks` 展示执行端上报的状态、日志和结果。第一版一个运行对应一个设备。

## Python SDK

从本仓库安装包含本版功能的 SDK：

```bash
python3 -m pip install ./packages/python-sdk
```

```python
from mywebscrcpy import TaskReporter

with TaskReporter(
    "http://127.0.0.1:8080",
    name="每日检查",
    device_serial="phone-serial",
    source="codex",
) as run:
    with run.capture_output():
        print("开始检查")
        # 在此调用公开设备 SDK / API，实际执行检查。
    run.finish("succeeded", exit_code=0, summary="检查完成",
               report_markdown="# 结果\n\n检查通过。")
```

不需要专用上报令牌。业务判定失败时显式调用 `finish("failed", ...)`。上下文中抛出异常会尝试上报失败；仅在正常退出且未显式结束时自动上报成功。不要用模型总结或退出码代替业务检查。

SDK 默认每 10 秒上传缓冲日志并发送心跳；可主动 `flush()`。日志按序号分批发送，最多缓存 5000 条，缓冲满会明确报错，不跳号丢弃。`finish()` 先上传全部待传日志，失败时保留本地 `.task-run-error-*.json` 证据并抛错；不得将此时的面板状态当作已完成。证据文件权限为 0600。

## HTTP 契约

当前结构以 `/api/openapi.json` 为准；请求仅接受已定义字段。运行写入不要求额外 Authorization。

| 请求 | 用途 |
| --- | --- |
| `POST /api/task-runs` | 提交 `client_run_id`、`name`、`device_serial` 和可选 `source`；相同标识及元数据去重，不同内容 409 |
| `POST /api/task-runs/{id}/heartbeat` | 保活 |
| `POST /api/task-runs/{id}/logs` | `entries` 数组，每条含 `seq`、`timestamp`（毫秒）、`stream`（stdout/stderr）、`message`；从 1 连续递增，冲突或跳号拒绝 |
| `POST /api/task-runs/{id}/finish` | `status` 为 succeeded/failed/cancelled，可附 `exit_code`、`summary`、`report_markdown`；相同结果幂等，冲突 409 |
| `GET /api/task-runs` | 分页、设备、来源和状态筛选；`connectivity=disconnected` 只查询失联的运行中任务 |
| `GET /api/task-runs/{id}` | 详情 |
| `GET /api/task-runs/{id}/logs` | 用 after/limit 增量读取；`download=true` 下载纯文本 |

60 秒无心跳时派生“已失联”，执行状态仍为 running，结果未知；恢复心跳可恢复在线。服务重启不恢复或终止外部进程。终态不会被迟到心跳改写，也不接受新增日志。单次日志请求上限 4 MiB，单运行最多保存 50000 条日志；到达上限会标记截断，SDK 对未确认日志报错。

报告仅安全渲染 Markdown，不执行脚本或加载嵌入外部媒体。网页不提供启动、停止外部脚本或安排定时的操作。

## Codex 接入

将 `.agents/skills/my-scrcpy-task-run/` 安装到实际执行项目的技能目录，配置服务地址并使用确定的设备序列号。先手动执行并在任务面板检查实时日志与结果，再由外部工具设置定时。每次实际执行生成新的 `client_run_id`，网络重试沿用同一标识。

## v1.25 数据策略

应用遇到旧数据库结构时，在同一事务中删去旧表、创建当前结构并记录版本 12500。旧数据库全部记录直接重置，不迁移、不保留历史查询或导出接口；新数据库重启保留记录。此行为不删除数据库外的录制、模板或用户脚本。发布正式升级前，应明确告知数据库重置效果。
