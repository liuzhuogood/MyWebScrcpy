# v1.33.0-feature 任务清单

状态：精简交付阶段 (Lean Delivery)，L1 - L4 顺利通过，等待 L5 提交与交付。

## 精简交付流程 (Lean Delivery: L1 - L5)

- ✅ L1 [设计确认] 原型验证完成：拟物手机卡片外观、1:1 对齐直接显示屏幕规格（270×508 / 200×380）、点击整屏投屏、去详情按钮、去 AI 弥散光斑。
- ✅ L2 [版本文档] 更新版本清单与精简交付记录；边界：`tasks.md`、`version.md`、`design.md`。
- ✅ L3 [业务实施] 将选定的拟物手机卡片正式重构至 `frontend/src/components/device/DeviceCard.vue`，并适配 `useDeviceGridVirtual.ts` 508px 拟物手机行高。
- ✅ L4 [全测门禁] 执行项目全量自动化测试套件（Go 后端测试全部通过、前端生产构建 `build` 成功通过、原型构建 `proto:build` 成功通过）。
- ✅ L5 [原型扩展] 完成首页右侧“操作”面板与无顶部Bar重构原型：
  - 沉淀交互原型：`prototypes/cases/home-action-panel-v1/`
  - 迁入顶部 Bar 功能、收起/展开动效、剔除紧凑阵列
  - 产出实拍截图证据：`action-panel-v1-expanded.png`、`action-panel-v1-collapsed.png`
  - 自动化构建门禁全部通过
- ⏳ L6 [提交与落地评估] 提交原型成果入库，等待评审授权后正式接入主视图。
