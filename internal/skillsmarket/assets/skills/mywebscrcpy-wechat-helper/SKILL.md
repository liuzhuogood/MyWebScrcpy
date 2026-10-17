---
name: mywebscrcpy-wechat-helper
description: 微信联系人与会话检索、未读消息读取与上下文草拟回复辅助。仅读取与草拟，绝不自动外发任何消息。
---

# 微信消息助手 (WeChat Helper)

协助在手机微信中快速检索目标联系人或群组会话，提取最新收到的文本消息并结合上下文草拟回复。

## 核心安全准则（强制）

> **重要**：本技能仅提供读取与草拟能力，**绝不自动向外发送任何消息**！所有向外发送的回复文本必须输出给用户审阅，并在手机微信内由人工最终确认并手动点击发送。严禁调用按键、手势注入或模拟点击“发送”按钮，严禁发送回车键（keycode 66）触发外发。

## 核心原则

每次设备动作前先进行只读采集：`page-info → XML → page-info`。按当前 Schema 核对前后包名、Activity 与页面状态；不稳定则有界重采（最多 3 次），仍不稳定则停止。XML 中的节点必须唯一、启用、可操作，且 bounds 位于当前可视区域。动作后重新采集页面并核对预期业务结果，不能把 HTTP 200 当作动作成功。

1. 服务地址与凭据：必须从环境变量 `MYWEBSCRCPY_API_URL` 或用户指令中读取已确认的服务地址。若缺失则立即停止确认，严禁回退到 `http://127.0.0.1:8080` 或任何默认端口。目标设备 `SERIAL` 必须通过 `/api/devices` 明确查询确认，缺失则停止确认。
2. 必须先读取当前服务 `/api/openapi.json` 确认接口契约与可用路由，不猜测字段。
3. 闭环验证流程（page-info -> XML -> 操作 -> page-info）：每次交互前后必须验证前台状态与页面跳转。
4. 仅读与草拟约束：本技能所有输出仅为建议草稿，在 AI 助手交互界面呈现供用户审阅。

## 脚本调用与标准流程

在任意工作目录下调用前，先确定脚本路径与目标设备。
助手在执行命令前，必须将 `<CONFIRMED_ABSOLUTE_SKILL_DIR>` 替换为实际刚读取的 `SKILL.md` 所在目录的绝对路径（例如 `/Users/.../.agents/skills/mywebscrcpy-wechat-helper`）。
严禁使用 `BASH_SOURCE` 或 `$0` 进行自动推导（在交互式终端粘贴或临时脚本执行时会指向错误目录）。

```bash
# 确认绝对技能目录与脚本绝对路径（由执行助手填写已确认的绝对路径，不依赖 cwd）
SKILL_DIR="<CONFIRMED_ABSOLUTE_SKILL_DIR>"
CLIENT_PY="${SKILL_DIR}/scripts/request_client.py"

# 服务地址检查（缺失时必须停止并向用户确认，禁止使用任何 8080 默认或硬编码）
if [ -z "${MYWEBSCRCPY_API_URL:-}" ]; then
  echo "错误: 未配置 MYWEBSCRCPY_API_URL。请先确认服务地址并导出环境变量，禁止预设任何默认地址。" >&2
  exit 1
fi
API_URL="${MYWEBSCRCPY_API_URL}"
```

### 0. 契约确认与设备可用性检查
```bash
# 1. 读取 OpenAPI 规范确认接口路径
python3 "${CLIENT_PY}" --base-url "${API_URL}" /api/openapi.json

# 2. 查询在线可用设备
python3 "${CLIENT_PY}" --base-url "${API_URL}" /api/devices
```
确认目标设备处于 `state == "device"` 后，将确认的序列号导出到 `SERIAL` 环境变量中：
```bash
if [ -z "${SERIAL:-}" ]; then
  echo "错误: 未指定设备 SERIAL。请根据 /api/devices 结果向用户确认目标设备后导出 SERIAL。" >&2
  exit 1
fi
```

### 1. 确认微信处于前台且设备就绪（闭环第一步：page-info）
调用 `/api/ui/page-info` 确认微信（包名 `com.tencent.mm`）处于前台且屏幕点亮：
```bash
python3 "${CLIENT_PY}" --base-url "${API_URL}" /api/ui/page-info --query "serial=${SERIAL}"
```
若屏幕熄灭或当前非微信界面，停止后续操作并提示用户先点亮屏幕并切换至微信。

### 2. 检索指定联系人或会话（闭环第二步与第三步：XML -> 轻触 -> page-info）
调用 `/api/ui/xml` 抓取当前界面层次树：
```bash
python3 "${CLIENT_PY}" --base-url "${API_URL}" /api/ui/xml --query "serial=${SERIAL}"
```
- 解析搜索按钮控件或会话列表条目，定位唯一可见可点击节点；
- 计算目标控件中心坐标并在用户确认后轻触进入对应聊天页面；
- **后置闭环验证**：再次调用 `/api/ui/page-info` 确认已成功进入聊天 Activity（例如 `com.tencent.mm.ui.chatting.ChattingUI`）：
```bash
python3 "${CLIENT_PY}" --base-url "${API_URL}" /api/ui/page-info --query "serial=${SERIAL}"
```

### 3. 读取未读或最新消息（闭环第四步：XML）
通过 `/api/ui/xml` 提取当前对话列表中的消息节点：
```bash
python3 "${CLIENT_PY}" --base-url "${API_URL}" /api/ui/xml --query "serial=${SERIAL}"
```
- 识别文本消息内容（`android.widget.TextView`）与头像归属；
- 提取最近 3~5 条消息的文本内容与发送者，按时间先后顺序整理；
- 将消息摘要呈现给用户。

### 4. 草拟回复建议（仅在 AI 助手内输出，绝不自动外发）
结合对话上下文，在 AI 助手回复中输出若干风格得体的建议草稿供用户选择：
- 建议草稿 1（简短确认）
- 建议草稿 2（详细回复）
- 提示说明：“以上内容为草拟建议，请审阅确认后复制并在手机微信内手动发送。本助手严格遵循只读草拟原则，不执行任何自动发送动作。”
