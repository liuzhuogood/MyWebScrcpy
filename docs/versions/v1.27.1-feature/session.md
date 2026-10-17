# v1.27.1-feature 会话记录

## 会话信息

- **时间**：2026-10-17
- **分支**：`lz/v1.27.1-feature`
- **基线 Commit**：`c23788794343bee22e08c28d748495b129e5f51f`
- **工作区**：`/Users/liuzhuo/.codex/worktrees/v1271-task-state/MyWebScrcpy`（隔离 worktree）
- **实施授权**：仙人模式直接派发，执行全量已批准设计。

## 关键约束记录

1. 保持独立判断，严格按批准稿执行，执行前复述理解。
2. 保护主工作区 `/Users/liuzhuo/code/MyWebScrcpy`，不进入不改动。
3. 不提交、不推送、不合并、不打 tag、不发布、不触发部署，不修改全局配置及用户真实环境。
4. 明确授权删除旧任务注册/上报链路、`TaskReporter` 及旧任务表与数据；保留设备、系统配置、操作日志、模板和用户文件。
5. 任务目录严格按 `~/.mywebscrcppy/tasks/`，通用技能名称按 `my-scrcpy-common`。
6. 仅做用户主动扫描，无后台自动扫描/轮询；设备状态接口仅两个（总状态与总状态及决策），无 level。
7. AI 配置放置于现有系统设置服务区域，密钥服务端脱敏不回传明文不入日志。
8. 验证先做可交互原型，再接正式页面；全量 Go 测试、前端构建、原型构建与各场景测试必须真实通过。

## 探针闭环排查与修复记录

针对外部探针 `task-probe` 与 `state-probe` 复现的门禁缺陷，本轮完成针对性攻坚与闭环验证：

1. **同前缀逃逸与任务目录越界（task-probe）**：
   - 使用 `filepath.Rel` 替代简单的 `strings.HasPrefix`，彻底杜绝 `/root/probe` 与 `/root/probe-outside` 类似同前缀兄弟目录符号链接泄漏（HTTP 403 Forbidden，`leaked=false`）。
   - 在 `GetTaskDetail` 与 `Scan` 中强制校验任务目录自身的实际解析路径必须位于 `rootDir` 内部，阻止符号链接指向工作根外部（`read=false`，`ErrPathTraversal`）。

2. **产出空文件与假冒格式识别（task-probe）**：
   - 过滤目录产出与单文件产出中大小为 0 的异常文件，截图、视频、报告等非日志必需产出严禁用空文件伪装。
   - 引入文件头魔数识别（`isValidImageHeader`），检测 PNG (`\x89PNG\r\n\x1a\n`)、JPEG 等真实特征，伪造图片直接判定产出不完整并在读取时返回 403 Forbidden，拦截假冒媒体。

3. **FIFO 扫描阻塞与根目录切换竞争（task-probe）**：
   - 在读取任务元数据（`task.json` / `state.json`）前先执行 `os.Lstat` 与 `os.Stat` 校验 `Mode().IsRegular()`，并使用 `syscall.O_NONBLOCK` 与有界读取，彻底消除 FIFO 导致的读取阻塞。
   - 在 `Scan` 退出写入内存缓存时比对活跃根目录，若扫描期间用户切换了任务根，旧目录结果立即废弃，不污染新根索引。

4. **决策健壮性、密钥脱敏与 HTML 转义（state-probe）**：
   - `AIClient.Decide` 增加对 `c == nil || c.database == nil` 的防御性校验，数据库不可用时返回优雅降级结果不崩溃。
   - 引入 `sanitizeSecret` 脱敏函数，上游报错或网络异常信息中出现的配置 API 密钥统一替换为 `[REDACTED]`。
   - 针对空决策 JSON（`{}`）进行内容有效性校验，核心字段为空时显式返回 `Available: false`。
   - 服务端 HTML 模板引入 `safeURL` 辅助函数，防止 Go `html/template` 将 Base64 截图转义为 `#ZgotmplZ`。
   - `CollectState` 在截屏或上下文获取失败时明确将 `context_stable` 标记为 `false`，并填充包含 `serial` 与 `status` 的 `DeviceInfo`。

5. **配置与通用操作脚本闭环**：
   - 修复前端 `SystemConfigDialog.vue` 保存按钮在桌面端口受环境变量覆盖时不可用的问题，允许独立持久化服务配置。
   - 后端配置写入接口增加 `verifySameOrigin` CSRF 防御。
   - 修复 `open_app.py`、`restart_app.py` 中 `request_client` 路径包含查询参数导致报错的问题，采用 `query=[("serial", serial)]` 标准调用。
   - `watch_video.py` 引入单调时钟（`time.monotonic()`）与参数边界校验；`try_return.py` 严格限定仅在包名/Activity 变更时判定返回成功，剔除视频字幕等纯 OCR 文本波动的误判。

## 2026-10-18 合并授权

用户明确要求将当前代码合入 main 并清理本次工作树。此授权取代此前实施阶段不提交、不合并的限制；未要求推送、打 tag 或发布。主目录原有两份设计文档删除保留。版本整体三项未通过验收的问题继续按 acceptance.md 如实记录。
