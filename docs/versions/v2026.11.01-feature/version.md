# v2026.11.01-feature 版本说明（构建失败保留）

## 1. 版本定位与状态

- 本版为 2026-11-01 例行发布，因版本号使用 `2026.11.01`（patch 版本号包含前导零 `01`），违反 SemVer 2.0.0 规范，导致 Tauri (`tauri.conf.json > version must be a semver string`) 与 Cargo (`invalid leading zero in patch version number`) 解析失败，构建中断，未生成 Release。
- 按照仓库规范保留标签，后续由 `v2026.11.1` 完成规范化修复与全链路发布。
