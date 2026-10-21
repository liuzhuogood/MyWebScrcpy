# v2026.10.22-feature 版本说明

## 1. 版本定位

- 本版为按北京时间日期的例行发布，与 `v2026.10.21` 代码内容一致，自该版以来无新增功能或修复提交。
- 版本号全量同步：`VERSION`、`frontend/package.json`、`desktop/package.json`、`desktop/tauri.conf.json`、`desktop/Cargo.toml`、`desktop/Cargo.lock`、`packages/python-sdk`（pyproject、`__init__.py`、`version.go`）统一为 `2026.10.22`。

## 2. 功能范围（沿用 v2026.10.21）

1. **设备列表首页全息空间甲板（Holo Spatial Deck）视觉**：磨砂玻璃拟态、极光渐变背景、3D 悬浮倾角与居中控制胶囊，统一入口文案为「大屏监控」。
2. **桌面端无级全透明穿透（0%~100%）与深空/澄澈双主题原生融合**，修复 macOS 浅色外观下黑暗模式泛白。
3. **macOS 原生沉浸式窗口**：`TitleBarStyle::Overlay` 红绿灯避让、紧凑顶栏与原生拖拽。
4. **投屏拖拽安装**：APK 拖入安装与文件保存至 Downloads 的双弹窗分流。
5. **Python SDK 3.9 兼容性修复**与版本同步发布。

## 3. 验证与交付

- 版本一致性：`node scripts/sync-version.mjs --check` 与 `node --test scripts/sync-version.test.mjs` 本地通过。
- 发布链路：推送 tag 后由 `Build and Release` 工作流完成五平台 CLI、Windows 签名桌面包、macOS DMG、Release 资产与 Docker 镜像；Python SDK 由 `Publish Python SDK` 工作流发布至 PyPI。
