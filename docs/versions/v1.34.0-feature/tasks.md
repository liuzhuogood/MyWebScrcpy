# v1.34.0-feature 任务清单

状态：需求与原型验证已就绪，已完成版本设计三件套归档，待正式实施合入主干。

---

## 阶段进展清单

- [x] **T1 [需求澄清与架构收敛]**：
  - [x] 确定彻底移除右侧 320px 固态操作侧栏，解放首页 100% 全宽视口；
  - [x] 确立 iOS 风格悬浮毛玻璃底栏架构（贴底居中悬浮、高斯模糊、可折叠为微型气泡）；
  - [x] 四功能按钮精炼：【操作】（批量导航与同步）、【视图】（纯净文字与三滑动调节）、【设备】（搜索+添加+在线已选+编号矩阵）、【更多】（系统服务与文档）；
  - [x] 色彩体系归正为 iOS 纯正灵动蓝（`#007AFF` / `blue-600`），全面剔除旧代码硬编码紫色；
  - [x] 设备矩阵区加入鼠标原生拉框框选（Marquee Box Selection）能力，并纳管离线设备。

- [x] **T2 [独立原型交互与全量证据归档]**：
  - [x] 独立原型案例落地：`prototypes/cases/home-ios-bottom-bar-v1/`；
  - [x] 完成公网自动化预览部署：`https://demo.wskj.work/#home-ios-bottom-bar-v1`；
  - [x] 截取并归档 7 张全流程实拍图（默认态、操作展开、视图展开、设备展开、鼠标拉框中、更多展开、暗色模式）；
  - [x] 通过 `pnpm --dir frontend proto:check` 与 `proto:build` 验证。

- [x] **T3 [版本资产三件套归档]**：
  - [x] 完成 `docs/versions/v1.34.0-feature/version.md`；
  - [x] 完成 `docs/versions/v1.34.0-feature/design.md`（包含架构图、时序图与 AABB 碰撞检测模型）；
  - [x] 完成 `docs/versions/v1.34.0-feature/tasks.md`。

- [ ] **T4 [正式业务代码实施]**：
  - [ ] 在 `frontend/src/components/home/` 创建正式版 `HomeIosDockBar.vue` 组件；
  - [ ] 改造 `frontend/src/views/HomeView.vue`，彻底移除右侧 `ActionPanel` 布局与标签，引入 `HomeIosDockBar`；
  - [ ] 废弃并清理 `frontend/src/components/home/ActionPanel.vue`；
  - [ ] 同步更新 `docs/glossary.md` 术语字典（登记「iOS 悬浮底栏」并更新「操作」指向）。

- [ ] **T5 [全栈门禁验证与交付发布]**：
  - [ ] 执行 `pnpm --dir frontend check`（Vue/TypeScript 类型检查）；
  - [ ] 执行 `pnpm --dir frontend build`（正式产物打包验证）；
  - [ ] 执行 `go test ./...`（后端接口与单测回归）；
  - [ ] 提交代码并记录正式版本提交。
