# v1.7.0-feature 实施任务

## 1. 规格与设计
- [x] 1.1 完成 session/design/tasks/version 初稿。
- [ ] 1.2 明确 Tab 文案与字段排版细节（包名、Activity、Component、Window、分辨率、DPI、方向）。

## 2. 后端服务 (internal/uipage)
- [ ] 2.1 新建 `internal/uipage` 模块，定义数据结构（PageInfo、DisplayInfo）。
- [ ] 2.2 实现 ADB 结果正则解析与回退容错逻辑（window/activity/wm/orientation）。
- [ ] 2.3 编写无真实设备依赖的单元测试，覆盖多种 Android 输出格式。
- [ ] 2.4 实现 HTTP Handler，并在 `main.go` 注册 `/api/ui/page-info` 路由。

## 3. 前端界面 (web/js/ui-inspector.js)
- [ ] 3.1 在“检查”面板头部增加第 3 个 Tab（UI Package / 页面信息）。
- [ ] 3.2 增加信息展示视图、刷新按钮、加载态与字段一键复制功能。
- [ ] 3.3 样式适配，保持与取色坐标、XML 树一致的深色现代主题风格。

## 4. 测试与验证
- [ ] 4.1 全量 Go 单测与静态检查。
- [ ] 4.2 真实连接设备（如 10.0.0.11）联调与展示验证。
- [ ] 4.3 验证复制交互与异常处理（如设备断开/离线）。
