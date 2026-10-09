# v0.6.0-feature 设计文档

## 背景

- 后端纯 Go 模板匹配（热重载 + 即时重测）；前端负责资产管理与定位点击。
- 存量全局模板 `chreome` 可直接用于联调。

## 目标与非目标

**目标：**

- 投屏工具栏模板入口，管理弹窗：列表、全部/设备/全局筛选、上传单张、导入/导出 ZIP、启用开关、阈值调整、设备↔全局切换、追加参考图、下载原图、删除、定位点击。
- 匹配命中 overlay：轮询 matches，绿框叠加，可见性开关（`show-green-boxes` 与老 key 互通，默认开）。
- 选区存为模板：依赖 v0.7 截图选区，本版只预留入口（按钮置灰 + 提示）。

**非目标：**

- 多尺度/方法/灰度参数编辑（仅展示；阈值可调）。
- Vision 通道识别框（独立特性，不在本版）。

## 关键决策

### 1. 管理界面用 Dialog 而非右侧面板

模板管理是低频弹窗操作，Dialog 足够；与检查器常驻面板区分。

### 2. 命中 overlay 独立轻量实现

不复用检查器 overlay（其显隐与面板耦合）；PlayerView 内第二块 overlay canvas + `useTemplateMatches` 轮询绘制，开关持久化沿用老 key。

### 3. 定位点击直接调后端 click 接口

`POST /api/templates/click` 服务端定位 + 注入（sdk/adb 双模式），前端只传参并 toast 结果。

## 风险与权衡

- [click/images 接口无文档] 按老前端调用方式实现，联调时先探后调，不通则降级（隐藏对应按钮并记录）。

## 迁移计划

1. `src/api/templates.ts` + `src/template/useTemplates.ts` + `useTemplateMatches.ts` + `TemplatePanel.vue`。
2. PlayerView 加模板入口 + 命中 overlay + 绿框开关。
3. 真机联调：列表 → 存量模板定位点击 → 上传/阈值/开关/删除（用测试模板，事后清理）→ 命中 overlay 显示。

## 回滚计划

1. 模板入口隐藏即回滚；新增 `src/template/` 独立。

## 待确认问题

- 无。
