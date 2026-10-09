# v0.6.0-feature 会话记录

## 记录原则

- 用户原话完整记录。
- 实际工具回复精简记录，只保留结论、承诺、关键问题和关键决策。
- 每次用户对话都要追加记录。

## 会话

### 1. OpenCode（目标模式继续推进）

- 按迁移规划开工 v0.6.0 模板匹配。
- 研读 `template-match.js`（439 行）+ 线上实测契约：`GET /api/templates`（含 threshold/method/scales/enabled/image_count）、`GET /api/templates/matches`（{count,matches}）。
- 存量模板：`chreome`（全局，阈值 0.67），正好用于联调定位点击。

### 联调记录

- click 接口以 Go 源码为准：`POST /api/templates/click`（{serial,template_id,mode}）真实存在；当前环境报 `adb screencap disabled`，属后端限制，前端链路与错误呈现已验证。
- 修删除静默失败：Reka AlertDialogAction 先关弹窗（清空展示态）再调用户 @click，onDelete 拿到 null 直接返回 → 删除 id 单独存放（FilesView 因状态分离天然免疫）。
- 阈值改自由文本为 ±0.05 步进器（回车/失焦在自动化与部分场景不可靠，点击最稳）。
- 开关/作用域/步进/删除 E2E 全过；`chreome` 恢复原状；测试模板已清。
