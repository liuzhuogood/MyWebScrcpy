# v1.32.0-feature 任务清单

状态：已完成实施前设计、契约自测、首页手机视图开发、批量面板与入口迁移；独立原型经真实浏览器验证通过；真机与端到端回归由主管进行验收中。正式版本维持 `v1.29.0`，后续版本保持未启动。

## 本轮规划

- ✅ P01 [探查/Scout][可并行] 核对竞品图片、功能手册与分享说明；边界：`design.md` 功能对照和官方链接；验收：区分官网声明、概念示意与客户端实测。
- ✅ P02 [探查/Scout][可并行] 核对首页、大屏、导航、设备模型、文件/安装与视频重放；边界：`design.md` 证据表；验收：每个“已有”项有当前源码路径，录屏与动作宏区分。
- ✅ P03 [探查/Scout][依赖: P01,P02] 将用户的手机视图、大屏迁入、入口移除、右侧批量面板与显示台数写入四件套；边界：本版本目录；验收：首版与后续版边界明确。
- ✅ P04 [审阅/Review][依赖: P03] 核对不恢复应用市场/旧任务中心、不猜测设备分享网络机制；边界：`design.md`、`session.md`；验收：待确认项不冒充能力。

## 实施前设计

- ✅ D01 [探查/Scout][依赖: P04] 核对大屏行为清单与旧路由现状；边界：`DashboardView.vue`、`DashTile.vue`、`routes.ts`、`design.md`；验收：逐项列明迁移行为和全部导航入口，旧路由按本版不修改的边界记录。
- ✅ D02 [实现/Worker][依赖: D01] 按 `ui-prototype` 创建首页第三视图与右侧批量面板的独立原型；边界：新增 `prototypes/cases/home-phone-batch-v1/`；验收：不改正式路由或业务页面。
- ✅ D03 [实现/Worker][依赖: D02] 增加每页台数、分页、多选及模拟部分失败交互；边界：同一原型目录；验收：显示数与选择数独立、模拟操作不调用真机接口。
- ✅ D04 [验证/Verify][依赖: D03] 浏览器实际操作原型并保存桌面/窄屏与执行结果截图；边界：`docs/versions/v1.32.0-feature/prototypes/` 与 `design.md`；验收：记录视口、实际交互、源码入口及未验证范围。
- ✅ D05 [审阅/Review][依赖: D04] 将原型结论和用户反馈收敛为正式实施范围；边界：本版四件套；验收：默认台数、上限与交互无未决阻塞。

## 状态与契约先行

- ✅ T01 [测试/Test][依赖: D05][可并行: T02,T03] 定义显示/选择状态断言；边界：新增 `frontend/tests/homePhoneView.test.mjs`；验收：0 台、页数回缩、非法偏好、筛选/翻页清选择、Mock 不建连的用例通过。
- ✅ T02 [测试/Test][依赖: D05][可并行: T01,T03] 定义批调度边界断言；边界：新增 `frontend/tests/deviceBatch.test.mjs`；验收：并发上限、去重、逐台失败、超时、取消、旧响应隔离和资源清理用例通过。
- ✅ T03 [探查/Scout][依赖: D05][可并行: T01,T02] 再核对动作接口与返回模型；边界：`internal/automation/routes.go`、`types.go`、`service.go`；验收：冻结 `events` 的实际契约，严格处理 ActionResult 与错误格式。

## 首页状态与手机视图

