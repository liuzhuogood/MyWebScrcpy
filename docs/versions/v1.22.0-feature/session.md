# v1.22.0-feature 会话记录

## 2026-10-13

**用户**：「看一下模板匹配的接口，它是怎么工作的？我的模板服务没启动的时候它是，它可以用吗？」

**Codex**：核对本地代码。Go 主服务提供模板管理、视频传输与结果广播，独立 Python 工程执行匹配。未启动独立工程时，模板管理可用，实际匹配无结果。旧 matches/status/detect/click 路由已移除。

**用户**：「如果在项目里面用 Parcel 做一个依赖包，对这些 API 做混装，你觉得是个什么样子。」

**Codex**：结合上下文将语音中的词理解为 Python 依赖包及 API 封装，提出 SDK 方案。调用 match/watch 时在调用方进程完成识图，无需另启匹配服务。用户未明确 Parcel 的字面含义，设计按后续承接的 Python SDK 方向推进；若后续纠正则调整方案。

**用户**：「那请你帮我搞一个版本设计一下。」

**Codex**：创建 v1.22.0-feature 四件套并加入版本总览；本次授权限于版本设计。核对模板路由、Vision 协议、独立 Python 工程及 OpenAPI。当前 VERSION 为 v1.19.2，最新已有记录为 v1.20.0-feature，本任务不属于桌面 App 版本，按 minor 递增创建独立记录。

关键决策：优先同步 SDK；HTTP 用 HTTPX，WebSocket 复用成熟 websocket-client，匹配采用现有 PyAV/OpenCV 算法；新包与旧工程并存，替换旧入口前另行确认。新包不做字段猜测、不默认切换到磁盘模板、不自动重试动作。

验证：仅文档路径、链接、范围和 git diff 检查；实现测试与安装、真机验收保持待办。

**用户**：「提交推送」；随后：「那先更新，本地的这个版本行暂存， 等更新完后，再换成新的版本号」。

**Codex**：将本次设计精确 stash，快进更新 main 至 a239c1b。远端 v1.21.0-feature 已用于 Pi 技能生成 Python，故恢复本次文档到 v1.22.0-feature，不覆盖远端记录。更新后正式 VERSION 为 v1.20.1；此前段落中的版本基线为创建当时的历史状态。继续用户已授权的 scoped 提交推送。

**2026-10-13 跨版衔接**：用户在 v1.21 实施会话要求重新采用 Python SDK 并核对本版设计，随后确认「先完成 SDK 基础能力并接入 v1.21」。基础包统一为 packages/python-sdk（mywebscrcpy-sdk 0.1.0 / mywebscrcpy），提供 HTTPX Client、Device.adb/page_info、结果与异常。完整 Vision 解码与模板匹配仍留本版，未实现；不重复连接封装，也不把本版标为完成。
