# MyWebScrcpy Python SDK

`mywebscrcpy-sdk`（版本与根目录 VERSION 一致），导入名 `mywebscrcpy`，支持 Python 3.10+。HTTP 使用 HTTPX，Vision 使用 websocket-client；视频在调用方进程通过 PyAV 解码、OpenCV 匹配，无需运行独立匹配工程。远程操作要求主服务和设备可达。

## 安装

从仓库根目录安装基础能力或识图能力：

```bash
python -m pip install ./packages/python-sdk
```

核心依赖包含 HTTPX、websocket-client、PyAV、OpenCV (opencv-python-headless) 与 NumPy，开箱即用支持模板资产、设备列表、ADB、页面尺寸、视频取帧与模板匹配。基础 import 保持懒加载，仅在实际取帧与匹配时加载 PyAV、OpenCV 与 NumPy。仍保留 `[vision]` 兼容标识。

构建并安装 wheel：

```bash
python -m pip install build
python -m build packages/python-sdk
python -m pip install --force-reinstall 'packages/python-sdk/dist/mywebscrcpy_sdk-1.20.1-py3-none-any.whl[vision]'
```

安装后脚本可以放在其他目录运行。外部脚本和全局匹配服务使用其 Python 环境中安装的 SDK。系统配置的 Python 卡片通过 pip 安装同版本 SDK 及识图依赖；服务仍需手动启动。

## 匹配与操作

```python
from mywebscrcpy import Client

with Client('http://HOST:PORT') as client:
    templates = client.templates.list(serial='DEVICE_SERIAL')
    with client.device('DEVICE_SERIAL') as device:
        frame = device.frame(timeout=10)
        matches = device.match('CONFIRMED_TEMPLATE_ID', timeout=10)
        print(matches)
        # 只有显式动作才操作设备；先确认目标模板 ID。
        target = device.wait_for('CONFIRMED_TEMPLATE_ID', timeout=10)
        result = device.tap_match(target, max_age_ms=600)
```

`frame()` 等待调用后到达的新画面。重放的缓存关键帧只初始化解码，不满足取帧请求；静止画面可能超时。`match()` 正常未命中返回 `[]`，`wait_for()` 到期抛 `MatchTimeout`。仅支持模板主图片与 `TM_CCOEFF_NORMED`；未知算法、禁用模板、其他设备模板明确报错。多参考图不会自动聚合，无 OCR。

`Frame.image` 为只读 uint8 BGR ndarray。`Match` 带模板 ID、名称、置信度、归一化框及 `center`、来源设备/会话/连接/重置代数、帧 ID 和本地接收时间。`tap_match()` 拒绝旧连接、旧配置与过期结果；仍不能消除画面变化与点击之间的竞态。

| API | 约定 |
| --- | --- |
| `client.devices.list()` | 精确解析 `/api/devices` 数组 |
| `client.templates.list/get/create/update/delete` | 创建和更新图片使用 bytes，SDK 编码为 `image_base64` |
| `client.templates.image/add_image` | 主 PNG 下载、multipart `image` 参考图上传 |
| `client.templates.import_zip/export_zip` | ZIP bytes；导入使用 multipart `file` + `serial` |
| `device.adb(args, timeout=30)` | 明确参数数组；不含 adb 或宿主/设备选择参数；检查真实退出状态 |
| `device.page_info(timeout=10)` | 返回正整数 width/height |
| `device.tap(point)` / `swipe(start, end, duration_ms=300)` | 归一化坐标 0..1；滑动时长为毫秒 |
| `device.key(keycode, meta_state=0)` / `text(text)` | uint32 按键；文本最多 300 UTF-8 字节，具体字符输入能力需设备验证 |
| `Matcher.match_image(image, template, ...)` | 内存 BGR 图像离线匹配，无网络，返回归一化框与分数 |
| `device.watch_templates(publish=True, max_fps=5, timeout=10)` | 使用上下文；按指定频率重新匹配当前画面，每批有总期限；每设备一个 watch |

`watch_templates(publish=True)` 将结果回传网页播放器，空结果与关闭都会清框；异常中断时服务端 TTL 兜住残留显示。每 10 秒刷新模板；刷新失败中断 watch，不以旧缓存继续。完整编码包持续解码，只保存最新画面，不因慢匹配堆积历史帧。每个 Device 一个收包线程，动作结果按 request_id 分发。

