---
name: mywebscrcpy-device-base
description: 使用 MyWebScrcpy 进行通用的 Android 屏幕捕获、UI XML/PageInfo 识别与手势按键输入。
---

# 手机操作基础 (Device Base)

为 AI 助手提供通用的 Android 设备底层屏幕捕获、控件层级树提取与精准输入模拟能力。

## 核心原则

每次设备动作前先进行只读采集：`page-info → XML → page-info`。按当前 Schema 核对前后包名、Activity 与页面状态；不稳定则有界重采（最多 3 次），仍不稳定则停止。XML 中的节点必须唯一、启用、可操作，且 bounds 位于当前可视区域。动作后重新采集页面并核对预期业务结果，不能把 HTTP 200 当作动作成功。

1. 服务地址与凭据：必须从环境变量 `MYWEBSCRCPY_API_URL` 或用户指令中读取已确认的服务地址。若缺失则立即停止并向用户确认，严禁回退到 `http://127.0.0.1:8080` 或任何默认端口。设备 `SERIAL` 同理，必须从 `GET /api/devices` 确认或由用户明确指定，缺失则停止确认，严禁假定默认设备。
2. 必须先通过服务地址读取当前 `/api/openapi.json` 确认接口路径与出入参，不猜测接口结构。
3. 必须明确目标设备 serial；未指定或多设备在线时先通过 `GET /api/devices` 查询并与用户确认，不硬编码 serial。
4. 设备动作执行前后必须进行页面确认；通过 `GET /api/ui/page-info` 与 `GET /api/ui/xml` 验证当前前台状态。
5. 所有网络请求使用本技能目录下的 `scripts/request_client.py`，无需额外第三方依赖。调用时使用基于当前 SKILL.md 所在目录的绝对路径，确保从任意项目工作目录（cwd）均可直接运行。

## 常用流程与脚本调用

在任意工作目录下调用前，先确定脚本路径与目标设备。
助手在执行命令前，必须将 `<CONFIRMED_ABSOLUTE_SKILL_DIR>` 替换为实际刚读取的 `SKILL.md` 所在目录的绝对路径（例如 `/Users/.../.agents/skills/mywebscrcpy-device-base`）。
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
API_URL="${MYWEBSCRCPY_API_URL}"
```

### 1. 设备检查与连接确认
```bash
python3 "${CLIENT_PY}" --base-url "${API_URL}" /api/devices
```
解析返回的设备列表中 `state == "device"` 的目标设备，提取其 `serial`（经确认后存入环境变量 `SERIAL`）。若未指定且存在多台设备，必须向用户确认具体目标设备，禁止擅自选择。

### 2. 页面信息与控件树提取
```bash
# 检查前置变量
if [ -z "${SERIAL:-}" ]; then
  echo "错误: 未指定设备 SERIAL。请先执行 /api/devices 查询可用设备并向用户确认后导出 SERIAL。" >&2
  exit 1
fi

# 获取前台应用包名与当前 Activity
python3 "${CLIENT_PY}" --base-url "${API_URL}" /api/ui/page-info --query "serial=${SERIAL}"

# 获取当前界面的 UIAutomator XML 结构树
python3 "${CLIENT_PY}" --base-url "${API_URL}" /api/ui/xml --query "serial=${SERIAL}"
```

### 3. 按键与系统操作注入
```bash
# 发送系统按键（例如返回键 keycode 4，主屏键 keycode 3）
# 注意：serial 必须作为 query 参数传递；body 严格遵循 SendKeyRequest 模式（仅 keycode 与 modifiers）
python3 "${CLIENT_PY}" --base-url "${API_URL}" --method POST /api/sendkey \
  --query "serial=${SERIAL}" \
  --header Content-Type=application/json \
  --json-file - << 'EOF'
{"keycode":4}
EOF
```
