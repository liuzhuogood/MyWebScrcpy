# v1.2.0 任务清单

## 后端（Go）

- [x] 1. 新建 `internal/scripts/` 包：路径安全校验工具（白名单正则 + `filepath.Rel` 防穿越）
- [x] 2. 实现 `scripts/` 工作目录管理：可由 `SCRIPTS_DIR` 环境变量覆盖，启动时自动建目录 + 写入示例分类/脚本
- [x] 3. 实现元数据解析/序列化：读取文件头 `# @key value`，新建/保存时自动写回 `@created`/`@updated`
- [x] 4. 实现分类 API：`GET/POST /api/scripts/categories`、`PATCH/DELETE /api/scripts/categories/{cat}`
- [x] 5. 实现脚本 API：`GET /api/scripts?category=`、`GET/POST/PUT/DELETE /api/scripts/{cat}/{name}`
- [x] 6. 在 `main.go` 注册上述路由（不改动现有路由）
- [~] 7. （可选）PUT 接口支持 `updated` If-Match 轻量并发保护，不一致返回 409

## 前端（Web）

- [x] 8. 新建 `web/scripts.html` 页面骨架 + 左右两栏布局
- [x] 9. 新建 `web/js/scripts.js`：分类列表加载、新建/重命名/删除分类
- [x] 10. 脚本列表加载（按分类）+ 新建/删除脚本
- [x] 11. 接入 CodeMirror 6（CDN），Python 语法高亮、行号；CDN 失败降级 textarea
- [x] 12. 脚本编辑：元数据表单（描述、参数）+ 代码编辑器 + 保存（PUT）
- [x] 13. 在 `web/css/style.css` 补充脚本管理页样式（复用现有配色/卡片风格）
- [x] 14. 在 `web/index.html` 头部添加「脚本管理」入口按钮

## 验证与收尾

- [x] 15. 本地启动验证：新建分类、新建脚本、编辑保存、刷新后数据仍在、删除分类连带删脚本
- [x] 16. 安全验证：构造 `../`、绝对路径、特殊字符请求，确认被拒绝、无法越出 `scripts/`
- [x] 17. 离线/CDN 失败场景：CodeMirror 加载失败能降级为 textarea 正常编辑保存
- [x] 18. 确认现有设备列表 / 播放器 / 大屏功能不受影响
- [ ] 19. 更新 README（项目结构、特性列表补「脚本管理」）
- [~] 20. 提交代码、打 v1.2.0 tag、发布 Release（提交/tag 由主控会话统一处理）

## 图例

- `[x]` 完成
- `[~]` 跳过/可选未做（第 7 项并发保护本版未做，第 20 项由主控会话处理）
- `[ ]` 未完成（第 19 项 README 待发布前补）
