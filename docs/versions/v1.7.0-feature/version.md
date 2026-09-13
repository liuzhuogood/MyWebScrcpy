# v1.7.0-feature 版本摘要

1. 在播放器“检查”面板新增“页面信息 (UI Package)”功能，支持查看当前前台 APP 应用包名、Activity 类名、焦点窗口及屏幕分辨率、DPI、旋转方向。
2. 新增安全只读后端服务 `internal/uipage` 与端点 `GET /api/ui/page-info?serial=...`。
3. 前端支持一键刷新与各核心字段（包名、类名、完整组件名）一键快捷复制。
