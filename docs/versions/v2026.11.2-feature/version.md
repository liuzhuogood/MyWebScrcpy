# v2026.11.2-feature 版本说明

## 1. 版本定位

- 本版为按北京时间日期的例行发布，包含 `v2026.11.1` 的全量能力与发布加固。
- 引入手机卡片视图对齐紧密网格与遥测黑屏感知，以及设备网格虚拟滚动能力。
- 版本号全量同步：`VERSION`、`frontend/package.json`、`desktop/package.json`、`desktop/tauri.conf.json`、`desktop/Cargo.toml`、`desktop/Cargo.lock`、`packages/python-sdk`（pyproject、`__init__.py`、`version.go`）统一为 `2026.11.2`。

## 2. 功能范围

1. **手机卡片视图对齐紧密网格与遥测黑屏感知**：
   - 手机卡片与投屏视图统一：`HomePhoneView` 支持 `viewMode='deck'|'phone'`，共用紧密设备墙规格（小/中/大/铺满同宽同高与 2px 间隙体系），视图切换卡片尺寸与排版 1:1 无缝重合零断层。
   - 免投屏视口与状态感知：顶部状态栏仅展示连接方式、温度（电池+CPU）与电量百分比；屏幕中央感知黑屏休眠与锁屏（显示已锁定，隐藏后台应用），亮屏正常展示前台运行应用专属彩色图标与中文名称。
   - 后端遥测与锁屏判定：`dumpsys power` 与 `dumpsys window policy` 精准解析 `screen_on` 与 `screen_locked`，完善 Device 契约与内存缓存同步。
2. **设备网格虚拟滚动与前台应用遥测感知**：
   - 虚拟滚动支撑：引入 `@tanstack/vue-virtual`，新增 `useDeviceGridVirtual` 与 `DeviceGridItem`，甲板与紧凑视图实现行窗口化渲染，支撑海量设备流畅滚动。
   - 遥测感知升级：后端通过 `dumpsys window` 解析设备前台焦点应用与常见应用图标映射，`dumpsys thermalservice` 优先提取 CPU 核心温度。
   - 卡片操作聚焦：卡片实时展示当前前台应用与专属彩色图标，纯净化状态标签，主操作聚焦画中画与恢复。
3. **投屏自动亮屏与空闲自动熄屏**（沿用加固）：
   - 优化投屏启动与会话激活时的屏幕唤醒逻辑，确保连接时稳定唤醒。
   - 系统设置 → 通用设置 → 空闲自动熄屏，按设备引用计数追踪投屏、视频保活、ADB 终端和异步命令；连续空闲满 10 分钟后排入有界队列发送 `KEYCODE_SLEEP` 自动熄屏。
4. **编号小方块显隐体系与批量调度**（沿用加固）：
   - 按 serial 字典序稳定映射编号 01..N（仅真实设备），点选切换单台显隐，支持「全显」与「只显已选」。
   - 有界并发、逐台超时、旧响应隔离，仅对显式勾选的在线真实设备生效。
   - 「显示离线」开关并持久化。
5. **CI/CD 与环境兼容性加固**：
   - 严格遵循 SemVer 规范，不含前导零（`2026.11.2`）。
   - Python SDK 发布工作流依赖与版本校验优化，支持 PEP 440 与 SemVer 版本兼容探测。

## 3. 验证与交付

- 版本一致性：`node scripts/sync-version.mjs --check` 与 `node --test scripts/sync-version.test.mjs` 本地通过。
- 自动化测试：`go test ./...` 单元测试通过，`pnpm --dir frontend build` 构建通过。
- 发布链路：推送 tag 后由 `Build and Release` 工作流完成五平台 CLI、Windows 签名桌面包、macOS DMG、Release 资产与 Docker 镜像；Python SDK 由 `Publish Python SDK` 工作流发布至 PyPI。
