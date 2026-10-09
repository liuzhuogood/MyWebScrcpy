# v1.10.0-feature 任务清单

> 2026-10-09 状态核对：**已实现；专项交互待验收**。详见 [版本摘要](version.md) 与 [统一状态总览](../README.md)；原文件路径与已完成勾选保留为历史记录。

## 1. 规格与设计

- ✅ 1.1 完成 session/design/tasks/version 初稿。
- ✅ 1.2 确认开源终端组件（xterm.js）引入方式（CDN vs 本地）及安全性。
- ✅ 1.3 确认 WebSocket PTY 后端方案（golang.org/x/crypto/ssh 或 os/exec PTY）。
- ✅ 1.4 用户确认设计方案。

## 2. 后端实现

- ✅ 2.1 新建 `internal/adbshell/` 包，实现 PTY 代理（`os/exec` + `creack/pty`）。
- ✅ 2.2 新增 WebSocket 路由 `GET /ws/adb-shell?serial=xxx`，完成 I/O 双向桥接。
- ✅ 2.3 在 `main.go` 注册新路由并校验在线 serial；实际采用每 serial 单 Shell 会话，新连接关闭旧会话，未接入 `devicegate`，修正原计划描述。
- ✅ 2.4 安全边界：只允许 `adb shell`，禁止 `-s`/`-H`/`-P` 覆盖，超时/断开自动 kill PTY。

## 3. 前端实现

- ✅ 3.1 在 `player.html` 工具栏"更多"菜单中新增"终端"按钮（Shell 图标）。
- ✅ 3.2 新建 `web/js/adb-shell.js`，封装 xterm.js 初始化、WebSocket 连接、resize 事件。
- ✅ 3.3 新建 `web/css/adb-shell.css`，复用 inspector panel 右侧嵌入布局。
- ✅ 3.4 在 `player.html` `player-main` 区块中插入 resizer + shell panel 结构。
- ✅ 3.5 已实现关闭时断开 WebSocket、最多 3 次重试和手动重连提示；同一面板生命周期保留本地终端输出。关闭面板后重新打开不恢复历史，跨面板历史恢复延期。

## 4. 验证

- ⬜ 4.1 手动验证：打开面板 → 输入 `ls /sdcard` → 验证输出。
- ⬜ 4.2 验证 Ctrl+C 中断进程、Tab 补全、方向键历史正常工作。
- ⬜ 4.3 验证关闭面板后 PTY 进程终止，不产生僵尸进程。
- ⬜ 4.4 验证面板拖拽 resize 正常，xterm.js `fit` addon 适配窗口尺寸。
- ✅ 4.5 Inspector 与 Shell 互斥规则已明确并实现；当前 Vue 通过单一侧栏状态选择展示面板。

## 5. 发布

- ✅ 5.1 更新版本摘要。
- ✅ 5.2 已提交终端实现 `0aa29b8`、侧栏布局 `ece55a1` 并有推送/部署会话记录；2026-10-09 已确认远端 `v1.10.0` 标签。
