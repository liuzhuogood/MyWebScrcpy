# v1.20.1：系统配置、依赖拆包与 CLI/GUI 发布

正式 VERSION 为 v1.20.1。用户授权提交推送全部 Git 改动并创建同名附注标签；发布包由标签工作流生成，远程构建结果需单独核对。

1. 首页顶端系统配置提供系统/明亮/黑暗主题，依赖安装、卸载、来源和路径在同一页签。
2. scrcpy-server.jar 保持内嵌；Pi/fd/rg 从主程序和桌面 sidecar 移出，改为按需下载；ADB 支持官方平台包安装。
3. 系统 Pi/ADB 自动查找执行路径，Pi 验证帮助与 RPC，并自动识别自身工具缓存和 PATH。界面不要求单独配置 fd/rg。
4. 依赖副本可真实卸载，使用中拦截；来源配置原子保存，相关操作使用当前来源。
5. 按用户反馈删除额外主机令牌、跨站/地址限制和前端配置权限禁用。
6. 一并提交现有 v1.21.0-feature 设计文档，保持其待实施状态。

详细设计、原型和本机验证见 [v1.19.5-feature](../v1.19.5-feature/verification.md)。主程序 macOS ARM64 同参数构建减重约67%；完整桌面安装包及其他平台运行不由本机构建替代。

发布下载清单：

- `mywebscrcpy-linux-amd64-cli`
- `mywebscrcpy-linux-arm64-cli`
- `mywebscrcpy-macos-amd64-cli`
- `mywebscrcpy-macos-arm64-cli`
- `mywebscrcpy-windows-cli-amd64.exe`
- `mywebscrcpy-windows-gui-amd64.exe`
- `mywebscrcpy-v1.20.1-windows-x86_64-gui-setup.exe`
- `mywebscrcpy-v1.20.1-macos-amd64-gui.dmg`
- `mywebscrcpy-v1.20.1-macos-arm64-gui.dmg`

另保留用户确认的 `desktop-latest.json` 小型 GUI 自动更新索引，签名包含在索引中，不另外发布 `.sig`。Windows GUI exe 内嵌 Go 服务，可独立释放并启动；系统需具备 WebView2，安装包会处理缺失的 WebView2。CLI 默认服务模式，更新器优先精确匹配新 CLI 文件名，保留旧版本精确名称读取；Docker 使用新 Linux CLI。旧 CLI 若不能匹配新资产名需手动下载一次。按用户追加要求补充 macOS 两种 GUI DMG，拖到 Applications 安装，使用 DMG 手动更新；Linux GUI 包停止发布。

Windows GUI 安装版使用索引自动更新；独立 GUI exe 使用完整安装包更新，避免安装器更新后仍重新打开旧便携文件。
