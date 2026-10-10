# v1.13.0-feature 开发会话记录

## 会话背景
用户需求：设计一个自动更新机制，当 github 上有新的 tag 版本时，提示有更新，且可以点击自动更新，显示下载进度。

## 记录
- 2026-10-10：梳理整体架构设计（GitHub API 检查 -> 平台资产适配 -> SSE 实时推流下载进度 -> 原地替换 -> 重启与前端轮询就绪）。
- 建立 `v1.13.0-feature` 四件套文档，确认后进入实施。
- 完成 Go 后端 `internal/updater`（检查、流式下载、热替换、SSE 与状态接口）及单元测试。
- 完成 Vue 前端 API、Pinia Store、`UpdateNoticeDialog` 更新弹窗与 `AppHeader` 状态集成。
- 完成全量构建与实机端点测试验证。
