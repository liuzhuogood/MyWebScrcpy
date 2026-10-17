# Android 常用按键编码速查 (Keycodes)

| 按键常量 | Keycode 值 | 常见用途 | 对应脚本 |
|---|---|---|---|
| `KEYCODE_BACK` | 4 | 标准系统返回，回退上一级 Activity、页面返回 | `scripts/android_back.py` |
| `KEYCODE_HOME` | 3 | 回到系统桌面 Launcher | 可通过 action/adb 注入 |
| `KEYCODE_APP_SWITCH` | 187 | 打开多任务最近应用列表 | 可通过 action/adb 注入 |
| `KEYCODE_ESCAPE` | 111 | 取消当前弹窗、对话框或软键盘，尽量不破坏页面层级 | `scripts/try_return.py` |
| `KEYCODE_ENTER` | 66 | 软键盘回车或确定 | 可通过 action/adb 注入 |

## 弹窗消除实战经验

- 部分短视频或购物 APP 在主界面弹出活动弹窗时，直接按 `KEYCODE_BACK` (4) 可能会触发“再按一次退出应用”的逻辑。
- 此时发送 `KEYCODE_ESCAPE` (111) 通常能够关闭顶层 Dialog 而不会触发退出应用拦截器。
