# v1.32.0-feature 任务清单

本轮授权为版本规划。只有规划核对项已完成；下面的原型、测试与业务实现均未开始。
每项以 10～20 分钟为拆分目标，超过时继续按文件/行为拆小；后续版本不在这里伪装成已授权实施任务。

## 本轮规划

- ✅ P01 [探查/Scout][可并行] 核对竞品图片、功能手册与分享说明；边界：`design.md` 功能对照和官方链接；验收：区分官网声明、概念示意与客户端实测。
- ✅ P02 [探查/Scout][可并行] 核对首页、大屏、导航、设备模型、文件/安装与视频重放；边界：`design.md` 证据表；验收：每个“已有”项有当前源码路径，录屏与动作宏区分。
- ✅ P03 [探查/Scout][依赖: P01,P02] 将用户的手机视图、大屏迁入、入口移除、右侧批量面板与显示台数写入四件套；边界：本版本目录；验收：首版与后续版边界明确。
- ✅ P04 [审阅/Review][依赖: P03] 核对不恢复应用市场/旧任务中心、不猜测设备分享网络机制；边界：`design.md`、`session.md`；验收：待确认项不冒充能力。

## 实施前设计

- ⬜ D01 [探查/Scout][依赖: P04] 核对大屏行为清单与旧路由现状；边界：`DashboardView.vue`、`DashTile.vue`、`routes.ts`、`design.md`；验收：逐项列明迁移行为和全部导航入口，旧路由按本版不修改的边界记录。
- ⬜ D02 [实现/Worker][依赖: D01] 按 `ui-prototype` 创建首页第三视图与右侧批量面板的独立原型；边界：拟新增 `prototypes/cases/home-phone-batch-v1/` 与原型注册入口；验收：不改正式路由或业务页面。
- ⬜ D03 [实现/Worker][依赖: D02] 增加每页台数、分页、多选及模拟部分失败交互；边界：同一原型目录；验收：显示数与选择数独立、模拟操作不调用真机接口。
- ⬜ D04 [验证/Verify][依赖: D03] 浏览器实际操作原型并保存桌面/窄屏与执行结果截图；边界：`prototypes/` 版本证据子目录与 `design.md`；验收：记录视口、实际交互、源码入口及未验证范围。
- ⬜ D05 [审阅/Review][依赖: D04] 将原型结论和用户反馈收敛为正式实施范围；边界：本版四件套；验收：默认台数、上限与交互无未决阻塞。

## 状态与契约先行

- ⬜ T01 [测试/Test][依赖: D05][可并行: T02,T03] 定义显示/选择状态断言；边界：拟新增 `frontend/tests/homePhoneView.test.mjs`；验收：0 台、页数回缩、非法偏好、筛选/翻页清选择、Mock 不建连的用例先失败。
- ⬜ T02 [测试/Test][依赖: D05][可并行: T01,T03] 定义批调度边界断言；边界：拟新增 `frontend/tests/deviceBatch.test.mjs`；验收：并发上限、去重、逐台失败、超时、取消、旧响应和资源清理用例先失败。
- ⬜ T03 [探查/Scout][依赖: D05][可并行: T01,T02] 再核对动作接口与返回模型；边界：`internal/automation/routes.go`、`types.go`、`service.go`、`api/openapi.json`；验收：冻结 `events` 的实际契约，不新增猜测字段或响应。

## 首页状态与手机视图

- ⬜ I01 [实现/Worker][依赖: T01] 实现显隐、每页台数与分页偏好；边界：拟新增 `frontend/src/composables/useHomePhoneView.ts`；验收：只计算本页范围，恢复偏好使用确切键与类型。
- ⬜ I02 [实现/Worker][依赖: I01] 实现本页选择模型；边界：拟新增 `frontend/src/composables/useDeviceSelection.ts`；验收：清选择规则与目标快照去重用例转绿。
- ⬜ I03 [实现/Worker][依赖: I01] 组合现有磁贴与分页为手机视图；边界：拟新增 `frontend/src/components/home/HomePhoneView.vue`、`frontend/src/utils/monitorGrid.ts`；验收：仅挂载当前页真实在线设备，显隐/尺寸/铺满可用。
- ⬜ I04 [实现/Worker][依赖: I03] 为磁贴增加独立选择区域与宿主事件；边界：`frontend/src/components/dashboard/DashTile.vue`；验收：选择不注入触控，原单机输入、播放器入口与全屏仍可用。
- ⬜ I05 [实现/Worker][依赖: I02,I04] 接入首页第三视图与唯一轮询；边界：`HomeView.vue`、`HomeControlCapsule.vue`、必要时 `stores/devices.ts`；验收：三种视图切换和小窗不退化，不引入第二轮询。

## 右侧批量操作