- ✅ I01 [实现/Worker][依赖: T01] 实现显隐、每页台数与分页偏好；边界：新增 `frontend/src/composables/useHomePhoneView.ts`；验收：只计算本页范围，恢复偏好使用确切键与类型。
- ✅ I02 [实现/Worker][依赖: I01] 实现本页选择模型；边界：新增 `frontend/src/composables/useDeviceSelection.ts`；验收：清选择规则与目标快照去重用例通过。
- ✅ I03 [实现/Worker][依赖: I01] 组合现有磁贴与分页为手机视图；边界：新增 `frontend/src/components/home/HomePhoneView.vue`；验收：仅挂载当前页真实在线且非 Mock 设备，显隐/尺寸/铺满可用。
- ✅ I04 [实现/Worker][依赖: I03] 增加独立选择区域与宿主事件；边界：`frontend/src/views/HomeView.vue`、`frontend/src/components/home/HomePhoneView.vue`；验收：选择不注入触控，保护 DeviceCard 脏改动。
- ✅ I05 [实现/Worker][依赖: I02,I04] 接入首页第三视图与唯一轮询；边界：`HomeView.vue`、`HomeControlCapsule.vue`；验收：三种视图切换和小窗不退化，不引入第二轮询。

## 右侧批量操作

- ✅ I06 [实现/Worker][依赖: T03][可并行: I01-I05] 封装既有单设备动作接口；边界：新增 `frontend/src/api/deviceActions.ts`；验收：严格按实际请求/响应与错误模型处理。
- ✅ I07 [实现/Worker][依赖: T02,I06] 实现有界派发、目标冻结与单台结果；边界：新增 `frontend/src/composables/useDeviceBatch.ts`；验收：并发和部分失败用例通过。
- ✅ I08 [实现/Worker][依赖: I07] 实现整批/单台超时、停止、卸载和旧响应隔离；边界：同一批调度模块；验收：无无限等待，未知结果不标成功，全部边界用例通过。
- ✅ I09 [实现/Worker][依赖: I02,I08] 实现右侧批量面板的设备选择、返回/主页/最近任务与结果；边界：新增 `frontend/src/components/home/BatchOperationPanel.vue`；验收：空选中禁用动作，显示设置仅为一个分区。
- ✅ I10 [实现/Worker][依赖: I05,I09] 首页接入面板收放与窄屏抽屉；边界：`HomeView.vue`、`BatchOperationPanel.vue`；验收：恢复入口可达、三种视图使用同一选择语义、结果不串批。

## 入口与术语迁移

- ✅ I11 [实现/Worker][依赖: I10] 移除 Web 桌面与移动菜单的大屏监控入口；边界：`frontend/src/components/layout/AppHeader.vue`；验收：各断点入口均消失，AI/文件/设置等入口保留。
- ✅ I12 [实现/Worker][依赖: I10][可并行: I11] 移除桌面原生菜单两处大屏入口；边界：`desktop/src/menu.rs`；验收：编译通过且对应平台菜单无遗留空项。
- ✅ I13 [实现/Worker][依赖: D01,I11,I12] 更新正式术语并核查路由改动未越界；边界：写入 `docs/glossary.md`，只读核对 `routes.ts`、`DashboardView.vue`；验收：首页手机视图和批量面板指向实际实现，未擅自删除旧路由或加入兼容跳转。

## 批量面板浮动化与离线设备过滤（2026-10-22 新增用户要求）

- ✅ I14 [实现/Worker] 将批量操作面板改为非模态浮动覆盖层；边界：`frontend/src/components/home/BatchOperationPanel.vue`、`HomeView.vue`；验收：面板打开关闭主布局及设备卡片完全不形变，彻底消除模态遮罩与焦点拦截，页面卡片/控制仍可自由交互，Esc快速关闭。
- ✅ I15 [实现/Worker] 实现首页设备过滤 composable 与控制胶囊开关；边界：`frontend/src/composables/useHomeFilter.ts`、`frontend/src/components/home/HomeControlCapsule.vue`、`HomeView.vue`；验收：默认仅显示在线设备（`state === 'device'`），开启开关显示离线与未授权设备，持久化布尔配置（`home_show_offline_devices`），在分页与搜索前组合生效，切换时清空多选。
- ✅ T04 [测试/Test] 编写首页过滤逻辑单元测试；边界：`frontend/tests/homeFilter.test.mjs`；验收：在线默认过滤、离线开关包含、与来源/关键字组合过滤用例通过。
- ✅ D06 [实现/Worker] 同步更新独立原型与设计文档；边界：`prototypes/cases/home-phone-batch-v1/Prototype.vue`、`design.md`、`glossary.md`；验收：原型中浮动图层与离线过滤开关表现与正式页面一致，原型构建通过。

