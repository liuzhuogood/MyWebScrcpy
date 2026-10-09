# v0.2.0-feature 会话记录

## 记录原则

- 用户原话完整记录。
- 实际工具回复精简记录，只保留结论、承诺、关键问题和关键决策。
- 每次用户对话都要追加记录。

## 会话

### 1. 用户

> 先 git init
> 下一版开工：tasks 4.2，v0.2.0 投屏核心用真机联调

### 1. OpenCode

- 执行 `git init`（仅初始化，不提交；提交推送等用户回 `2`）。
- 研读老播放器：`decoder.js`（WebCodecs + `[1B kind][8B pts][payload]` 帧格式 + Annex-B/AVCC 转换）、`control.js`（32/21/14/变长/2/1 字节控制消息 + keycode 映射）、`player.html`（`/ws?serial=` 连接、meta/error/disconnected 处理、退避重连、看门狗、灭屏检测、鼠标/触摸/键盘、工具栏）。
- 开工 v0.2.0，真机联调。

### 联调中发现并修复

1. WS 代理缺失：`vite.config` 只有 `/api`，`/ws` 未代理导致一直连接中 → 补 `ws:true` 代理并重启 dev。
2. Template ref 误用：`ref="p.canvasRef"` 注册的是字面字符串，canvasRef 恒为 null，connect 根本没跑 → 改 `:ref="p.canvasRef"`（Vue 3.5 动态 ref）。
3. 对照ด้วยการ：同面板打开老 `player.html` 正常出画面（448x1024/15fps），确认后端与面板 WS 能力无问题，才定位到自身 bug。
4. 旋转在 MIUI 桌面被锁定竖屏（预期内，状态反馈正常）；误进 mock serial 时错误路径符合设计（3 次静默重试后红屏 + 真实错误）。

### 目标模式继续推进（3.6 补充验证）

- 画中画：开 → 按钮变“退出画中画”、流继续；关 → 恢复常态。完整通过。
- 全屏：面板内点击无状态变化；同条件对比老前端行为一致，判定为面板环境限制（非实现 bug），留真机浏览器复验。
