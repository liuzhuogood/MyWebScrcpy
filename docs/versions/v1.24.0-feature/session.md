# v1.24.0-feature 会话记录：GUI 小窗透明度与鼠标穿透

## 分支与基线

- 主分支：`main`
- 版本分支：`lz/v1.24.0-feature`
- 起点 commit：`93a2c8e`
- 现场说明：主分支工作区存在前置未提交的 macOS DMG 指引相关修改（`scripts/build-desktop.mjs`、`scripts/verification/macos-dmg.mjs`、`desktop/macos/`），按版本工作流与 Git 规则保持现场不丢弃、不自动 stash/clean。已基于 `main` 基线建立版本分支 `lz/v1.24.0-feature`。

## 会话

### 1. 用户

> Side Question: 还有评估GUI功能是否可行：
> 1.设置小窗透明度
> 2. 设置小窗鼠标穿透
> ...
> 这个请搞个版本来实现， 因为我觉得不一定能实现 /lz-vlog

### 1. Antigravity

精简记录回应与独立可行性评估：
- **评估结论**：两项功能在 Tauri v2 原生桌面框架下均具备可行性，但鼠标穿透存在“操作死锁”关键风险，必须配套非鼠标交互的恢复机制（全局快捷键或托盘/主窗口操作）。
- **实施准备**：在 Tauri 桌面端与前端之间设计透明度与穿透控制链路。

### 2. 用户

> 先不直接实现， 只建版本， 你把改动的还原一下

### 2. Antigravity

- **还原改动**：立即检出并还原探索验证期间在 `desktop/src/links.rs` 中引入的临时 API 测试代码，恢复工作区纯净，保留主分支原有前置未提交文件。
- **只建版本**：按 `lz-vlog` 工作流规范建立 `v1.24.0-feature` 版本，建立分支 `lz/v1.24.0-feature`，沉淀完整的四件套文档（`session.md`、`design.md`、`tasks.md`、`version.md`）以及总览更新，不提前进行业务实现与代码提交。
