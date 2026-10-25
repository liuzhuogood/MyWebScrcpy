# v2026.10.26-feature 版本说明

## 1. 版本定位

- 本版为按北京时间日期的例行发布，与 `v2026.10.25` 代码内容一致，自该版以来无新增功能或修复提交。
- 版本号全量同步：`VERSION`、`frontend/package.json`、`desktop/package.json`、`desktop/tauri.conf.json`、`desktop/Cargo.toml`、`desktop/Cargo.lock`、`packages/python-sdk`（pyproject、`__init__.py`、`version.go`）统一为 `2026.10.26`。

## 2. 功能范围（沿用 v2026.10.25）

1. **首页实时手机视图**：甲板（deck）与紧凑（compact）之外的第三种视图（phone），与同档视图统一宽高、2px 间隔、等比黑边 contain 显示；整合全屏、显隐控制、尺寸切换与单机控制。
2. **右侧批量操作面板**：预留位置不遮挡设备卡片，窄屏自然流式堆叠；面板壳、编号显隐网格、批量勾选范围、动作按键、显示设置与逐台结果六部分组件化。
3. **编号小方块显隐体系**：按 serial 字典序稳定映射编号 01..N（仅真实设备），点选切换单台显隐，支持「全显」与「只显已选」。
4. **批量调度**：有界并发、逐台超时、旧响应隔离，仅对显式勾选的在线真实设备生效。
5. **默认仅显示在线设备**：「显示离线」开关并持久化；移除独立「大屏监控」入口，旧 `/dashboard` 路由原样保留。

## 3. 验证与交付

- 版本一致性：`node scripts/sync-version.mjs --check` 与 `node --test scripts/sync-version.test.mjs` 本地通过。
- 发布链路：推送 tag 后由 `Build and Release` 工作流完成五平台 CLI、Windows 签名桌面包、macOS DMG、Release 资产与 Docker 镜像；Python SDK 由 `Publish Python SDK` 工作流发布至 PyPI。
