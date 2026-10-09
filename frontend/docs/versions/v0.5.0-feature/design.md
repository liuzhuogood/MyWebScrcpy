# v0.5.0-feature 设计文档

## 背景

- 投屏链路已稳；检查器是投屏页右侧面板：取色、XML/XPath 联动绿框、页面信息、操作日志。
- 接口：`GET /api/ui/xml`（xml + display_size + captured_at）、`GET /api/ui/page-info`（包/Activity/窗口/分辨率/DPI/方向）。

## 目标与非目标

**目标：**

- 投屏工具栏加“检查”入口，右侧面板四 Tab（shadcn Tabs）。
- 取色坐标：悬停读数（像素 + 归一化 + HEX/RGB）+ 13×13 放大镜 + 点击固定；检查期间操控不注入设备。
- XML 树：快照获取、树渲染、节点点选 ↔ 投屏绿框联动、属性复选生成 XPath、测试/显示全部/清除。
- 页面信息：一键复制 + 刷新；操作日志：记录操控（描述/adb 双视图）、复制、清空。

**非目标：**

- 面板拖拽调宽（固定宽度，窄屏堆叠；老拖拽逻辑不移植）。
- 大屏页检查器（老版本也没有）。

## 关键决策

### 1. 输入拦截不用监听器顺序，用显式 guard

老代码靠 capture 监听器先行，不可靠且难测。新版 `inspectorGuard`：面板打开且取色 Tab → 点击只固定十字线；XML Tab 点中绿框 → 只选中节点。`usePlayer` 在 mousedown/touchstart 开头查 guard，命中直接返回。

### 2. overlay 与 canvas 同胞绝对定位，免手动同步

overlay canvas 放在 canvas 包裹层内 `absolute inset-0 w-full h-full`，backing 尺寸随视频尺寸设置；CSS 缩放自动对齐，无需老 `syncOverlay` 的 getBoundingClientRect 数学。

### 3. 操作日志是独立 store，usePlayer 经 onOp 回调写入

`opLog` 单例（上限 500 条）；usePlayer 新增 `onOp` 选项，鼠标/触摸/键盘/工具栏动作处上报；DashTile 不接（老版本亦无）。

## 风险与权衡

- [XPath 注入] 用户输入 XPath 仅用于本地 `document.evaluate`，try/catch 包裹，无注入风险。
- [getImageData] 跨域污染不适用（同源 canvas），可直接取色。

## 迁移计划

1. `src/api/ui.ts` + `src/inspector/`（guard、opLog、Panel + 4 Tab 组件）。
2. `usePlayer` 加 `onOp` + guard 检查；PlayerView 加检查按钮 + 右侧面板布局。
3. 真机联调：取色读数 → XML 获取/点选联动 → XPath 测试 → 页面信息 → 操作日志有记录。

## 回滚计划

1. 检查按钮隐藏即回滚；新增 `src/inspector/` 独立目录。

## 待确认问题

- 无。
