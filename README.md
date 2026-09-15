# MyWebScrcpy

一个基于 Go 和 WebCodecs 的开源 Android 设备投屏与控制工具。通过浏览器即可实时查看设备画面、执行触控和键盘操作，并管理设备的共享存储。访问端无需安装专用应用，Android 设备通过 ADB 连接到运行 MyWebScrcpy 的主机后即可使用。

[English](README_EN.md)

## 截图

**设备列表**

![设备列表](screenshots/device-list.png)

**投屏操控**

![投屏操控](screenshots/player.png)

**播放器检查器**

![播放器检查器](screenshots/ui-inspector.jpg)

**大屏监控**

![大屏监控](screenshots/dashboard.png)

**文件管理**

![文件管理](screenshots/file-manager.jpg)

## 特性

- 纯浏览器端，无需安装任何客户端
- 基于 WebCodecs 硬件解码，低延迟
- 支持 H.264 / H.265 / AV1 编码
- 鼠标操控：点击、拖拽、滚轮、右键返回
- 键盘输入：文本注入、快捷键
- 触摸支持（移动端浏览器）
- 一键旋转屏幕
- 全屏模式（支持 iOS 伪全屏）
- 浏览器画中画：将当前投屏画面置于独立浮窗，方便边操作其他页面边查看设备
- 播放器检查器：取色坐标、像素放大镜、XML 树、XPath 与元素定位联动
- 页面信息 (UI Package)：实时查看当前前台应用包名、Activity 类名、焦点窗口、屏幕分辨率、DPI 及旋转方向
- 模板匹配与目标定位：纯 Go 算法实现，支持选区截图存为模板、多参考图、多尺度匹配，具备本地文件全自动热重载与实时画面重测，无缝联动投屏绿框高亮
- 动作执行多模式：支持极低延迟 SDK 拟人化注入（贝塞尔曲线/落点微抖动）与 ADB Shell input 系统级原生穿透执行
- Web ADB Shell 交互式终端：基于 xterm.js 与真实 PTY 分配，支持 Tab 补全、方向键历史与终端色彩
- 右侧自适应抽屉面板：统一管理检查器、文件管理、模板匹配和 ADB 终端，自适应视口宽度与布局
- 设备音频播放：默认静音，用户可主动开启网页播放
- 受控 MP4 录屏：选择自动停止时长、可选录入设备声音、提前停止与完成下载
- 屏幕熄灭检测
- 自动重连
- 大屏监控模式（多设备同屏展示，支持小/中/大三档尺寸）
- 文件管理：浏览目录、搜索筛选、上传下载、移动、重命名、批量删除和撤销
- 单二进制文件，内嵌 scrcpy-server 和前端资源

## API 文档

启动服务后访问 [`/api-docs.html`](/api-docs.html) 查看全部 HTTP/WebSocket 接口；机器可读清单位于 [`/api/openapi.json`](/api/openapi.json)。文档首段提供可复制的 `my-scrcpy-use` 技能提示词。

## 原理

```
浏览器 ──WebSocket──▶ Go Server ──ADB Forward──▶ scrcpy-server (设备端)
  │                       │
  │  H.264 视频帧         │  控制消息透传
  │  ◀──────────────────  │  ──────────────▶
  │                       │
WebCodecs 解码          app_process 启动
Canvas 渲染             视频编码 + 控制注入
```

Go 后端负责：
1. 将内嵌的 scrcpy-server jar 通过 ADB push 到设备
2. 建立 ADB forward 隧道
3. 启动设备端 scrcpy server 进程
4. 通过 WebSocket 在浏览器和设备之间双向转发视频帧和控制消息

### Python Vision 对接

项目提供独立的 Vision WebSocket，Python 可以订阅同一套 scrcpy 视频会话，
返回识别框和自动化动作；浏览器会把识别结果叠加到投屏画面上。完整的二进制帧格式、
示例脚本、结果与动作协议见 [Python Vision 对接说明](docs/vision-integration.md)。

```bash
python3 -m pip install -r scripts/vision/requirements.txt
python3 scripts/vision/demo_vision.py --url ws://127.0.0.1:8080 --serial 10.0.0.30:5555
```

Vision 接口当前支持协议版本 `1`。显式发送未知版本会收到
`unsupported_protocol`；省略版本号仍兼容旧客户端。

## 环境要求

- Go 1.21+
- ADB（Android Debug Bridge）
- Chrome 94+（需要 WebCodecs 支持）
- Android 设备已开启 USB 调试或已通过网络 ADB 连接

## 快速开始

