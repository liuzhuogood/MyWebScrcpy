# v1.10.0-feature 任务清单

## 1. 规格与设计

- ⬜ 1.1 完成 session/design/tasks/version 初稿。
- ⬜ 1.2 确认开源终端组件（xterm.js）引入方式（CDN vs 本地）及安全性。
- ⬜ 1.3 确认 WebSocket PTY 后端方案（golang.org/x/crypto/ssh 或 os/exec PTY）。
- ⬜ 1.4 用户确认设计方案。

## 2. 后端实现

- ⬜ 2.1 新建 `internal/adbshell/` 包，实现 PTY 代理（`os/exec` + `creack/pty`）。
- ⬜ 2.2 新增 WebSocket 路由 `GET /ws/adb-shell?serial=xxx`，完成 I/O 双向桥接。
- ⬜ 2.3 在 `main.go` 注册新路由，复用 `devicegate` 串行限流。
- ⬜ 2.4 安全边界：只允许 `adb shell`，禁止 `-s`/`-H`/`-P` 覆盖，超时/断开自动 kill PTY。

## 3. 前端实现

- ⬜ 3.1 在 `player.html` 工具栏"更多"菜单中新增"终端"按钮（Shell 图标）。
- ⬜ 3.2 新建 `web/js/adb-shell.js`，封装 xterm.js 初始化、WebSocket 连接、resize 事件。
- ⬜ 3.3 新建 `web/css/adb-shell.css`，复用 inspector panel 右侧嵌入布局。
- ⬜ 3.4 在 `player.html` `player-main` 区块中插入 resizer + shell panel 结构。
- ⬜ 3.5 面板关闭时断开 WebSocket，断线时显示重连提示；重连后恢复历史缓冲区。

## 4. 验证

- ⬜ 4.1 手动验证：打开面板 → 输入 `ls /sdcard` → 验证输出。
- ⬜ 4.2 验证 Ctrl+C 中断进程、Tab 补全、方向键历史正常工作。
- ⬜ 4.3 验证关闭面板后 PTY 进程终止，不产生僵尸进程。
- ⬜ 4.4 验证面板拖拽 resize 正常，xterm.js `fit` addon 适配窗口尺寸。
- ⬜ 4.5 验证 inspector 面板与 shell 面板不同时开启（或可并存）行为明确。

## 5. 发布

- ⬜ 5.1 更新版本摘要。
- ⬜ 5.2 git commit & push。
