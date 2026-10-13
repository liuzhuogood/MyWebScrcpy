# 全局模板匹配服务

在「系统配置 → 匹配」点击「检测依赖」，环境可用后点击「启动」。Go 启动和重启不会自动运行匹配进程。服务运行时，一个 Python 进程负责所有当前投屏设备；关闭投屏或禁用该设备全部模板后，对应匹配任务在下一轮设备检查时退出。没有投屏设备时总服务仍运行，设备数为 0。

需在实际配置的 Python 3.10+ 中安装 httpx、websocket-client、av、opencv-python-headless 和 numpy。使用 SDK 源码时，可将下面的 python3 替换为检测显示的解释器路径：

```sh
python3 -m pip install './packages/python-sdk[vision]'
```

应用不自动安装这些依赖。依赖不完整时不能启动，检测显示缺少的模块。修改 Python 路径后需停止服务再检测，启动也会重新检查。

模板启用且算法受 SDK 支持时，匹配结果通过现有 Vision 接口发布，播放器的匹配框开关继续生效。当前 SDK 支持主图片、TM_CCOEFF_NORMED、阈值、灰度与缩放；多参考图片与其他算法仍按 SDK 能力限制处理。运行中不代表一定命中，静止画面没有新帧时显示「等待画面」。自动服务只识别，不点击或执行 ADB。

模板管理中可直接进入系统配置启动服务；独立 SDK/旧 CLI 发布结果的入口继续可用。进程异常退出时显示错误，需要手动重新启动。

接口：GET `/api/matching-service/status`；POST `/api/matching-service/check`、`/start`、`/stop`。完整路径和响应结构见应用 API 文档。停止会清理匹配连接和进程，超过退出期限才强制结束本次服务的进程。
