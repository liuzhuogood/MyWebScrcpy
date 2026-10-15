# 系统配置

首页“系统配置”保留主题、ADB、Python 与模板匹配。Pi、模型提供商、模型地址、API Key、fd/rg 配置和内置脚本调度已移除。

ADB、Python 可选择应用管理的内置副本，或 PATH / 绝对路径指定的系统程序；错误路径直接报错。ADB 安装仍需接受 Android SDK 许可，来源变更需等待设备连接与操作空闲。Python 的“安装依赖”使用所选解释器安装同版本 SDK 和识图依赖。

当前 `dependencies/config.json` 的 schema 为 2，仅包含 `adb`、`python` 的 `source` 和 `path`。v1.25 遇到 schema 1 时直接重置选择，不迁移 Pi 或归档旧配置；新配置重启后保留。未知版本报错，不覆盖文件。全局 Pi、个人技能和用户业务脚本不参与清理。

任务上报只需服务地址与设备序列号，见 [任务运行集成](task-runs.md)。
