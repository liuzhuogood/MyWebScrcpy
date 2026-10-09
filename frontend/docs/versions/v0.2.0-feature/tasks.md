# v0.2.0-feature 任务清单

> 2026-10-09 状态核对：**已实现；待验收**。详见 [版本摘要](version.md) 与 [统一状态总览](../../../../docs/versions/README.md)；历史验收不等于本次重新验收。

## 1. 规格与设计

- ☑ 1.1 完成 session/design/tasks/version 初稿。
- ☑ 1.2 研读 decoder/control/player，范围收敛（不含检查器/模板/录制等）。

## 2. 实现

- ☑ 2.1 `src/scrcpy/protocol.ts`：控制消息打包 + keycode 映射（平移 control.js）。
- ☑ 2.2 `src/scrcpy/decoder.ts`：WebCodecs 解码器 TS 版（平移 decoder.js；const enum 改普通对象以合 erasableSyntaxOnly）。
- ☑ 2.3 `src/scrcpy/audio.ts` + `pip.ts`：设备音频开关 + 画中画（音量调节延后）。
- ☑ 2.4 `src/scrcpy/usePlayer.ts`：WS 生命周期 + 重连 + 看门狗 + 灭屏 + 输入事件。
- ☑ 2.5 `PlayerView.vue`：工具栏（shadcn Button/DropdownMenu/Popover+Slider/AlertDialog）+ 状态/fps + canvas 自适应。
- ☑ 2.6 路由 `/player` 替换 Stub。

## 3. 验证（真机 `10.0.0.30`）

- ☑ 3.1 出画面 + 分辨率 + fps（448x1024，fps 正常上报；音频按钮就绪）。
- ☑ 3.2 点击画布无报错；Home 键 E2E 验证（page-info 显示 `com.miui.home`）。
- ☑ 3.3 工具栏：Home / 旋转（launcher 锁定竖屏，状态反馈正常、流不断）/ 显示尺寸弹窗 / 更多菜单渲染；任务切换·锁屏·重启走同 keyPress 通道（Home 已代验），重启只看弹窗不真点。
- ☑ 3.4 错误路径：误进 mock serial（emulator-5554）触发 3 次静默重试后亮红屏，展示后端真实错误 + 重试按钮。
- ☑ 3.5 `vue-tsc + build` 通过；路由标题切换正常。
- ☑ 3.6 补充验证：画中画开/关完整通过（按钮状态切换、流不中断）；全屏在面板内无状态变化，与老前端同面板对比一致（面板环境限制，真机浏览器复验）。
- ⬜ 3.7 待用户日常复验：键盘打字、滚轮、断线重连、真旋转解码重配（需非锁定方向应用）。

## 4. 发布

- ☑ 4.1 版本摘要已更新；2026-10-09 已补充当前状态与验收边界。
