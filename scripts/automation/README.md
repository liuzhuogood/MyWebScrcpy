# 快手极速版：进入去赚钱

`kuaishou_earn.py` 使用 `mywebscrcpy.Client`，所有设备命令通过 `device.adb()` 执行，不需要本机另行配置 ADB。目标包名和入口来自实机查询：`com.kuaishou.nebula/com.yxcorp.gifshow.HomeActivity`。

```bash
packages/python-sdk/.venv/bin/python scripts/automation/kuaishou_earn.py \
  10.0.0.30:5555 --base-url http://127.0.0.1:8080
```

上面的设备、地址是本次测试值；脚本本身要求传入 serial 和服务地址。其他 Python 环境先安装当前仓库的 SDK：`python -m pip install ./packages/python-sdk`。Token 如有启用，需显式指定 `--token-header`，不假设认证头或前缀。服务由使用者启动，脚本不管理服务。

流程：

1. 唤醒手机，确认无密码锁屏，调用 `wm dismiss-keyguard` 并检查锁屏确实解除。有安全锁屏时停止，等待手工解锁。
2. 如果焦点正是快手 HomeActivity，先回桌面，避免活动动画持续阻塞 UIAutomator；其他焦点不执行这一步。随后采集新 XML，检查精确的 `text="仅限充电"`。存在时，只在同一窗口、相同实际 package 中唯一定位 `text="取消"` 的可点击节点或祖先，点击后重新采集验证。
3. 用 `am start -W -S` 冷启动快手极速版，统一启动状态。这会停止当前快手进程，不清除应用数据。
4. 等待唯一的、已启用且可点击的 `content-desc="去赚钱"`，按本次 XML 的 bounds 中心点击，检查坐标落在设备尺寸内。
5. 用 `dumpsys activity top` 确认实机核对过的任务中心 Fragment 同时为已添加、未隐藏且用户可见，前后焦点均须为已确认的 HomeActivity。任务中心的活动动画不影响这项检查。

每次运行使用独立的设备临时文件，并先删除该文件再 dump。除 ADB 退出码外，还检查 UIAutomator 的生成确认；遇到 `could not get idle state` 只重试只读采集，最多两次，禁止读取旧文件。找不到、多匹配、页面变化、动作失败或状态不符均非零退出，不盲目重发点击。

默认在系统临时目录保存独立的 XML、采集上下文、dump 输出、启动输出及结果；也可以用 `--artifacts /尚不存在的目录` 指定位置。设备临时文件结束后清理。本地证据可能包含页面和账号资料，不应提交仓库。同一设备请串行运行。

该脚本参考 `.pi/skills/markdown-to-python` 的 Client、设备参数、资源释放和失败退出约定。为实现本次动态 XML 定位，额外使用 Python 标准库 XML、正则和文件操作，并按职责拆分函数，因此作为独立 Python 脚本运行；它不符合当前 Markdown 生成器仅允许四个标准库、唯一 main 的校验白名单。本次没有修改生成器或 SDK。

验证范围（2026-10-15）：

- 设备：`10.0.0.30:5555`，小米 M2004J7AC，1080×2400，竖屏。
- 服务：`http://127.0.0.1:8080`。先核对当前 OpenAPI 和 API 目录，再按 page-info → XML → page-info 分析，XPath 唯一命中去赚钱。
- 无充电弹窗流程已实机验证；包含从赚钱页再次运行、熄屏且锁屏状态为 true 后运行，以及从桌面运行。结果和原始证据保存在 `/tmp/ks-test-20261015/`。
- 点击后的截图显示任务中心及其活动弹窗，未继续点击活动、广告、提现或领奖按钮。XML 不暴露活动弹窗文字，脚本验证进入任务中心，不验证余额、任务内容或网络加载是否完整。
- “仅限充电 → 取消”分支尚未实机验证，本次按用户要求先测无弹窗流程；合成节点测试仅验证定位范围和拒绝误点击逻辑。
- 第一轮发现赚钱页动画导致 dump 退出码为 0 但不生成 XML，已据此修正；以修正后的实机结果为准。

防误点击与失败处理测试：

```bash
packages/python-sdk/.venv/bin/python -m unittest discover \
  -s scripts/automation -p 'test_kuaishou_earn.py' -v
```
