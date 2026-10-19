# v1.30.0-feature 任务清单

## 1. 规格与设计

- [x] 1.1 梳理应用安装与应用管理需求，完成总体实施计划 `app_management_plan.md`。
- [x] 1.2 完成可交互原型设计 `prototypes/cases/app-management-v1/`，经公网验证并实拍截图。
- [x] 1.3 采纳用户反馈，清理界面冗余技术说明与多余卡片，确认双入口（抽屉内嵌 + 全屏独立页）方案。
- [x] 1.4 创建版本分支 `lz/v1.30.0-feature`，初始化版本工作流四件套文档。

## 2. 后端开发 (`internal/apps`)

- ⬜ 2.1 [实现][可并行] 编写核心数据结构与配置参数定义 (`internal/apps/types.go`)。
- ⬜ 2.2 [实现] 编写应用管理器服务 (`internal/apps/manager.go`)：
  - 实现 `ListPackages`（合并 `pm list packages -f -3`、`-s` 和 `-d`，毫秒级快速分类）；
  - 实现 `InstallApk`（封装 `adb install [-r] [-t] [-g] [-d]` 及错误码友好中文映射）；
  - 实现 `Launch`, `ForceStop`, `ClearData`, `SetEnabled`, `Uninstall`, `ExportApk`。
- ⬜ 2.3 [实现] 编写 HTTP 处理器 (`internal/apps/handlers.go`)，暴露 `/api/apps` 相关 REST 接口与 Multipart 上传流。
- ⬜ 2.4 [测试] 编写单元测试 (`internal/apps/manager_test.go`, `handlers_test.go`) 覆盖关键边界与异常路径。
- ⬜ 2.5 [实现] 在 `main.go` 组装服务并在 `webapp.go` 中注册 `/apps` 路由白名单。

## 3. 前端开发 (`frontend/src`)

- ⬜ 3.1 [实现][可并行] 封装应用管理前端 API 客户端 (`frontend/src/api/apps.ts`)。
- ⬜ 3.2 [实现][可并行] 编写响应式应用管理 Composable (`frontend/src/composables/useApps.ts`)。
- ⬜ 3.3 [实现] 移植并完善安装确认弹窗 (`frontend/src/components/apps/AppInstallDialog.vue`)。
- ⬜ 3.4 [实现] 移植并完善应用详情弹窗 (`frontend/src/components/apps/AppDetailDialog.vue`)。
- ⬜ 3.5 [实现] 编写应用管理主视图 (`frontend/src/views/AppsView.vue`)，支持独立全屏与抽屉内嵌双模式。
- ⬜ 3.6 [实现] 在投屏播放器 (`frontend/src/components/player/DevicePlayer.vue`) 集成：
  - 画布容器的 `dragover`/`dragleave`/`drop` 事件监听，拖入 `.apk` 弹出安装确认；
  - 右侧抽屉内嵌应用管理面板与顶栏更多菜单入口。
- ⬜ 3.7 [实现] 在路由配置 (`routes.ts`) 与设备卡片 (`DeviceCard.vue`) 注册全屏应用管理入口。

## 4. 验证与交付

- ⬜ 4.1 运行 Go 自动化测试与代码静态检查。
- ⬜ 4.2 运行前端 `pnpm run build` 确保无 TypeScript / Vue 模板错误。
- ⬜ 4.3 登记与更新术语字典 `docs/glossary.md`。
- ⬜ 4.4 完善 `version.md` 并更新 `docs/versions/README.md`。
- ⬜ 4.5 提交版本分支代码并生成交付总结。
