# v0.2.0-feature 版本说明

> 2026-10-09 状态核对：**已实现；待验收**。投屏核心实施与已记录的真机冒烟完成。键盘、滚轮、断线重连、非锁定应用真旋转和真实浏览器全屏仍待专项验收。

1. 投屏核心可用（真机 `10.0.0.30` 联调通过）：`/player?serial=` 出画面（448x1024 + fps），可点可控。
2. 新增 `src/scrcpy/`：`protocol.ts`（控制消息打包/keycode）、`decoder.ts`（WebCodecs H.264 + Annex-B/AVCC）、`audio.ts`（默认静音可开启）、`pip.ts`（原生优先兜底文档 PiP）、`usePlayer.ts`（WS/重连/看门狗/灭屏/输入/工具栏动作）。
3. `PlayerView.vue`：shadcn 工具栏（返回/Home/旋转/音频/画中画/全屏/显示尺寸Popover+Slider/更多DropdownMenu/重启AlertDialog）+ 加载/失败/灭屏 overlays；路由标题按页切换。
4. 联调修复两个 bug：`/ws` 代理缺失；template ref 字面字符串导致 connect 未执行。
5. 验证：Home 键 E2E（page-info 落到 `com.miui.home`）；旋转反馈正常；错误路径（3 次静默重试后红屏）符合设计；`vue-tsc + build` 通过。
6. 补充验证：画中画开/关通过；全屏面板内无法验证（与老前端对比一致，系环境限制）。
7. 待日常复验：键盘打字、滚轮、全屏（真浏览器）、断线重连、真旋转解码重配。
