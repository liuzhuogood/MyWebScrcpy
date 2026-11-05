# 项目发布工作流（release）

状态：可执行。入口：`用 lz-workflow 启动 release，发布当前已验证版本`。

## 范围与前置条件

读取根 `AGENTS.md`、`VERSION`、待发布版本记录和实际 diff，核对授权、目标提交及最新远端主干。只提交本次交付必需文件，保护其他改动；不使用强推、重指已发布 tag 或隐式丢弃工作区内容。功能实施和验证完成后才进入发布；规划、原型及未覆盖验收不得写成已交付结果。

正式版本取 `VERSION`，使用纯 SemVer，日历版本不补零。同日补发沿用仓库已有递增后缀约定，例如 `v2026.11.5-1`；必须先检查同名 tag/Release，不覆盖已有版本。新 feature 规划的 `v1.35.0-feature` 等记录编号不代表本次正式版本。

## 固定执行顺序

1. **准备非空说明。** 优先使用本次正式版本精确匹配的 `docs/versions/<tag>-fix/version.md` 或 `<tag>-feature/version.md`，仅保留一个说明来源，核对其事实、测试和限制。存在文件却为空、版本不匹配或仅有标题时直接失败，不偷偷改用未来规划文档。
2. **缺少说明时据实生成。** `scripts/prepare-release-notes.mjs` 从目标提交可达的最近已发布 tag 读取真实提交和 diff，按新增、修复、其他变更整理中文说明，并明确测试/部署状态不可从提交推断。不只贴 hash，不编造效果。无法确认发布基线时停止并核对；`--base-tag` 只能传已核实的发布 tag。
3. **提交前核对。** 先执行版本同步和该改动必需的检查，再生成待审说明。逐路径暂存、检查 staged diff，提交本次更改；不得先打 tag 后补提交。
4. **正常合入主干。** 刷新远端后整合最新 `main`，保护他人提交与当前无关工作。核对合并结果并运行受影响检查；若当前分支含本次范围外的历史整理，明确保留主线原文件，不把无关删除带入发布。
5. **确认正确提交再打 tag。** 从合入主干后的目标提交重新生成说明并核对版本一致；tag 必须指向该提交。推送主干与明确 tag；若远端非快进，停止推送并重新整合，不强推。
6. **创建或更新 Release。** tag 推送触发 `.github/workflows/release.yml`：从 tag 提交读取说明，再发布 9 个程序与 Windows 更新索引。`softprops/action-gh-release` 使用 `body_path`，同 tag 已存在时幂等更新说明和产物，不重建 tag。人工更新用 `gh release edit <tag> --notes-file <file>`，保留真实换行，禁止把多行正文插入 shell 命令字符串。
7. **核验到终态。** CI 读取 Release 校验非空说明与来源内容一致、tag 和提交匹配；操作人继续监测所需构建、macOS DMG、Windows、Release、Docker 及关联 SDK 发布到成功或失败终态。最后读取远端 tag 对应提交、Release 正文和资产，确认实际版本；不能以触发 Actions 或退出码作为发布成功证据。

## 项目命令

提交前准备（`<tag>` 以实际 `VERSION` 为准）：

```bash
node scripts/sync-version.mjs --check
node --test scripts/prepare-release-notes.test.mjs scripts/sync-version.test.mjs
node scripts/prepare-release-notes.mjs <tag> --working-tree --output .cache/verification/release-notes.md
```

合入主干后，从准确提交重新准备：

```bash
node scripts/prepare-release-notes.mjs <tag> --target HEAD --output .cache/verification/release-notes.md
```

文档不存在时脚本通过 `gh release list` 核对已发布基线，因此需已配置 GitHub 凭据且有完整相关 tag 历史。CI checkout 使用完整历史并传入现有 `GITHUB_TOKEN`，不把凭据写进项目。

## 发布说明与交付

说明主体来自实际发布版本的 `version.md`；包括已实现变更、真实通过的检查、未验证的设备/平台场景，以及独立规划的排除范围。发布失败时保留事实和日志入口，不标记已发布。交付报告给出主干提交、tag、Release、Actions 和版本文档链接，区分本机运行、远端产物和实际部署。
