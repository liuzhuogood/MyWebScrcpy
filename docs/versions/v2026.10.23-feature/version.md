# v2026.10.23-feature 版本说明

## 1. 版本定位

- 本版合入 `lz/v1.32.0-feature` 分支：首页新增实时手机视图与右侧批量操作面板体系（v1.32 规划的主体实施内容）。
- 版本号全量同步：`VERSION`、`frontend/package.json`、`desktop/package.json`、`desktop/tauri.conf.json`、`desktop/Cargo.toml`、`desktop/Cargo.lock`、`packages/python-sdk`（pyproject、`__init__.py`、`version.go`）统一为 `2026.10.23`。

## 2. 功能范围（来自 v1.32.0-feature）

1. **首页实时手机视图**：在甲板（deck）与紧凑（compact）之外新增第三种视图（phone），与同档视图统一宽高、2px 间隔、等比黑边 contain 显示，黑边不响应触控；整合全屏、显隐控制、尺寸切换（小/中/大/铺满）与单机控制。
2. **右侧批量操作面板**：预留位置不遮挡设备卡片，窄屏自然流式堆叠；组件化拆分为面板壳、编号显隐网格、批量勾选范围、动作按键、显示设置与逐台结果六部分。
3. **编号小方块显隐体系**：按 serial 字典序稳定映射编号 01..N（仅真实设备，不伪造台数），点选切换单台显隐，支持「全显」与「只显已选」，取消旧分页。
4. **批量调度**：有界并发、逐台超时、旧响应隔离，仅对显式勾选的在线真实设备生效。
5. **默认仅显示在线设备**：控制胶囊新增「显示离线」开关并持久化；移除 Web 与桌面原生菜单的「大屏监控」独立入口，旧 `/dashboard` 路由原样保留。

## 3. 验证与交付

- 版本一致性：`node scripts/sync-version.mjs --check` 与 `node --test scripts/sync-version.test.mjs` 本地通过。
- 前端：新增 `deviceBatch`、`homeFilter`、`homePhoneView`、`phoneGrid` 单元测试；生产构建与类型检查在功能分支已完成，本版发布前回归通过。
- 后端：`go test ./...` 发布前本地回归通过。
- 验证边界：完整横屏/批量真机与桌面原生运行场景待主管真机验收，发布说明如实记录，不影响本版交付。
- 发布链路：推送 tag 后由 `Build and Release` 工作流完成五平台 CLI、Windows 签名桌面包、macOS DMG、Release 资产与 Docker 镜像；Python SDK 由 `Publish Python SDK` 工作流发布至 PyPI。
