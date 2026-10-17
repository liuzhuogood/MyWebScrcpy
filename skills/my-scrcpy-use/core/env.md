# 执行环境与工作目录规范

1. **统一工作目录**：
   - 任务在系统统一任务根目录（默认为 `~/.mywebscrcppy/tasks/<task-id>/`）下创建与执行，取消强制临时目录、结束自动清理和默认录屏要求。
   - 外部编排助手直接使用规范的工作目录，启动任务前确保服务地址与设备 serial 已明确，不使用默认 8080 或猜设备。

2. **环境变量注册**：
   - 脚本继承已有环境，可通过 [run-with-env.py](../scripts/run-with-env.py) 为任务进程树注册下列变量：

| 环境变量 | 用途 |
|---|---|
| `MYWEBSCRCPY_API_URL` | 服务根地址 (例如 `http://127.0.0.1:15272`) |
| `MYWEBSCRCPY_SERIAL` | 设备 serial |
| `MYWEBSCRCPY_TASK_DIR` | 本次任务工作目录 (例如 `~/.mywebscrcppy/tasks/<task-id>/`) |
| `MYWEBSCRCPY_TASK_ROOT` | 任务根目录 (默认为 `~/.mywebscrcppy/tasks`) |

3. **产出规范**：
   - 日志 (`logs/task.log`)、报告 (`report.md`)、截图 (`screenshots/`) 和可选录屏 (`videos/`) 均按 `task.json` 声明的相对路径直接写入任务目录中。
   - 目录持久保留，供任务中心在用户主动触发“扫描刷新”时读取并展示。

```bash
python3 "$skill_dir/scripts/run-with-env.py" \
  --base-url "$base_url" --serial "$serial" --task-id "$task_id" -- \
  python3 "$task_script"
```
