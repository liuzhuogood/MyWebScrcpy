# v1.34.0-feature 任务清单

状态：全栈实施与门禁验证已通过，纳入 v2026.11.5-1 收口，实际 Git/发布状态以日历版本记录与远端为准。

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

- [x] **T4 [正式业务代码实施]**：
  - [x] 在 `frontend/src/components/home/` 创建正式版 `HomeIosDockBar.vue` 组件；
  - [x] 改造 `frontend/src/views/HomeView.vue`，彻底移除右侧 `ActionPanel` 布局与标签，引入 `HomeIosDockBar`；
  - [x] 改造 `frontend/src/components/home/HomePhoneView.vue`，实现设备卡片靠左紧凑排布（8px 紧凑间距，少数设备靠左不拉伸撑行）；
  - [x] 首页设备卡片展示主区接入原生鼠标拉框（Marquee Box Selection）批量框选，联动已选状态并防点击冲突；
  - [x] 废弃并清理 `frontend/src/components/home/ActionPanel.vue`；
  - [x] 同步更新 `docs/glossary.md` 术语字典（登记「iOS 悬浮底栏」并更新「操作」指向）。

- [x] **T5 [全栈门禁验证与交付发布]**：
  - [x] 执行 `pnpm --dir frontend check`（Vue/TypeScript 类型检查，通过 `vue-tsc -b`）；
  - [x] 执行 `pnpm --dir frontend build`（正式产物打包验证）；
  - [x] 执行 `go test ./...`（后端接口与单测回归，435 个测试全数通过）；
  - [x] 版本实施收口，正式发布说明关联 `docs/versions/v2026.11.5-1-fix/version.md`，由发布流程记录实际提交。

- [x] **T6 [卡片交互与选区体验优化]**：
  - [x] 设备卡片按序列号正序排列，与底栏编号方块顺序保持一致；
  - [x] 拉框选区改为视口级坐标（`Teleport` 至 `body` + `fixed`），可拖出卡片区域覆盖整个屏幕；
  - [x] 卡片容器补齐 `min-height`（填满剩余视口高度），空旷区域同样可作为拉框起点，不再因高度不足无法起拖；
  - [x] 点击设备卡片不再直接跳转详情页，改为打开投屏小窗（桌面端原生小窗，Web 端内嵌浮动小窗）；
  - [x] 小窗“更多”菜单新增「设备详情」入口，可直接进入完整投屏详情页；
  - [x] 手机卡片尺寸滑块范围扩到 10%~200%；
  - [x] 验证：`pnpm --dir frontend build`、`proto:check`、`go test ./...` 全数通过（投屏小窗按需加载，首页首屏包体积未增加）。