```bash
# 克隆项目
git clone https://github.com/liuzhuogood/MyWebScrcpy.git
cd MyWebScrcpy

# 构建
go build -o mywebscrcpy .

# 运行
./mywebscrcpy
```

浏览器打开 `http://localhost:8080`，点击设备即可投屏。

文件管理从投屏页面“更多”菜单打开，操作对象是当前选定手机的 `/storage/emulated/0` 共享存储。多手机场景下，每个投屏页面都会绑定自己的设备。

### 画中画

投屏画面准备完成后，点击播放器工具栏的“画中画”按钮即可打开浏览器原生浮窗。浮窗持续显示当前投屏画面，关闭浮窗或再次点击按钮会退出画中画。该能力依赖浏览器的画中画与画布捕获支持；不支持时按钮会保持禁用。

### 录屏

播放器工具栏提供“录制”入口。开始前选择 1 分钟至 8 小时的自动停止时长（默认 30 分钟）；可以提前停止，完成后直接下载 MP4。录屏复用当前设备的共享 H.264 会话，不中断投屏、控制或 Vision 订阅；每台设备同一时刻只能有一条录制。

| 方法 | 路径 | 说明 |
|------|------|------|
| `POST` | `/api/recordings?serial=...` | 请求体 `{"max_duration_ms":300000,"record_audio":false}`，创建录制；`record_audio` 缺省或 `false` 保持无声录像。 |
| `GET` | `/api/recordings?serial=...` | 获取该设备的录制记录。 |
| `GET` | `/api/recordings/{recording_id}?serial=...` | 查询状态。 |
| `POST` | `/api/recordings/{recording_id}/stop?serial=...` | 提前停止，幂等。 |
| `GET` | `/api/recordings/{recording_id}/download?serial=...` | 下载已完成的 MP4。 |
| `GET` | `/api/recordings/download?serial=...&recording_id=...` | 下载指定记录；省略 `recording_id` 下载最后一次完成的录像。 |
| `DELETE` | `/api/recordings/{recording_id}?serial=...` | 删除已结束的录制记录及文件。 |

录制仅支持当前默认的 H.264 共享流；未完成文件不会开放下载。完成文件会在受控目录中保留到期，服务重启会清理未完成的临时文件。

### 播放器检查器

在投屏页面打开“更多”中的“检查”。检查器会和投屏并排显示：投屏保持靠左的原始显示宽度，检查面板自动占用剩余空间；拖动中间分隔线可调整面板宽度。窄屏设备会自动改为上下布局。

#### 取色坐标

- 鼠标移动时显示画布坐标、HEX、RGB、十字线和 13 × 13 像素放大镜。
- 首次点击投屏画面会固定十字线与读数，便于复制；再次点击会解除固定并恢复跟随。读数同时展示像素坐标和 0～1 的归一化坐标。
- 检查器打开期间的取色操作不会发送到 Android 设备。

#### XML 树与 XPath

- 切换到“XML 树”会读取当前设备的只读 UI 快照；也可以用“刷新 XML”重新获取。
- 树默认展开，标签、属性名、属性值和布尔值以轻量语法高亮显示；树区域有独立滚动条。
- 点击树节点会在投屏上标出对应元素；点击标出的绿框也会反选树节点。XML 原始坐标会按视频 Canvas 尺寸换算，因此缩放投屏时绿框仍与元素对齐。
- 选中树节点后，每个属性左侧都有复选框；勾选后会将对应属性组合为 XPath 条件、同步到输入框并自动测试，匹配元素随即显示绿框。默认优先选择非空 `resource-id`，没有时选择非空 `text`；其他属性（包括 `class`、`package`、`bounds`）可按需要加入，且默认不选。
- 输入 XPath 后点击“测试 XPath”，全部命中元素会显示绿框；“显示全部”可显示当前快照中所有带 bounds 的元素，再次点击可关闭。
- “清除”会清空 XPath 命中和所有元素标记。

XML 快照来自 Android 的 UI Automator，属于获取瞬间的静态状态，并不与视频逐帧同步。Canvas/WebView 等自绘内容可能没有可用节点；“显示全部”在元素密集的页面会产生较多重叠绿框。

| 方法 | 路径 | 说明 |
|------|------|------|
| `GET` | `/api/ui/xml?serial=...` | 获取指定在线设备的只读 XML 快照、时间戳和显示坐标基准 `display_size`。 |

### 页面信息 (UI Package)

在投屏播放器“检查”面板中提供“页面信息”选项卡，无需启动重量级 UI Automator 转储即可秒级读取当前设备的前台状态：
- 实时展示当前顶层前台 APP 应用包名、Activity 类名、焦点窗口标识。
- 展示屏幕物理分辨率、DPI、当前屏幕旋转方向。
- 提供核心字段的一键复制与实时刷新。

