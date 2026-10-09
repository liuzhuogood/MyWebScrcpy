# v0.4.0-feature 设计文档

## 背景

- 老文件管理操作当前手机 `/storage/emulated/0` 共享存储，入口在投屏页“更多 → 文件管理”。
- 接口全部 `?serial=` 作用域；错误体 `{error:{message,code}}`，`conflict` 需二次确认保留副本。

## 目标与非目标

**目标：**

- `/files?serial=`：面包屑、搜索、类型筛选、排序、分页加载更多、容量显示。
- 新建文件夹、重命名、移动（单/批量）、删除（单/批量）+ 撤销、下载、上传（进度/重试/取消/冲突保留副本）。
- 图片/视频/文本预览；投屏工具栏“更多”加文件管理入口。

**非目标：**

- 大文件断点续传、解压缩、APK 安装（老版本也没有）。

## 关键决策

### 1. 状态收敛到 `useFiles` composable

列表、选择、分页、上传队列全在 composable 内；视图只做渲染。上传用 XHR（fetch 无上传进度），其余走 fetch。

### 2. 通知用 Sonner，危险操作用 AlertDialog

- 删除成功 → Sonner 带“撤销” action（对齐老 toast + undo）。
- 删除确认、重命名/移动冲突 → AlertDialog（替代老 `confirm()`）。
- 上传 409 → Sonner action“保留副本”直接重传。

### 3. 表格用 shadcn Table + Checkbox，移动端允许横向滚动

行操作按钮组在窄屏下收进 DropdownMenu（桌面端直接展示）。

## 风险与权衡

- [中文文件名] 上传 multipart 沿用老字段（path/conflict/file），后端已验证。
- [大目录] 分页 page_size=100 + 加载更多，与老一致。

## 迁移计划

1. `src/api/files.ts` + `src/files/useFiles.ts` + `FilesView.vue`，路由 `/files`。
2. PlayerView 更多菜单加文件管理入口（带 serial 跳转）。
3. 真机联调：建文件夹 → 上传 → 预览 → 重命名 → 移动 → 删除 → 撤销 → 下载（用无害测试文件，事后清理）。

## 回滚计划

1. `/files` 独立新增，路由指回 Stub 即回滚；PlayerView 只增一个菜单项。

## 待确认问题

- 无。
