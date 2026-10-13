# v1.22.0-feature：Python SDK 与进程内模板匹配（设计完成，尚未实施）

1. 规划项目内可安装依赖包 `mywebscrcpy-sdk`，导入名 `mywebscrcpy`，统一封装模板资产 HTTP API 和 Vision WebSocket 协议。
2. 提供设备会话、取帧、单次匹配、等待目标、持续匹配及点击等同步 Python API；匹配运行于调用方进程，无需另启模板匹配服务。
3. 普通 API 与识图依赖分离，通过 `[vision]` 可选依赖安装 PyAV、OpenCV 与 NumPy；主服务和设备连接仍为远程操作前提。
4. 保留现有独立模板匹配工程与主服务协议；不恢复旧 matches/detect/click HTTP 接口，不增加前端界面。

本次仅完成版本设计与任务拆分，未实现 SDK、未运行真机验证、未修改正式版本号、未提交推送或发布包。包名为设计候选，未核对 PyPI 占用。

2026-10-13 衔接更新：用户确认基础 SDK 先在 v1.21 交付，本版沿用同一 mywebscrcpy 包继续扩展。已提供 Client/Device.adb/page_info，不代表本文规划的模板、Vision、匹配或专用动作已实现。
