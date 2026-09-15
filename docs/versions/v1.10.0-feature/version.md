# v1.10.0-feature 版本说明

1. 播放器"更多"菜单新增"终端"入口，点击后在右侧展开可拖拽宽度的 ADB Shell 侧边面板。
2. 后端新增 `GET /ws/adb-shell?serial=xxx` WebSocket 端点，通过 `creack/pty`（MIT）为 `adb -s <serial> shell` 分配真实 PTY；支持 Ctrl+C、Tab 补全、方向键历史和彩色输出。
3. 前端引入 xterm.js v5（MIT，CDN）渲染终端，`FitAddon` 随面板尺寸自动 reflow。
4. 每个 serial 同时只允许一个 shell session，新连接自动关闭旧连接；WebSocket 断开时 SIGKILL 进程组、无活动 30 分钟自动超时。
5. 断连后指数退避最多重连 3 次，失败后提示手动重连按钮。
6. Inspector 与 Shell 面板互斥，互相打开时自动关闭另一个。