## 批量面板预留布局、编号方块显隐与组件化重构（2026-10-22 最新用户要求）

- ✅ I16 [实现/Worker] 批量面板改为右侧预留空间布局，不遮挡设备卡片；边界：`HomeView.vue`、`BatchOperationPanel.vue`；验收：桌面端面板展开时与设备卡片并排预留专属位置，设备卡片不被遮挡；顶部搜索/筛选控制中枢占满整行不被挤压；窄屏自然流式堆叠且无遮罩。
- ✅ I17 [实现/Worker] 批量面板全面组件化重构；边界：`frontend/src/components/home/batch/*` 与 `BatchOperationPanel.vue`；验收：按职责清晰拆分为面板壳、编号显隐选择区、批量范围、离散按键动作、显示设置、逐台结果 6 个子组件，类型严格声明，代码结构可独立调整。
- ✅ I18 [实现/Worker] 编号小方块选择器彻底替代旧分页与显隐弹窗；边界：`useHomePhoneView.ts`、`HomePhoneView.vue`、`BatchDeviceGrid.vue`；验收：按 serial 字典序稳定映射 01..N，真实设备不伪造 60 台；点选编号切换显隐，支持“全显”与“只显已选”；隐藏后编号方块不丢且可恢复；彻底清理旧分页/偏好/事件分支死代码。
- ✅ T05 [测试/Test] 编写编号稳定映射与显隐交互边界行为测试；边界：`frontend/tests/homePhoneView.test.mjs`；验收：轮询乱序映射恒定、单台显隐、全显恢复、只显已选快照防递归等断言全部通过。
- ✅ D07 [实现/Worker] 新建独立交互原型 `home-phone-batch-v2`；边界：`prototypes/cases/home-phone-batch-v2/`；验收：保留历史方案，采用纯模拟数据禁真机请求，完整呈现预留面板、全宽中枢与编号方块显隐交互，构建通过。

## 首页手机视图紧密设备墙与统一格（2026-10-22 最新需求）

- ✅ I19 [实现/Worker] 手机视图格子同宽同高规范与 2px 紧密网格；边界：`frontend/src/utils/phoneGrid.ts`、`frontend/src/components/home/HomePhoneView.vue`；验收：小/中/大/铺满同档位所有格子等宽等高，横屏设备不撑出不同高度，横竖相邻边框间距严格 2px，卡片为紧密设备墙视觉（`rounded-[2px]`），窄屏无横向溢出。
- ✅ I20 [实现/Worker] DashTile contain 模式支持与黑边触控拦截；边界：`frontend/src/components/dashboard/DashTile.vue`；验收：支持 `mode="contain"`，画面等比居中不拉伸不裁剪，黑边区域点击不向设备发送触控指令，保持 DashboardView 既有调用不受影响。
- ✅ T06 [测试/Test] 紧密网格尺寸、contain 计算与黑边坐标拦截单元测试；边界：`frontend/tests/phoneGrid.test.mjs`；验收：覆盖间隙 2px、三档尺寸与顶栏高度、横竖屏等比居中尺寸及黑边触控拦截断言。
- ✅ D08 [实现/Worker] 同步更新独立交互原型 `home-phone-batch-v2`；边界：`prototypes/cases/home-phone-batch-v2/Prototype.vue`；验收：原型中同宽同高卡片与 2px 紧密网格表现一致，构建通过。

## 验收与收口

