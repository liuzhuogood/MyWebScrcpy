# v1.21.0-feature：Pi 技能生成 Python

第一阶段实施完成，版本分支 d8bbb5f 已合入 main（0742996），全量复测与远端推送核对通过；正式 VERSION 保持 v1.20.1。

1. 项目 Pi 技能将 Markdown 转为使用 mywebscrcpy SDK 的独立 Python，生成阶段只读资料。
2. 统一可安装 SDK 基础版 0.1.0 提供连接、设备绑定、ADB、页面尺寸和明确结果/异常；应用内外共享源码。
3. 独立 SSE 生成、思考与工具活动、停止、严格 JSON/AST 校验、最多两次修正及事务保存已接入。
4. 删除旧占位生成器，取消手动/Cron 执行前自动编译；只运行有效产物，编辑后保留旧源码并阻止过期执行。
5. 真正的视频解码与模板匹配留在 v1.22；不把基础 SDK 当作完整匹配交付。

受控 Pi/设备接口、浏览器和本机软件测试已执行。真机、Windows/Linux 原生运行与远端发布 CI 尚未验收；未部署、未发布标签或 PyPI。细节见 [design.md](design.md)、[tasks.md](tasks.md)、[session.md](session.md)。
