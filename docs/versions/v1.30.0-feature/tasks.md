# v1.30.0-feature 任务清单

## 1. 规格与设计

- [x] 1.1 梳理需求，根据用户最新要求将范围聚焦于**投屏拖拽安装 APK**，移除应用管理/应用市场相关内容。
- [x] 1.2 完成可交互原型设计 `prototypes/cases/app-management-v1/`，经公网验证并实拍截图。
- [x] 1.3 采纳用户反馈，清理界面冗余技术说明，确认投屏极简拖放与确认弹窗设计。
- [x] 1.4 创建版本分支 `lz/v1.30.0-feature`，初始化版本工作流四件套文档并提交推送。
- [x] 1.5 响应最新需求：APK 弹窗提示是否安装，非 APK 文件弹窗提示是否保存到设备 Downloads；封装为公共功能同时覆盖投屏详情页与小窗模式。

## 2. 后端开发 (`internal/apps`)

- [x] 2.1 [实现][可并行] 编写核心数据结构与安装参数定义 (`internal/apps/types.go`)。
- [x] 2.2 [实现] 编写 APK 安装服务核心 (`internal/apps/manager.go`)：
  - 处理临时文件安全写入与 `defer os.Remove` 自动清理；
  - 参数化装配 `adb -s <serial> install [-r] [-t] [-g] [-d]`；
  - 实现 ADB 输出解析与常见错误码（降级冲突、空间不足、架构不兼容等）友好中文映射。
- [x] 2.3 [实现] 编写 HTTP 处理器 (`internal/apps/handlers.go`)，暴露 `POST /api/apps/install` 接口（Multipart 流式上传）。
- [x] 2.4 [测试] 编写单元测试 (`internal/apps/manager_test.go`) 覆盖安装参数拼接、成功解析与各错误分支。
- [x] 2.5 [实现] 在 `main.go` 中注册服务路由。

## 3. 前端公共封装与开发 (`frontend/src`)

- [x] 3.1 [实现][可并行] 编写前端 API 客户端 (`frontend/src/api/apps.ts`)，使用 XHR 封装支持实时上传进度回调的安装方法。
- [x] 3.2 [实现] 编写公共拖放逻辑 Composable (`frontend/src/composables/useDeviceFileDrop.ts`)，管理悬停防闪烁计数、拖拽类型感知（APK/普通文件）与双弹窗生命周期。
- [x] 3.3 [实现] 封装投屏画面拖拽高亮浮层组件 (`frontend/src/components/player/DeviceFileDropOverlay.vue`)，根据拖拽文件类型动态提示。
- [x] 3.4 [实现] 封装应用安装确认弹窗 (`frontend/src/components/player/AppInstallDialog.vue`)，展示文件名、格式化体积与核心参数开关（`-r`, `-g`, `-d`），支持上传/安装阶段进度条与错误重试。
- [x] 3.5 [实现] 封装保存到 Downloads 确认弹窗 (`frontend/src/components/player/SaveToDownloadsDialog.vue`)，复用成熟的 `uploadFile` 接口将非 APK 文件推送到 `/storage/emulated/0/Download/`，支持进度展示与取消中断。
- [x] 3.6 [实现] 在 `DevicePlayer.vue` 中挂载拖拽事件监听、高亮浮层与双弹窗，以公共能力形式同时赋能投屏详情页（`PlayerView.vue`）、Web 浮动小窗（`FloatingPlayer.vue`）和桌面原生小窗。

## 4. 验证与交付

- [x] 4.1 运行 Go 自动化测试与代码检查（`go test -v ./internal/apps/...` 全部通过）。
- [x] 4.2 运行前端 `pnpm run build` 和 `pnpm run proto:check`，确保 TypeScript / Vue 模板无错误。
- [x] 4.3 登记与更新术语字典 `docs/glossary.md`。
- [ ] 4.4 完善 `version.md` 并更新交付总结。
- [ ] 4.5 提交版本代码。
