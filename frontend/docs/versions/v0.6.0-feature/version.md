# v0.6.0-feature 版本说明

> 2026-10-09 状态核对：**已被后续版本替代；前端待同步**。历史管理、请求和错误显示验证保留。主项目 v1.11 已删除 matches/click 等匹配接口，当前不能称模板匹配可用；Vue 尚存对应调用、控件及轮询，需要同步为仅模板管理。

1. 历史模板面板实施（匹配能力现已由主项目 v1.11 移除）：投屏工具栏模板入口，管理弹窗含列表、全部/设备/全局筛选、上传单张、导入/导出 ZIP、启用开关、阈值步进、设备↔全局切换、追加参考图、下载原图、删除、定位点击、实时命中查看。
2. 新增 `src/api/templates.ts` + `src/template/`（useTemplates、useTemplateMatches、TemplatePanel）；命中绿框 overlay 轮询绘制，开关持久化沿用老 `show-green-boxes` key。
3. 联调修：删除静默失败（AlertDialogAction 关弹窗先于用户 @click，删除 id 单独存放）；阈值改步进器。
4. 验证：开关/作用域/步进/删除 E2E 通过，定位点击链路通（后端 screencap 受限属后端环境问题）；`vue-tsc + build` 通过；测试模板已清，存量 `chreome` 恢复原状。
5. 选区存为模板延到 v0.7（依赖截图选区）。
