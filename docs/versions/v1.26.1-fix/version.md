# v1.26.1-fix：首页添加网络设备 Toast 反馈与即时展示修复

状态：已修复并完成本地验证

### 待修复 Bug 列表

1. **首页添加网络设备反馈缺失**：
   - 现象：添加网络设备成功后对话框直接关闭，缺少结果反馈 Toast 提示；失败时缺少明确的全局错误提示。
   - 诉求：增加 Toast 提示（成功提示与失败原因反馈）。

2. **添加网络设备后未即时呈现（需刷新页面才出现）**：
   - 现象：添加网络设备后，设备卡片没有立即在设备列表中出现，有时需要手动刷新页面才能看见。
   - 根因分析：
     1. 后端 `AddManualDevice` 原同步调用 `m.refresh()` 执行 `adb devices -l`，与异步的 `m.Connect` 竞争，耗时较长导致返回延迟。
     2. `m.refresh()` 构建手动设备时默认将状态硬编码为 `offline`，忽略了刚添加时的 `connecting` 状态。
     3. 前端 `useDeviceStore` 的 3 秒静默轮询与添加设备后的 `load()` 存在网络竞态，旧请求响应可能覆盖新状态。
     4. 前端未在添加返回后立即进行本地乐观更新。
     5. 若用户此前切换到了“自动发现”分类过滤，新添加的手动设备会被分类过滤器隐藏，刷新页面重置为“全部”后才显示。

### 修复方案

1. **前端交互与 Toast 提示**：
   - `HomeView.vue` 引入 `vue-sonner` 的 `toast`，添加成功弹出 `toast.success('已添加网络设备')`，失败弹出 `toast.error`。
   - `DeviceCard.vue` 补充删除设备与重新连接的友好反馈。
2. **设备即时呈现与状态保护**：
   - `frontend/src/stores/devices.ts`：
     - `addDevice` 接口返回后立即乐观追加/合并入 `devices.value`，视图立即可见。
     - 若当前分类过滤为 `auto`，添加成功后自动切回 `all`。
     - `load()` 引入请求序列号检查，防止轮询旧响应覆盖最新设备列表。
     - `removeDevice` / `reconnect` 同样增加本地乐观状态更新。
3. **后端解耦与状态保护**：
   - `internal/device/manager.go`：
     - `AddManualDevice` 保存 DB 后立即原子更新内存中 `m.devices`（状态为 `connecting`），`m.refresh()` 改为异步后台执行，使添加接口毫秒级返回。
     - `m.refresh()` 手动设备遍历时保留数据库已有的 `connecting` 等有效状态，不硬编码覆盖为 `offline`。

### 本地验证

- [x] Go 模块全量单元测试与 `internal/device` 专项测试通过。
- [x] 前端 TypeScript 类型检查与 Vite 生产构建通过。
- [x] Node 回归测试脚本（`player_windows_test.cjs`、`roi_test.cjs`）通过。
- [x] 版本号全模块一致性与换行符测试通过（1.26.1）。