- ⬜ I06 [实现/Worker][依赖: T03][可并行: I01-I05] 封装既有单设备动作接口；边界：拟新增 `frontend/src/api/deviceActions.ts`；验收：严格按实际请求/响应与错误模型处理。
- ⬜ I07 [实现/Worker][依赖: T02,I06] 实现有界派发、目标冻结与单台结果；边界：拟新增 `frontend/src/composables/useDeviceBatch.ts`；验收：并发和部分失败用例转绿。
- ⬜ I08 [实现/Worker][依赖: I07] 实现整批/单台超时、停止、卸载和旧响应隔离；边界：同一批调度模块；验收：无无限等待，未知结果不标成功，全部边界用例转绿。
- ⬜ I09 [实现/Worker][依赖: I02,I08] 实现右侧批量面板的设备选择、返回/主页/最近任务与结果；边界：拟新增 `frontend/src/components/home/BatchOperationPanel.vue`；验收：空选中禁用动作，显示设置仅为一个分区。
- ⬜ I10 [实现/Worker][依赖: I05,I09] 首页接入面板收放与窄屏抽屉；边界：`HomeView.vue`、`BatchOperationPanel.vue`；验收：恢复入口可达、三种视图使用同一选择语义、结果不串批。

## 入口与术语迁移

- ⬜ I11 [实现/Worker][依赖: I10] 移除 Web 桌面与移动菜单的大屏监控入口；边界：`frontend/src/components/layout/AppHeader.vue`；验收：各断点入口均消失，AI/文件/设置等入口保留。
- ⬜ I12 [实现/Worker][依赖: I10][可并行: I11] 移除桌面原生菜单两处大屏入口；边界：`desktop/src/menu.rs`；验收：编译通过且对应平台菜单无遗留空项。
- ⬜ I13 [实现/Worker][依赖: D01,I11,I12] 更新正式术语并核查路由改动未越界；边界：写入 `docs/glossary.md`，只读核对 `routes.ts`、`DashboardView.vue`；验收：首页手机视图和批量面板指向实际实现，未擅自删除旧路由或加入兼容跳转。

## 验收与收口

- ⬜ V01 [验证/Verify][依赖: I13] 执行前端状态/调度/布局相关测试与生产构建；边界：`frontend/tests/` 与前端构建产物；验收：新用例、既有布局/电源用例和 `pnpm --dir frontend build` 通过。
- ⬜ V02 [验证/Verify][依赖: V01] 浏览器验证三种视图、台数/翻页、面板、筛选、显隐、全屏和加载失败；边界：真实运行首页及本版证据；验收：保存交互与截图证据，不用构建结果替代。
- ⬜ V03 [验证/Verify][依赖: V02] 两台及以上真机验证批量动作与单机独立控制；边界：所选测试设备；验收：逐台确认页面变化，未选中设备不受影响；设备不足则保持未完成。
- ⬜ V04 [验证/Verify][依赖: V03] 验证单台断线、执行超时、停止和离开页面；边界：批调度/投屏生命周期；验收：无悬挂触点/请求、旧响应覆盖、重试风暴或资源泄漏。
- ⬜ V05 [验证/Verify][依赖: V02][可并行: V03,V04] 逐级测实际设备规模与订阅回收；边界：测试环境的 4/9/16/25/36 档；验收：记录实际设备数和资源指标，不据 Mock 声称通过容量。
- ⬜ V06 [验证/Verify][依赖: I12] 验证桌面原生菜单、全屏、小窗与 macOS 标题栏；边界：桌面构建和实际运行；验收：平台分别记录，未运行平台保持未验证。
- ⬜ V07 [审阅/Review][依赖: V01-V06] 独立检查最终 diff、跨设备误操作风险、无障碍及模块边界；边界：本版改动文件；验收：阻塞问题关闭，文件/方法规模符合项目约束。
- ⬜ V08 [验证/Verify][依赖: V07] 运行项目现有完整校验并更新记录；边界：本版最终工作区；验收：Go、版本一致性、前端与受影响桌面检查通过，实测边界如实写入。
- ⬜ V09 [实现/Worker][依赖: V08] 在后续业务实施获授权并全部验收后按 lz-vlog 完成版本提交与合并；边界：本版本分支；验收：合并后复测、实际远端状态分别核实；本次纯规划不触发。

## 后续版本的启动条件

- ⬜ R33 [探查/Scout] v1.33 前置探查；边界：`internal/db/db.go`、`internal/device/manager.go`、`DashTile.vue` 与本版 `design.md` 路线图；验收：列明标签 Schema 与同步输入/旋转契约的缺口，后续另建该版细粒度任务。
- ⬜ R34 [探查/Scout] v1.34 前置探查；边界：`internal/files/handlers.go`、`internal/apps/handlers.go`、`frontend/src/scrcpy/useClipboardSync.ts` 与路线图；验收：明确分发映射、取消和成功去重需求，后续另建该版四件套。
- ⬜ R35 [探查/Scout] v1.35 前置探查；边界：`internal/automation/`、`internal/devicegate/`、`frontend/src/inspector/opLog.ts` 与路线图；验收：形成动作序列/执行租约/条件等待问题清单，不复活旧任务平台，后续另建该版四件套。
- ⬜ RC [探查/Scout] 候选能力分别研究；边界：本版 `design.md` 候选研究小节与独立实验记录；验收：通知、授权分享、非 ADB 接入分别写明网络/权限前提和可复现实验，未验证前不承诺交付日期。
