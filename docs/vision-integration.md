# Python Vision 对接说明

本文档说明 Python Vision 如何连接 MyWebScrcpy，接收 Android 投屏帧、回传识别框，并按需请求设备动作。

## 1. 启动

```bash
cd /Users/liuzhuo/code/MyWebScrcpy
go run .
adb devices -l
curl http://127.0.0.1:8080/api/devices
python3 -m pip install -r scripts/vision/requirements.txt
python3 scripts/vision/demo_vision.py --serial 10.0.0.30:5555 --max-fps 5
```

演示脚本会在 Python 窗口显示解码画面，绘制 `demo-box`，把同一个框回传到浏览器播放器，并每 5 秒发送一次 heartbeat；`--max-fps 0` 可关闭 Vision 端降采样。

如需取得本机性能基线，可运行只读基准：

```bash
/Users/liuzhuo/.venv/bin/python scripts/vision/benchmark_stream.py --serial 10.0.0.30:5555 --seconds 10 --max-fps 0
```

输出包含接收包率、解码帧率、H.264 负载带宽、frame 时间戳到 Python 收包的传输延迟、PyAV 解码耗时、Python 侧 JPEG 转码耗时和进程峰值内存增量。该结果用于评估当前 H.264 旁路和本地 JPEG 成本；不能直接等同于 Go 端 JPEG 推流性能。

## 2. Vision WebSocket

```text
WS /api/vision/stream?serial=<URL 编码后的设备序列号>
```

示例：

```text
ws://127.0.0.1:8080/api/vision/stream?serial=10.0.0.30%3A5555
```

同一 `serial` 的浏览器投屏和 Python Vision 共享一个 Go Session，不会各自重复启动 scrcpy-server。

连接后，Go 先发送：

```json
{"type":"hello","protocol_version":"1","device_id":"10.0.0.30:5555","session_id":"a1b2c3d4","format":"scrcpy-frame","codec":"h264","width":448,"height":1024}
```

之后重复发送一条 JSON `frame` 元数据和紧随其后的 binary 视频帧：

```json
{"type":"frame","device_id":"10.0.0.30:5555","session_id":"a1b2c3d4","frame_id":123,"pts":19155162967,"timestamp":1788751244800,"width":448,"height":1024,"kind":1,"codec":"h264"}
```

Python 可在收到 `hello` 后回传订阅参数，要求 Go 端对 delta 帧降采样：

```json
{"type":"hello","max_fps":5}
```

Go 返回 `hello.ack`。`max_fps=0` 表示不额外采样，最大接受值为 30；config、key 和尺寸变化帧始终保留，只有 delta 帧会被跳过。

客户端可以省略 `protocol_version` 以兼容旧脚本；如果显式发送版本号，当前必须为 `1`。
未知版本不会被静默降级，服务端返回 `{"type":"error","code":"unsupported_protocol"}`，
并在调试事件中记录 `protocol.rejected`。

## 3. Binary 帧格式

```text
[1 字节 kind][8 字节 PTS，大端序][payload]
```

除 `kind=3` 的尺寸变化包外，binary 消息长度严格为 `9 + len(payload)`，末尾没有填充字节；这点对 PyAV/FFmpeg 的包边界处理很重要。

| kind | 含义 | payload |
| ---: | --- | --- |
| `0` | codec config | H.264 SPS/PPS，通常为 Annex-B，也可能是 AVCC |
| `1` | key frame | H.264 IDR 数据 |
| `2` | delta frame | H.264 P 帧数据 |
| `3` | session/尺寸变化 | 8 字节：`width`、`height`，均为大端序 `uint32` |

当前常见 `codec` 为 `h264`。推荐用 PyAV 的 `CodecContext.parse()` 再调用 `decode()`，不要假设一个 WebSocket binary 消息就是一个完整编码包。完整实现见 `scripts/vision/demo_vision.py`。

## 4. 回传识别结果

Python 在同一条 Vision WebSocket 发送：

```json
{"type":"detection.result","device_id":"10.0.0.30:5555","session_id":"a1b2c3d4","frame_id":123,"timestamp":1788751244800,"expires_ms":1000,"objects":[{"label":"确认按钮","confidence":0.96,"x":0.61,"y":0.72,"w":0.18,"h":0.07}]}
```

坐标均为 `0~1` 归一化值：

