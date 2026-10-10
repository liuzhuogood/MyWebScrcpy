# v1.14.0-feature 开发会话记录

- 2026-10-10：开启 v1.14.0-feature 版本规划。
- 完成 SQLite 表设计评估，确定纯 Go 驱动 `modernc.org/sqlite`，避免引入 CGO 造成跨平台交叉编译失效。
- 通过 `$ui-prototype` 完成前端交互与界面原型 `manual-device-v1` 的设计与验证。
- 用户确认方案（1=继续），正式开展后端 SQLite 接入与设备管理重构。
