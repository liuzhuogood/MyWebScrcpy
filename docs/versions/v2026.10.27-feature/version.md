# v2026.10.27-feature 版本说明

## 1. 版本定位

- 本版包含新特性提交：修复投屏自动亮屏，并新增「空闲自动熄屏」功能。
- 版本号全量同步：`VERSION`、`frontend/package.json`、`desktop/package.json`、`desktop/tauri.conf.json`、`desktop/Cargo.toml`、`desktop/Cargo.lock`、`packages/python-sdk`（pyproject、`__init__.py`、`version.go`）统一为 `2026.10.27`。

## 2. 功能范围

1. **修复投屏自动亮屏**：优化投屏启动与会话激活时的屏幕唤醒逻辑，确保连接时稳定唤醒。
2. **新增空闲自动熄屏**：
   - **入口与持久化**：系统设置 → 通用设置 → 空闲自动熄屏。默认关闭，开启后立即生效并持久化至后端数据库。
   - **设备独立活动与计时**：按设备引用计数追踪投屏、视频保活、ADB 终端和异步命令；连续空闲满 10 分钟后排入有界队列发送 `KEYCODE_SLEEP`（223）自动熄屏，新操作或设备离线自动取消。
   - **接口与契约**：提供 `GET /api/system/idle-screen` 与 `PUT /api/system/idle-screen`，更新 `web/openapi.json` 接口定义。
3. **沿用既有核心能力**：
   - 首页实时手机视图（第三视图）与右侧批量操作面板体系。
   - 稳定编号小方块显隐体系与「全显 / 只显已选」。
   - 有界并发、逐台超时的批量调度体系与「显示离线」过滤持久化。

## 3. 验证与交付

- 版本一致性：`node scripts/sync-version.mjs --check` 与 `node --test scripts/sync-version.test.mjs` 本地通过。
- 自动化测试：`internal/idlescreen`、`internal/dependencies` 单元测试与端到端接口校验通过。
- 发布链路：推送 tag 后由 `Build and Release` 工作流完成五平台 CLI、Windows 签名桌面包、macOS DMG、Release 资产与 Docker 镜像；Python SDK 由 `Publish Python SDK` 工作流发布至 PyPI。
