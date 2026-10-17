---
name: mywebscrcpy-kuaishou-daily
description: 快手客户端的界面识别、滑动浏览与日常活动打卡辅助流程，需要结合 MyWebScrcpy 设备控制。
---

# 快手日常助手 (Kuaishou Daily)

专为快手短视频客户端定制的界面识别与交互辅助技能，支持在已授权的移动设备上完成推荐流浏览、特定组件识别与活动任务辅助流转。

## 核心原则

每次设备动作前先进行只读采集：`page-info → XML → page-info`。按当前 Schema 核对前后包名、Activity 与页面状态；不稳定则有界重采（最多 3 次），仍不稳定则停止。XML 中的节点必须唯一、启用、可操作，且 bounds 位于当前可视区域。动作后重新采集页面并核对预期业务结果，不能把 HTTP 200 当作动作成功。

1. 服务地址与设备确认：必须从环境变量 `MYWEBSCRCPY_API_URL` 或用户指令中读取已确认的服务地址。若缺失则立即停止确认，严禁回退到 `http://127.0.0.1:8080` 或任何默认端口。目标设备 `SERIAL` 必须通过 `GET /api/devices` 确认可用状态并经用户确认后明确指定，严禁假定默认设备。
2. 必须先读取当前服务 `/api/openapi.json` 确认接口契约，不猜测字段。
3. 动态验证与容错：严禁硬编码屏幕坐标（如不可硬编码 `540 1600` 等绝对像素）或假设固定组件位置。必须先执行 `page-info` 确认快手前台运行，再通过 `xml` 抓取节点并解析 `bounds` 动态计算中心坐标，完成边界检查后再执行动作。
4. 点击操作安全规范：
   - 必须携带当前页面上下文（通过 `page-info` 验证包名与 Activity）。
   - 唯一定位已启用且可见的节点（匹配明确属性，并校验 `enabled="true"` 且 `clickable="true"`）。
   - 操作后必须进行业务后置验证（如页面跳转、新弹窗或状态变化），并设置有界等待与重试限制（最多 3 次，超时明确退出）。
5. 严禁未经授权的账户敏感操作（如私信回复、打赏、支付等）；日常打卡与浏览需受控进行。

## 脚本调用与标准流程

在任意工作目录下调用前，先确定脚本路径与目标设备。
助手在执行命令前，必须将 `<CONFIRMED_ABSOLUTE_SKILL_DIR>` 替换为实际刚读取的 `SKILL.md` 所在目录的绝对路径（例如 `/Users/.../.agents/skills/mywebscrcpy-kuaishou-daily`）。
严禁使用 `BASH_SOURCE` 或 `$0` 进行自动推导（在交互式终端粘贴或临时脚本执行时会指向错误目录）。

```bash
# 确认绝对技能目录与脚本绝对路径（由执行助手填写已确认的绝对路径，不依赖 cwd）
SKILL_DIR="<CONFIRMED_ABSOLUTE_SKILL_DIR>"
CLIENT_PY="${SKILL_DIR}/scripts/request_client.py"

# 服务地址与设备检查（缺失时必须停止并向用户确认，禁止使用任何 8080 默认或硬编码）
if [ -z "${MYWEBSCRCPY_API_URL:-}" ]; then
  echo "错误: 未配置 MYWEBSCRCPY_API_URL。请先确认服务地址并导出环境变量，禁止预设任何默认地址。" >&2
  exit 1
fi
if [ -z "${SERIAL:-}" ]; then
  echo "错误: 未指定设备 SERIAL。请先执行 /api/devices 查询可用设备并向用户确认后导出 SERIAL。" >&2
  exit 1
fi
API_URL="${MYWEBSCRCPY_API_URL}"
```

### 1. 确认前台应用与屏幕就绪
调用 `/api/ui/page-info` 获取当前设备前台应用包名与 Activity：
```bash
python3 "${CLIENT_PY}" --base-url "${API_URL}" /api/ui/page-info --query "serial=${SERIAL}"
```
验证返回结果中包含快手相关包名（例如 `com.smile.gifmaker` 或其极速版等同系变体）。若当前非快手前台，先提示用户或通过应用启动流程打开，绝不盲目注入输入。

### 2. 界面分析与动态手势滑动
通过 `/api/ui/xml` 获取当前界面的层次树与屏幕边界信息：
```bash
python3 "${CLIENT_PY}" --base-url "${API_URL}" /api/ui/xml --query "serial=${SERIAL}"
```
解析当前屏幕分辨率或根节点 `bounds`，严禁使用固定坐标值。由 Python 根据实时屏幕尺寸计算滑动起点与终点，并通过 `json.dumps` 动态生成请求体：
```bash
# SCREEN_WIDTH / SCREEN_HEIGHT 必须由本次稳定页面的实际截图或 XML bounds 提取。
# 缺少任何数值时停止；下列命令不提供尺寸默认值。
: "${SCREEN_WIDTH:?缺少已确认的实际屏幕宽度}"
: "${SCREEN_HEIGHT:?缺少已确认的实际屏幕高度}"
SWIPE_PAYLOAD="$(python3 - "$SCREEN_WIDTH" "$SCREEN_HEIGHT" <<'PYCODE'
import json, sys
w, h = map(int, sys.argv[1:])
if w < 2 or h < 2:
    raise SystemExit("屏幕尺寸无效")
print(json.dumps({"args": ["shell", "input", "swipe", str(w // 2), str(h * 4 // 5), str(w // 2), str(h // 5), "300"]}))
PYCODE
)" || exit 1
printf '%s\n' "$SWIPE_PAYLOAD" | python3 "${CLIENT_PY}" --base-url "${API_URL}" --method POST /api/adb/commands \
  --query "serial=${SERIAL}" --json-file -
```
滑动后等待视频缓冲与播放停留，然后调用 `/api/ui/page-info` 进行后置业务验证，确认是否成功停留在推荐视频流页面。

### 3. 日常签到与活动入口识别与安全点击
从 `/api/ui/xml` 的控件树中检索目标活动节点（例如“签到”、“福利”、“领金币”）：
1. **上下文校验**：再次通过 `/api/ui/page-info` 确认前台仍为快手页面；
2. **唯一节点定位**：确保选取的节点在层次树中唯一匹配，且属性 `enabled="true"` 且 `clickable="true"`（或其可点击父级）；
3. **坐标动态计算与有效性校验**：解析节点 `bounds="[x1,y1][x2,y2]"`，计算中心点 `cx = (x1 + x2) // 2`，`cy = (y1 + y2) // 2`，并验证坐标落在屏幕可视区域内；
4. **生成请求体**：使用 Python `json.dumps({"args": ["shell", "input", "tap", str(cx), str(cy)]})` 生成标准 JSON；
5. **后置业务验证与有限重试**：点击注入后，进行有界等待（如轮询间隔 1s，最多重试 3 次），再次调用 `/api/ui/page-info` 与 `/api/ui/xml` 验证目标弹窗是否打开或打卡状态已更新。