| 字段 | 含义 |
| --- | --- |
| `x` | 框左边距 / 画面宽度 |
| `y` | 框上边距 / 画面高度 |
| `w` | 框宽 / 画面宽度 |
| `h` | 框高 / 画面高度 |

Go 会按 `device_id + session_id` 将结果广播给浏览器 overlay；旧 session 的结果不会进入新视频会话。浏览器接收结果的通道是：

```text
WS /api/vision/results?serial=<serial>
```

Python 不需要连接这个结果广播通道。

检测框必须满足 `label` 非空、`confidence` 在 `0~1`，且 `x/y/w/h` 为归一化坐标并完整落在画面内；不符合时 Go 返回 `invalid_detection`，不会广播。
如果提供 `timestamp` 和 `expires_ms`，Go 会在广播前检查结果时效，超时返回 `expired_result`。

## 5. 请求设备动作

Python 可发送：

```json
{"type":"action.request","device_id":"10.0.0.30:5555","request_id":"vision-001","action":"tap","x":0.70,"y":0.75,"frame_id":123,"expires_ms":1000}
```

Go 返回：

```json
{"type":"action.result","request_id":"vision-001","accepted":true,"executed":true}
```

当前有效自动动作支持 `tap`、`swipe`、`key` 和 `text`；`swipe` 使用 `x/y` 到 `x2/y2` 的归一化起止坐标，`key` 需要 Android keycode，`text` 注入 UTF-8 文本。坐标越界返回 `invalid_coordinate`；队列满返回 `queue_full`；过期返回 `expired`。动作会与浏览器控制消息共用同一设备级队列。

示例：

```json
{"type":"action.request","request_id":"swipe-001","action":"swipe","x":0.8,"y":0.8,"x2":0.2,"y2":0.8,"expires_ms":1000}
{"type":"action.request","request_id":"back-001","action":"key","keycode":4,"expires_ms":1000}
{"type":"action.request","request_id":"text-001","action":"text","text":"你好","expires_ms":1000}
```

## 6. Python 核心循环

```python
import json
import websocket

serial = "10.0.0.30:5555"
url = "ws://127.0.0.1:8080/api/vision/stream?serial=" + serial.replace(":", "%3A")
ws = websocket.create_connection(url, timeout=15)
hello = json.loads(ws.recv())
print(hello)

latest = None
while True:
    message = ws.recv()
    if isinstance(message, str):
        info = json.loads(message)
        if info.get("type") == "frame":
            latest = info
        continue

    kind = message[0]
    payload = message[9:]
    if kind not in (0, 1, 2):
        continue
    # 用 PyAV 解码 payload，再执行 YOLO/OCR。
    ws.send(json.dumps({
        "type": "detection.result",
        "device_id": serial,
        "session_id": hello.get("session_id"),
        "frame_id": latest.get("frame_id", 0) if latest else 0,
        "objects": [{"label":"demo","confidence":0.99,"x":0.25,"y":0.25,"w":0.5,"h":0.5}],
    }))
```

## 7. 断线、安全与限制

- 最后一个订阅者断开后，Go Session 会保留约 10 秒；新订阅者会先收到最近的 config/key 帧。
- Python 首次连接可能包含 ADB push 和 scrcpy-server 握手，连接建立超时建议至少 30 秒；断线后等待 1~2 秒再重连，不要高频循环。
- Python 可发送 `{"type":"heartbeat"}`，Go 会返回带时间戳的 heartbeat 响应。
- 设备旋转时收到 `kind=3`，应清理旧解码状态并等待新的 config/key 帧；Go 也会清理共享 Session 中缓存的旧配置，避免新订阅者拿到过期 SPS/PPS。
- 当前 Vision/Action 没有独立鉴权，Go 默认监听 `0.0.0.0:8080`，只适合本机或可信内网，不要暴露到公网。
- Python 可直接 ADB 做只读诊断、安装和环境准备；控制动作优先使用 `action.request`，避免绕过 Go 队列。
- 可用 `GET /api/vision/stats?serial=...` 查看当前共享 Session 的订阅数、帧数、丢帧数、动作队列长度和最近帧时间。
- 可用 `GET /api/vision/debug?serial=...&limit=100` 查看最近的 Session、检测拒绝和动作结果事件；这是只读的有界缓冲，不保存视频内容。
- Go 端 JPEG 采样、鉴权和更完整的多设备调试 UI 仍在后续范围；当前结构化动作已支持 `tap`、`swipe`、`key`、`text`，播放器和 dashboard overlay 已支持。
