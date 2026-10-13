# MyWebScrcpy Python SDK

SDK 使用 HTTPX，支持 `Client`、绑定设备的 `Device.adb()`、`Device.page_info()` 和类型化结果。按键、点击、滑动及 ASCII 输入可通过明确的 ADB 参数执行。当前尚不提供视频解码、模板管理和匹配 API，也无需安装 OpenCV/PyAV。

使用 Python 3.10+，从 PyPI 安装：

```bash
python -m pip install mywebscrcpy-sdk
```

也可从仓库根目录安装源码并运行示例：

```bash
python -m pip install ./packages/python-sdk
python packages/python-sdk/examples/device_query.py DEVICE_SERIAL --base-url http://HOST:PORT
```

也可构建并安装 wheel：

```bash
python -m pip install build
python -m build packages/python-sdk
python -m pip install --find-links packages/python-sdk/dist mywebscrcpy-sdk
```

```python
from mywebscrcpy import Client

with Client("http://HOST:PORT") as client:
    with client.device("DEVICE_SERIAL") as device:
        result = device.adb(["shell", "getprop", "ro.product.model"], timeout=10)
        print(result.stdout)
        device.adb(["shell", "input", "keyevent", "3"], timeout=10)
        size = device.page_info()
        device.adb(["shell", "input", "tap", str(size.width // 2), str(size.height // 2)], timeout=10)
```

`timeout` 为秒，SDK 转换为接口的 `timeout_ms`。ADB 参数不得包含可执行文件名、设备选择或宿主切换选项。SDK 检查 HTTP 响应及实际 `status/exit_code`；失败、截断输出或超时抛异常，不自动重试动作。HTTP 超时后的设备结果可能未知，不应直接重发。

`CommandResult` 包含 `status/exit_code/stdout/stderr/truncated`；`PageInfo` 包含正整数 `width/height`。必需字段缺失或类型错误抛 `ProtocolError`，不猜替代字段。`CommandError.result` 保留命令结果，`HTTPError` 保留 `status/code/detail`；连接错误使用 `ConnectionError`。

`Client(headers=...)` 接收显式请求头，默认验证 TLS，可传 CA 路径。不会读取未知凭据、自动启动主服务或修改 root logger。Client/Device 可重复关闭，关闭后的对象拒绝操作。

应用任务会释放同一套 SDK 源文件，并提供连接环境变量；配置的 Python 环境仍需安装 `httpx`。导出的脚本可在其他目录通过安装本 SDK 独立运行，远程操作要求服务与设备可达。

接口依据 `web/openapi.json` 的 `/api/adb/commands` 和 `/api/ui/page-info`。安装版与应用内释放版共享源码，避免两套运行库。仅本机和受控接口已验证，真机及 Windows/Linux 原生链路仍待验收。

SDK 版本与仓库根目录 `VERSION` 一致，Python 包版本去掉 `v` 前缀。修改 `VERSION` 后运行 `node scripts/sync-version.mjs`，同时更新主程序与 SDK 版本。

GitHub Actions 的 `Publish Python SDK` 工作流在主分支的版本、SDK 或发布配置变更时运行，也支持 `v*` 标签和手动触发；构建检查及安装测试通过后发布未发布的版本，已存在的完整版本会跳过。仓库需在 Settings → Secrets and variables → Actions 配置 `PYPI_API_TOKEN`，值为有此项目发布权限的 PyPI API Token。只修改 SDK 而不更新版本号不会覆盖 PyPI 上已有的版本。
