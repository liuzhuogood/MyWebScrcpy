---
name: kuaishou-short-video
description: 使用 MyWebScrcpy 操作快手极速版，监测红包弹窗、执行连续打卡白拿手机任务，并按连续签到入口观看指定时长的视频。
---

# 快手极速版短视频任务

先从当前仓库安装包含 TaskReporter 的 SDK：`python3 -m pip install ./packages/python-sdk`。

先加载项目内 `skills/my-scrcpy-use/SKILL.md`。服务地址与设备必须由当前会话确认。

按序读取：

1. [流程与边界](rules/00-overview.md)
2. [前置条件](rules/01-preconditions.md)
3. [页面与定位](rules/02-pages-and-locators.md)
4. [动作实现](rules/03-actions.md)、[设备与证据](rules/05-runtime.md)、[入口](rules/06-entrypoint.md)
5. [验证与报告](rules/04-verify-and-report.md)

`rules/` 是生成模块的手写源。修改规则中的代码后，从代码块重生成对应脚本；独立运行入口需同步更新其契约与参数边界。先离线验证再真机执行：

```bash
python3 skills/kuaishou-short-video/scripts/test_flow.py
```

实际执行通过项目环境注册器，路径须为绝对路径：

```bash
python3 "$project/skills/my-scrcpy-use/scripts/run-with-env.py" \
  --base-url "$base_url" --serial "$serial" -- \
  python3 "$project/skills/kuaishou-short-video/scripts/main.py"
```

用户要求单独刷新视频时，可使用长时入口；时长可设为1秒至4小时，先读取“去赚钱”页金币，确认基线后进入首页，期间按15～30秒随机上滑、每60秒核验一次视频页、每分钟最多保存一张截图，结束后返回收益页复核金币；金币读数失败只记录、不中断观看计时，最终数值可另行复核。若任务要求领取观看视频奖励，须在任务中心找到与“待领取/继续看视频得金币”同卡的唯一“领金币”按钮，点击一次，并通过金币余额增加或明确的领取成功状态核验；目标卡片或唯一按钮无法确认时停止，不点击其他任务奖励。该机“任务中心”为 RN 页且弹层轮播（翻倍任务/瓜分百亿/签到/打卡/朋友推荐），返回键常被弹层吞掉，故弹层按“点文字按钮（立即打卡/立即签到/去看内容/去领取）→ 按返回键 → ADB force-stop + monkey 冷启回首页”逐级清理，不可回到首页时用冷启兜底；`去赚钱`、弹层按钮与视频页判定均按画面文字框中心点击（该机 XML bounds 与实际触摸区偏移，RN 页 XML 为下层原生树）：

```bash
python3 "$project/skills/my-scrcpy-use/scripts/run-with-env.py" \
  --base-url "$base_url" --serial "$serial" -- \
  python3 "$project/skills/kuaishou-short-video/scripts/refresh_hour.py" --seconds 3600
```

领取观看视频奖励时，使用已确认的服务和设备并通过环境注册器执行：

```bash
python3 "$project/skills/my-scrcpy-use/scripts/run-with-env.py" \
  --base-url "$base_url" --serial "$serial" -- \
  python3 "$project/skills/kuaishou-short-video/scripts/claim_video_reward.py"
```

`project`、`base_url`、`serial` 均使用已确认值。默认执行完整任务；`--mode watch` 单独进入首页验证观看模式；`--watch-seconds` 默认120秒，范围(0,14400]（最多4小时）。运行时限和录像随时长扩展，证据截图每分钟最多保存一次；视频刷新入口每60秒核验页面，收益页判定为“任务中心 + 我的金币唯一读数”（该机每次进入任务中心会弹签到/打卡弹窗，可能遮住日常任务）；首页视频流按底部导航文字核验，必要时用 XML 复核顶部 tab。OCR默认使用真机对比验证过的macocr，可通过--ocr-engine显式选择。

重生成方法：在仓库根目录执行以下命令，仅覆盖本技能生成文件：

```bash
python3 - <<'PY'
from pathlib import Path
import re
root = Path('skills/kuaishou-short-video')
for rule in sorted((root / 'rules').glob('*.md')):
    for name, code in re.findall(r'```python path=(scripts/[^\n]+)\n(.*?)\n```', rule.read_text(), re.S):
        target = root / name
        target.parent.mkdir(parents=True, exist_ok=True)
        target.write_text('# Generated from rules/' + rule.name + '; do not edit.\n' + code + '\n')
PY
```

真实执行只在用户授权后进行。观看满时长与平台签到完成分别报告，定位失败不得用固定坐标继续点击。
