# v1.13.0-feature 任务清单：自动更新机制

- [x] 后端：版本管理与 GitHub Release 检查模块 (`internal/updater/checker.go`)
- [x] 后端：平台资产匹配与流式下载进度管理 (`internal/updater/downloader.go`)
- [x] 后端：原地二进制文件替换与重启机制 (`internal/updater/applier.go`)
- [x] 后端：更新 API 与 SSE 进度流路由接口 (`internal/updater/handlers.go`, `main.go`)
- [x] 后端：单元测试与接口集成测试 (`internal/updater/updater_test.go`)
- [x] 前端：自动更新 API 与 Pinia 状态管理 (`frontend/src/api/updater.ts`, `frontend/src/stores/updater.ts`)
- [x] 前端：更新弹窗与下载进度条组件 (`frontend/src/components/updater/UpdateNoticeDialog.vue`)
- [x] 前端：顶栏 `AppHeader.vue` 嵌入当前版本与更新徽标按钮
- [x] 验证：构建测试、类型检查、SSE 进度与自更新链路测试