- ✅ V01 [验证/Verify][依赖: I13] 执行前端状态/调度/布局相关测试与生产构建；边界：`frontend/tests/` 与前端构建产物；验收：新用例、既有布局/电源用例和 `pnpm --dir frontend build` 通过。
- ⬜ V02 [验证/Verify][依赖: V01] 浏览器验证三种视图、台数/翻页、面板、筛选、显隐、全屏和加载失败；边界：真实运行首页及本版证据；验收：由主管亲自在真实浏览器验证并记录。
- ⬜ V03 [验证/Verify][依赖: V02] 两台及以上真机验证批量动作与单机独立控制；边界：所选测试设备；验收：由主管在真实真机中验证逐台页面变化与控制隔离。
- ⬜ V04 [验证/Verify][依赖: V03] 验证单台断线、执行超时、停止和离开页面；边界：批调度/投屏生命周期；验收：由主管在真实真机环境验收。
- ⬜ V05 [验证/Verify][依赖: V02][可并行: V03,V04] 逐级测实际设备规模与订阅回收；边界：测试环境的 4/9/16/25/36 档；验收：由主管实测容量指标。
- ⬜ V06 [验证/Verify][依赖: I12] 验证桌面原生菜单、全屏、小窗与 macOS 标题栏；边界：桌面构建和实际运行；验收：主管实测桌面行为。
- ⬜ V07 [审阅/Review][依赖: V01-V06] 独立检查最终 diff、跨设备误操作风险、无障碍及模块边界；边界：本版改动文件；验收：由主管最终独立审查。
- ⬜ V08 [验证/Verify][依赖: V07] 运行项目现有完整校验并更新记录；边界：本版最终工作区；验收：主管验收通过后收口。
- ⬜ V09 [实现/Worker][依赖: V08] 在后续业务实施获授权并全部验收后按 lz-vlog 完成版本提交与合并；边界：本版本分支；验收：本次不触发提交推送。

## 后续版本的启动条件

- ⬜ R33 [探查/Scout] v1.33 前置探查；边界：`internal/db/db.go`、`internal/device/manager.go`、`DashTile.vue` 与本版 `design.md` 路线图；验收：列明标签 Schema 与同步输入/旋转契约的缺口，后续另建该版细粒度任务。
- ⬜ R34 [探查/Scout] v1.34 前置探查；边界：`internal/files/handlers.go`、`internal/apps/handlers.go`、`frontend/src/scrcpy/useClipboardSync.ts` 与路线图；验收：明确分发映射、取消和成功去重需求，后续另建该版四件套。
- ⬜ R35 [探查/Scout] v1.35 前置探查；边界：`internal/automation/`、`internal/devicegate/`、`frontend/src/inspector/opLog.ts` 与路线图；验收：形成动作序列/执行租约/条件等待问题清单，不复活旧任务平台，后续另建该版四件套。
- ⬜ RC [探查/Scout] 候选能力分别研究；边界：本版 `design.md` 候选研究小节与独立实验记录；验收：通知、授权分享、非 ADB 接入分别写明网络/权限前提和可复现实验，未验证前不承诺交付日期。


### 统一格子最终发布门禁（主管复测追加）
- [x] 主管：开发页四档与390px窄屏同宽高、横纵2px及黑边点击不发送触控。
- [x] 主管：50项自动化、前端/原型构建、版本一致性及保护文件核对。
- [x] 实施者：修复动态Tailwind尺寸类在正式CSS缺失的P1；在 `phoneGrid.ts` 中维护静态字面量样式映射表，生产构建真实输出 `.w-[200px]`、`.h-[380px]`、`.w-[270px]`、`.h-[508px]`、`.w-[348px]`、`.h-[648px]` 及 `.gap-[2px]`；整合为唯一生产事实源 `PHONE_TIER_SPECS`，清理测试专用冗余常量与用例，49项自动化自测通过。
- [x] 主管：干净正式构建的桌面/390px四档布局、CSS规则、在线画面黑边点击、编号显隐与恢复；49项自动化及双构建通过。
- [ ] 主管：补充横屏真实设备和持续拖拽实际结果；完整批量/容量/原生验收仍待完成。
- 实施状态：用户新授权Pi修复批次已经主管独立复测，本轮发布样式P1关闭。完整版本仍待剩余硬件与原生验收，未提交或推送。
