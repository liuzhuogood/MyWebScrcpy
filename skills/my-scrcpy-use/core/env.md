# 执行环境规范

1. 执行前统一通过 [run-with-env.py](../scripts/run-with-env.py) 注册环境并启动任务；确认服务地址与设备，不写默认目标。
2. 脚本继承已有环境，注册下列变量并传给任务及其子进程；任务读取后显式传入 SDK，不把变量值写入日志。

| 环境变量 | 用途 |
|---|---|
| `MYWEBSCRCPY_API_URL` | 服务根地址 |
| `MYWEBSCRCPY_SERIAL` | 设备 serial |
| `MYWEBSCRCPY_RUN_DIR` | 本次独立临时执行目录，同时作为任务工作目录 |
| `TMPDIR` / `TEMP` / `TMP` | 均指向本次执行目录 |

3. 日志、截图、录屏、下载文件、失败证据和中间文件均保存到 `MYWEBSCRCPY_RUN_DIR` 内；显式输出路径也须在该目录内。工作目录和环境变量不能限制程序主动写入其他绝对路径。
4. 注册仅作用于本次任务进程树，不修改父 shell 或系统配置；收尾按[执行规范](execution.md)清理，保留证据仍留在本次目录内。

以下变量均使用已确认的值，`skill_dir`、`task_script` 为绝对路径：

```bash
python3 "$skill_dir/scripts/run-with-env.py" \
  --base-url "$base_url" --serial "$serial" -- \
  python3 "$task_script"
```