静止画面没有新视频帧时，watch 继续匹配最后一张有效画面并刷新绿框；新帧到来立即替换缓存，旋转、编码器重置、断流和关闭使缓存失效。绿框的 TTL 按本轮检测刷新，图像的 `frame_id`、`received_at` 不变，`tap_match` 仍拒绝超过 600ms 的图像。`frame`、`match`、`wait_for` 继续等待新帧。

Vision 订阅携带 `current_frame=true`，服务器回放有大小上限的完整关键帧与增量帧链，并在末包标记 `snapshot_complete=true`，用于静止时启动 watch。未标记完整性的回放只用于初始化解码器；缓存超限则等待新关键帧。回放匹配结果标记 `replayed=True`，不能用于点击。连接每 5 秒发送心跳，5 秒内没有响应会结束检测；停止发布后，绿框按 TTL 清除。

## 异常与资源

Client、Device、watch 使用 `with` 或显式 `close()`，可重复关闭。断流终止当前连接，由调用方重新创建设备对象；关闭会唤醒等待者并回收线程。标准 logging 不修改 root logger、不记录凭据、图片或输入文本。

超时单位为秒，范围 `(0, 600]`。视觉方法有总期限，覆盖模板请求、连接、取帧和匹配。ADB 保留 v0.1 的命令期限及传输余量。`ProtocolError` 表示确切字段/格式错误，不猜替代字段；`HTTPError` 保留 status/code/detail，`ConnectionError` 表示连接问题。`ActionError.result` 保留 accepted/executed/error_code；`ActionTimeout` 表示执行结果未知。动作和 ADB 不自动重试。

默认验证 TLS，可通过 `Client(verify='CA_PATH', headers=...)` 配置 CA 和明确请求头；不读取未知凭据。每次发布和点击都校验会话与帧来源，点击 expires_ms 在发送锁内按剩余期限计算。

## 示例与验证

- [device_query.py](examples/device_query.py)：读取型号。
- [template_assets.py](examples/template_assets.py)：列出模板或上传 PNG 创建模板。
- [match_target.py](examples/match_target.py)：单次匹配；显式 `--tap` 才等待并点击。
- [watch_templates.py](examples/watch_templates.py)：持续发布，Ctrl+C 退出清框。
- [offline_match.py](examples/offline_match.py)：两个本地图像离线匹配。

```bash
python packages/python-sdk/examples/match_target.py DEVICE_SERIAL TEMPLATE_ID --base-url http://HOST:PORT
python -m pip install './packages/python-sdk[vision,test]'
python -m unittest discover -s packages/python-sdk/tests -v
```

本版在 macOS 验证了真实 H.264 受控服务、任务执行、包安装与小米手机只读取帧/匹配。真机点击等待用户指定目标；浏览器绿框和 Windows/Linux 原生链路仍待验收。macOS 的当前 PyAV/OpenCV wheel 同时加载时出现 libavdevice Objective-C 类重复警告；测试未出现故障，跨平台稳定性仍须原生验证。本次新增 Vision 能力须安装本仓库构建的 SDK；PyPI 上已发布的同版本基础包不包含这些新增能力。

## 版本与发布

SDK 版本与仓库根目录 `VERSION` 一致，Python 包版本去掉 `v` 前缀。修改 `VERSION` 后运行 `node scripts/sync-version.mjs`，同时更新主程序与 SDK 版本。

GitHub Actions 的 `Publish Python SDK` 工作流在主分支的版本、SDK 或发布配置变更时运行，也支持 `v*` 标签和手动触发；构建检查及安装测试通过后发布未发布的版本，已存在的完整版本会跳过。仓库需在 Settings → Secrets and variables → Actions 配置 `PYPI_API_TOKEN`，值为有此项目发布权限的 PyPI API Token。只修改 SDK 而不更新版本号不会覆盖 PyPI 上已有的版本。

## 外部任务上报

`TaskReporter(base_url, name=..., device_serial=..., source="codex")` 支持状态、心跳、实时日志和结果上报，无需专用上报令牌。完整示例与失败处理见 [任务运行集成](../../docs/task-runs.md) 和 `examples/report_task_run.py`。本版发布前请从本仓库安装 SDK。
