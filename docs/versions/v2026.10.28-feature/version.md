# v2026.10.28-feature 版本说明

## 1. 版本定位

- 本版为按北京时间日期的例行发布，与 `v2026.10.27` 代码内容一致，自该版以来无新增功能或修复提交。
- 版本号全量同步：`VERSION`、`frontend/package.json`、`desktop/package.json`、`desktop/tauri.conf.json`、`desktop/Cargo.toml`、`desktop/Cargo.lock`、`packages/python-sdk`（pyproject、`__init__.py`、`version.go`）统一为 `2026.10.28`。

## 2. 功能范围（沿用 v2026.10.27）

1. **投屏自动亮屏与空闲自动熄屏**：
   - 优化投屏启动与会话激活时的屏幕唤醒逻辑，确保连接时稳定唤醒。
   - 系统设置 → 通用设置 → 空闲自动熄屏，按设备引用计数追踪投屏、视频保活、ADB 终端和异步命令；连续空闲满 10 分钟后排入有界队列发送 `KEYCODE_SLEEP` 自动熄屏。
2. **首页实时手机视图与右侧批量操作面板体系**：
   - 甲板（deck）与紧凑（compact）之外的第三种视图（phone），与同档视图统一宽高、2px 间隔、等比黑边 contain 显示；整合全屏、显隐控制、尺寸切换与单机控制。
   - 右侧批量操作面板，预留位置不遮挡设备卡片；面板壳、编号显隐网格、批量勾选范围、动作按键、显示设置与逐台结果组件化。
3. **编号小方块显隐体系与批量调度**：
   - 按 serial 字典序稳定映射编号 01..N（仅真实设备），点选切换单台显隐，支持「全显」与「只显已选」。
   - 有界并发、逐台超时、旧响应隔离，仅对显式勾选的在线真实设备生效。
   - 「显示离线」开关并持久化。

## 3. 验证与交付

- 版本一致性：`node scripts/sync-version.mjs --check` 与 `node --test scripts/sync-version.test.mjs` 本地通过。
- 自动化测试：`internal/idlescreen`、`internal/dependencies` 单元测试与端到端接口校验通过。
- 发布链路：推送 tag 后由 `Build and Release` 工作流完成五平台 CLI、Windows 签名桌面包、macOS DMG、Release 资产与 Docker 镜像；Python SDK 由 `Publish Python SDK` 工作流发布至 PyPI。