| 方法 | 路径 | 说明 |
|------|------|------|
| `GET` | `/api/ui/page-info?serial=...` | 获取前台应用包名、Activity、焦点窗口、分辨率及旋转等元信息。 |

### 模板匹配与目标定位

集成基于纯 Go 实现的 OpenCV 级模板匹配能力（`TM_CCOEFF_NORMED` / `TM_SQDIFF_NORMED`）：
- **全自动热重载**：内置 `fsnotify` 目录监听与动态防抖机制，无论在 Web 界面“存为模板”还是直接在磁盘目录添加/修改图片与 `meta.json`，服务均自动增量重载，**无需重启**。
- **画面即时重测**：模板变动瞬间自动利用最新解码帧执行重测，静止画面下高亮绿框也能毫秒级立即显现。
- **多参考图与多尺度**：支持为同一模板添加多个光照/样式的变体参考图，支持配置多尺度（Scales）搜索。
- **一键定位点击**：在模板卡片上可直接触发定位点击，支持 `sdk`（拟人化手势）与 `adb`（原生注入）双模式。

| 方法 | 路径 | 说明 |
|------|------|------|
| `GET` | `/api/templates?serial=...` | 查询设备特定与全局可用模板列表。 |
| `POST` | `/api/templates` | 创建/上传新模板（支持 multipart 表单或 Base64）。 |
| `GET` | `/api/templates/{id}` | 获取模板详情。 |
| `PUT` | `/api/templates/{id}` | 更新模板参数（阈值、尺度、启用状态、作用域等）。 |
| `DELETE` | `/api/templates/{id}` | 删除模板及关联磁盘文件。 |
| `POST` | `/api/templates/{id}/images` | 为指定模板追加多参考图变体。 |
| `POST` | `/api/templates/click` | 在屏幕上定位指定模板并执行点击动作。 |
| `GET` | `/api/templates/matches?serial=...` | 获取当前实时匹配命中的目标坐标与置信度。 |

### Web ADB Shell 交互终端

在投屏播放器“更多”菜单中打开“终端”：
- 基于 `creack/pty` 与 `adb -s <serial> shell` 分配真实终端 PTY。
- 前端集成 xterm.js，支持快捷键输入、Tab 命令自动补全、上下键历史命令及完整终端 ANSI 彩色显示。
- 面板支持宽度自由拖拽调整，终端会随尺寸自适应 Reflow；断连支持自动重连。

| 协议 | 路径 | 说明 |
|------|------|------|
| `WebSocket` | `/ws/adb-shell?serial=...` | 建立与 Android 设备的交互式 ADB Shell PTY 会话。 |

### 动作执行多模式 (SDK / ADB)

所有的自动化点击与滑动动作均支持 `mode` 通道选择：
- `sdk`（默认）：通过内嵌的 scrcpy 控制 Socket 注入，具有极高响应速度，且支持 `humanize=true` 的拟人化贝塞尔曲线与落点抖动。
- `adb`：通过系统级 `adb shell input tap` 与 `input swipe` 执行，配合 `duration_ms` 控制滑动毫秒时长，能够有效穿透系统限制弹窗与权限拦截层。

### 受控 ADB 命令与组合键

`POST /api/adb/commands?serial=...` 仅以参数数组运行 `adb -s <serial> <args...>`，不执行宿主机 shell，也拒绝 `-s`、`--serial`、`-H`、`-P`、`-L` 等改变目标或 host 的参数。默认超时 60 秒；可用 `async: true` 查询后续状态，或以 `parallel: true` 在资源上限内显式绕过同设备 FIFO（调用方需自行承担设备状态竞态）。

`POST /api/sendkey?serial=...` 接收 `{"keycode":29,"modifiers":["ctrl","shift"]}`，支持 `shift`、`alt`、`ctrl`、`meta` 四种修饰键。

### Docker 部署

```bash
# 拉取镜像
docker pull liuzhuogood/mywebscrcpy:latest

# 运行（需要挂载 ADB 设备）
docker run -d \
  --name mywebscrcpy \
  --privileged \
  -p 8080:8080 \
  -v /dev/bus/usb:/dev/bus/usb \
  liuzhuogood/mywebscrcpy:latest

# 或者使用 host 网络（便于发现网络 ADB 设备）
docker run -d \
  --name mywebscrcpy \
  --privileged \
  --network host \
  -v /dev/bus/usb:/dev/bus/usb \
  liuzhuogood/mywebscrcpy:latest
```

