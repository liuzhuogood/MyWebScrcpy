# v1.13.0-feature 版本说明：自动更新机制

更新日期：2026-10-10。

**状态：已实现。**

## 目标与完成情况

实现基于 GitHub Release 的自动更新检查与一键下载更新功能：
1. **版本检测与对比**：后端检测 GitHub 最新 Release，比对 SemVer（`internal/updater/checker.go`）。
2. **多平台资产适配**：依据当前系统（Linux / macOS / Windows / amd64 / arm64）精准匹配对应的 GitHub 构建资产。
3. **SSE 下载进度推流**：提供 `/api/version/upgrade-stream` SSE 端点，实时推流下载字节、总大小、百分比与下载速度。
4. **原地安全替换与重启**：将当前运行二进制备份为 `.old`，移动新二进制就位并赋予执行权限，支持延迟安全拉起与重启（`internal/updater/applier.go`）。
5. **前端 UI 闭环**：
   - 顶栏 `AppHeader.vue` 嵌入当前版本展示；
   - 发现新版本时展示高亮更新按钮与徽标；
   - 点击弹出 `UpdateNoticeDialog.vue`，展示版本号对比、Release Notes、实时平滑进度条、下载速率；
   - 更新完成后展示重启提示并自动探测服务恢复后刷新页面。

