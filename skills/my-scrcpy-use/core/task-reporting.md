# 外部任务运行上报

1. 使用用户已确定的服务地址，读取 `/api/openapi.json` 核对实际字段，不猜测接口或设备序列号。
2. 在执行环境安装包含 `TaskReporter` 的本版 Python SDK（本仓库 `python3 -m pip install ./packages/python-sdk`）；不安装或运行内置 Pi。
3. 为每次实际执行创建新运行；网络重试沿用 `client_run_id`。不需要额外上报令牌。
4. 使用 `capture_output()` 显式采集当前脚本的 stdout/stderr；交互式 Codex 步骤仅能上报实际阶段事件，不声称获取完整进程输出或私有思考。
5. 按实际业务结果提交 succeeded/failed/cancelled、摘要和可选 Markdown。上传失败必须报告，并保留 SDK 生成的本地失败证据，不能声称面板已收到。
6. 在 `/tasks` 检查运行、实时日志及最终结果。失联是连接状态，不能据此判断业务失败、已停止或完成。
7. 用户要求定时时交给外部工具本身；MyWebScrcpy 不负责调度或停止外部进程。先手动验证再配置定时。

```python
from mywebscrcpy import TaskReporter

with TaskReporter("http://127.0.0.1:8080", name="检查任务",
                  device_serial="明确的设备序列号", source="codex") as run:
    with run.capture_output():
        print("开始执行")
        # 在这里完成实际设备检查。
    run.finish("succeeded", exit_code=0, summary="检查通过")
```

详细接口与容量限制见[项目任务运行文档](../../../docs/task-runs.md) 和当前 OpenAPI。任务日志不写入密钥、令牌或完整敏感请求。
