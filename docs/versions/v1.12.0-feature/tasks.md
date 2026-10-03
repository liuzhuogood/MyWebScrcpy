# v1.12.0-feature 任务清单

## 1. 规格与设计

- ✅ 完成 session/design/tasks/version 初稿。
- ✅ 确认影响范围（前端 `MyWebScrcpyNew` + 后端 `MyWebScrcpy`）与回滚策略。

## 2. 实现（前端 MyWebScrcpyNew）

- ⬜ 2.1 显示尺寸并入设置面板：迁移 Slider + 适应窗口，删除 `sizeOpen` 与「更多」菜单条目。
- ⬜ 2.2 手机边框：内置机型 CSS/SVG 绘制，扩展 `fitCanvas` 屏幕区对齐。
- ⬜ 2.3 录制选项 UI：录制面板增加「收录边框」「收录点击反馈」，同步 `POST /api/recordings` 参数。

## 3. 实现（后端 MyWebScrcpy）

- ⬜ 3.1 录制期间记录触控事件时序日志。
- ⬜ 3.2 边框/点击反馈合成（ffmpeg 后处理或实时叠加，路径待确认）。
- ⬜ 3.3 `POST /api/recordings` 接收 `record_frame`/`record_touch` 参数并持久化。

## 4. 验证

- ⬜ 4.1 前端尺寸/边框在横竖屏与多分辨率下对齐正确。
- ⬜ 4.2 录制选项前后端联调，产物含/不含边框与点击反馈符合预期。
- ⬜ 4.3 `go test ./...`、`go vet ./...`、`go build ./...`（后端改动）。

## 5. 发布

- ⬜ 5.1 更新版本摘要与发布/交付说明。
- ⬜ 5.2 按用户授权提交与部署。