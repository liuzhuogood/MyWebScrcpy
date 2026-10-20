# v1.30.0-feature 版本说明

## 1. 新增与调整

1. **投屏拖拽文件交互与双弹窗分流**：
   - 投屏画面容器支持文件拖入，悬停时动态识别文件类型并展示高反差半透明高亮浮层；
   - **APK 文件**：松开后弹出模态安装确认对话框，展示包体大小与常用安装参数（`-r` 保留数据覆盖安装、`-g` 自动授予运行时权限、`-d` 允许降级安装）；提供上传与安装进度条反馈，针对降级冲突、存储不足、签名不符等 ADB 错误精准输出友好中文提示；
   - **其他类型文件**：松开后弹出保存确认对话框，提示“是否保存到设备的 Downloads 目录”，确认后流式推送到设备 `/storage/emulated/0/Download/`；
   - **公共架构封装**：拖拽感知与状态管理抽象为 `useDeviceFileDrop` Composable，高亮浮层与双对话框作为公共组件统一挂载于 `DevicePlayer.vue`，投屏详情页面（`PlayerView.vue`）、Web 浮动小窗（`FloatingPlayer.vue`）和桌面原生小窗自动具备完整交互能力。
2. **范围裁减**：
   - 按照明确指令，彻底移除应用管理/应用市场列表、分类与启停卸载功能，保持轻量纯粹。

## 2. 优化与保障

1. **临时文件安全清理**：
   - 上传的 APK 写入系统临时目录，在安装完成后立即通过 `defer os.Remove` 清理，避免服务端磁盘泄漏。
2. **错误码中文精准映射**：
   - 针对常见 ADB 错误码（`INSTALL_FAILED_ALREADY_EXISTS`, `INSTALL_FAILED_VERSION_DOWNGRADE`, `INSTALL_FAILED_INSUFFICIENT_STORAGE`, `INSTALL_FAILED_UPDATE_INCOMPATIBLE`, `INSTALL_FAILED_NO_MATCHING_ABIS`, `INSTALL_PARSE_FAILED_NOT_APK` 等）提供清晰易懂的中文说明。

## 3. 验证与注意事项

1. **自动化测试**：
   - 后端单元测试覆盖参数装配、ADB 结果解析、安装执行与 HTTP 接口（`go test -v ./internal/apps/...` 全部通过）；
   - 前端通过 `vue-tsc -b && vite build` 生产构建与 `proto:check` 类型检查。
2. **兼容性**：
   - 纯增量能力，不侵入投屏核心管道与 scrcpy 协议；
   - 支持多小窗并行拖放与安装中断取消。