浏览器打开 `https://IP:8080`（默认启用 HTTPS），点击设备即可投屏。

### 命令行参数

| 参数 | 说明 |
|------|------|
| `-https` | 启用 HTTPS（使用内置自签名证书） |

### 环境变量

| 变量 | 说明 | 默认值 |
|------|------|--------|
| `PORT` | HTTP 监听端口 | `8080` |
| `ANDROID_HOME` | ADB 路径查找 | 系统 PATH |
| `TLS_CERT` | 自定义 SSL 证书路径 | - |
| `TLS_KEY` | 自定义 SSL 私钥路径 | - |
| `FILES_MAX_UPLOAD_BYTES` | 单文件上传上限（字节） | `268435456` |
| `RECORDINGS_DIR` | MP4 录像目录 | 用户缓存目录下的 `mywebscrcpy/recordings` |
| `RECORDINGS_MAX_BYTES` | 录像目录总配额（字节） | `10737418240` |
| `RECORDINGS_RETENTION_HOURS` | 完成录像保留时长（小时） | `168` |
| `ADB_MAX_TIMEOUT_MS` | 单条 ADB 命令最大超时（毫秒） | `600000` |
| `ADB_MAX_QUEUE_WAIT_MS` | FIFO 命令最大排队时长（毫秒） | `600000` |
| `ADB_MAX_OUTPUT_BYTES` | 单条 ADB 命令最大输出（字节） | `1048576` |
| `ADB_MAX_PARALLEL_PER_SERIAL` | 同设备显式并行 ADB 命令上限 | `2` |

### HTTPS 配置

WebCodecs API 需要安全上下文（HTTPS 或 localhost）才能工作。如果通过 IP 地址访问，需要启用 HTTPS。

**方式 1：使用内置证书（最简单）**

```bash
./mywebscrcpy -https
```

访问 `https://IP:8080`，浏览器会提示证书不受信任，点击「高级」→「继续访问」即可。

**方式 2：使用自定义证书**

```bash
# 设置环境变量
export TLS_CERT=/path/to/cert.pem
export TLS_KEY=/path/to/key.pem
./mywebscrcpy
```

**方式 3：Nginx 反向代理（推荐生产环境）**

```nginx
server {
    listen 443 ssl;
    server_name your-domain.com;

    ssl_certificate /path/to/cert.pem;
    ssl_certificate_key /path/to/key.pem;

    location / {
        proxy_pass http://127.0.0.1:8080;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
    }

    location /ws {
        proxy_pass http://127.0.0.1:8080;
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";
        proxy_set_header Host $host;
        proxy_read_timeout 86400;
    }
}
```

## 操控方式

| 操作 | 说明 |
|------|------|
| 鼠标左键 | 点击/拖拽 |
| 鼠标右键 | 返回键 |
| 鼠标滚轮 | 滚动页面 |
| 键盘 | 文本输入 |
| 工具栏 | Home、返回、最近任务、电源、旋转、全屏 |

## 项目结构

```
MyWebScrcpy/
├── main.go                    # 入口，HTTP 服务
├── assets/
│   └── scrcpy-server          # scrcpy server jar（内嵌）
├── internal/
│   ├── device/manager.go      # ADB 设备管理
│   ├── files/                  # 文件管理 API、路径安全和回收站
│   ├── uixml/                  # Android XML 快照服务与校验
│   ├── scrcpy/
│   │   ├── server.go          # scrcpy server 生命周期
│   │   ├── connection.go      # TCP 连接 + 帧读取
│   │   ├── protocol.go        # scrcpy 4.0 协议常量
│   │   └── control.go         # 控制消息打包
│   └── ws/hub.go              # WebSocket 管理
└── web/
    ├── index.html             # 设备列表页
    ├── player.html            # 投屏播放器页
    ├── dashboard.html         # 大屏监控页
    ├── files.html              # 文件管理页
    ├── css/
    │   ├── style.css            # 播放器与通用样式
    │   └── ui-inspector.css     # 检查器样式
    └── js/
        ├── decoder.js         # WebCodecs H.264 解码器
        ├── control.js         # 浏览器端控制消息打包
        ├── dashboard.js        # 大屏监控逻辑
        ├── files.js            # 文件管理逻辑
        └── ui-inspector.js     # 取色、XML 树和 XPath 检查器
```

## 技术栈

- **后端**: Go + gorilla/websocket
- **前端**: 原生 JavaScript + WebCodecs API + Canvas
- **投屏协议**: scrcpy 4.0
- **视频编码**: H.264 (Baseline)

## License

MIT
