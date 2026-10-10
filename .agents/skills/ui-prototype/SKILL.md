---
name: ui-prototype
description: 在 MyWebScrcpy 中使用现有 Vue 组件和主题创建、修改可交互界面原型，独立预览并保留方案版本。适用于先看原型或比较布局，不用于直接实现正式业务功能。
---

# 项目界面原型

产出可运行的 Vue 原型，复用项目源码中的组件和主题。项目根目录为包含 `frontend/package.json` 与 `prototypes/` 的 MyWebScrcpy 仓库。

## 读取与边界

- 先读取 `frontend/AGENTS.md`、`frontend/docs/rules/coding.md`、`frontend/docs/rules/ui.md`、`prototypes/AGENTS.md` 和 `prototypes/README.md`。
- 核对 `frontend/package.json`、`frontend/components.json`、`frontend/src/style.css`，按界面需要查看相关组件的 props、事件、slots 和现有页面用法，不扫描或复制全部组件 API 文档。
- 原型任务默认只编辑 `prototypes/`。正式 `frontend/src/`、路由、主题和业务接口只读；技能本身与预览基础设施仅在任务明确涉及维护时修改。
- 原型运行独立 Vue 应用，不导入正式 `main.ts`、`App.vue` 或路由。检查业务组件的请求、Store、定时器、设备操作和存储副作用；只有提供明确、可验证的模拟依赖后才能引用。纯 UI 与纯布局组件优先复用。

## 生成与迭代

- 每个方案放入 `prototypes/cases/<功能名>-<版本>/`，包含 `meta.ts` 和 `Prototype.vue`。沿用 `cases/task-center-v1/` 的入口约定：`meta.ts` 默认导出 `{ title: '页面名称' }`，类型引用 `../../catalog` 的 `PrototypeMeta`；预览自动发现，无需注册正式路由。
- 按已确认的 props、slots、事件和数据类型使用组件，不猜字段或写多字段兜底。按钮、表单、弹窗等复用现有组件；普通布局元素允许使用 Vue 模板和项目 Tailwind 类，不用独立静态 HTML 重做整页 UI。
- 使用现有主题变量，不复制或覆盖全局主题。缺失的基础组件先报告缺口；原型任务不自动安装组件或修改正式组件库。
- 用本地数据与 Vue 状态完成所需交互。业务模型已有类型时按其结构造数据；新设计的数据明确限定在原型内。不要连接真实 API、设备、正式 Store 或持久化存储。
- 按需求处理筛选、空结果、表单错误等状态；异步模拟须清理计时器。新方案另存目录，已有方案按用户指定更新，不自动删除旧版本。
- 产品页面只保留自然业务文案，不加入实现说明、修改结果或调试提示。预览壳上的原型选择与明暗切换由独立入口提供。

## 验证与交付

1. 检查依赖已安装，运行 `pnpm --dir frontend proto:check` 与 `pnpm --dir frontend proto:build`。
2. 使用 `pnpm --dir frontend proto` 启动本地预览，打开 `http://127.0.0.1:5174/#<目录名>`。端口占用时报告，不终止未知进程或重启正式服务。
3. 实际验证主要交互、桌面与小屏、键盘和主题，检查控制台及真实请求边界。截图等验证产物放入 `prototypes/artifacts/`。
4. 核对 Git 改动范围，交付预览链接、原型目录和验证结论；不要把配置成功当作浏览器运行成功。

定稿接入正式页面属于后续实现任务，只有用户要求落地时才修改正式页面、路由和数据连接，并保留原型版本。
