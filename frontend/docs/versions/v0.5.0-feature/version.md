# v0.5.0-feature 版本说明

> 2026-10-09 状态核对：**已实现；验收记录待核实**。检查器历史联调已记录。tasks.md 勾选放大镜目视确认通过，但 session.md 与本文件仍写待确认；缺少明确确认过程，本次不据此断言已验收，保留待核实。

1. 投屏检查器可用（真机联调通过）：工具栏“检查”入口打开右侧面板，四 Tab（shadcn Tabs）。
2. 新增 `src/api/ui.ts` + `src/inspector/`（guard、opLog store、Panel、取色/XML/页面/日志四组件、XML 工具函数）；overlay 与 canvas 同胞定位免同步；输入拦截用显式 guard（不用监听器顺序）。
3. 取色：悬停读数（像素/归一化/HEX·RGB）+ 240px 放大镜（20×20 取源）+ 点击固定；检查期间操控不注入。
4. XML：快照获取、树渲染折叠、节点点选 ↔ 绿框联动、属性复选生成 XPath、测试/显示全部/清除。
5. 页面信息：一键复制 + 刷新；操作日志：操控记录（描述/ADB 双视图）+ 复制 + 清空（usePlayer 经 onOp 上报）。
6. 联调修：Tab 切不动（补 `@click` 显式同步）、放大镜黑屏（`:ref` 未绑定）、解码 canvas `willReadFrequently`。
7. 验证：`vue-tsc + build` 通过；放大镜画面待用户目视确认。
