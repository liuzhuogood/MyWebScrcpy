# 项目工作流

工作流规范定义保存在本目录，特性的执行进展与交付物统一归档在 `docs/versions/<version>/`。由 `lz-workflow` 按文档执行。

默认流程：`requirements-prototype`。

| 名称 | 文件 | 用途 | 状态 |
| --- | --- | --- | --- |
| `requirements-prototype` | [requirements-prototype.md](requirements-prototype.md) | 收集需求、设计交互原型，将原型与设计资料收口合入主干 | 可执行 |
| `release` | [release.md](release.md) | 准备版本说明、精确提交合入主干、推送 tag 并核验 Release 与产物 | 可执行 |
| `feature-dev` | [feature-dev.md](feature-dev.md) | 全栈特性端到端开发（需求、后端、前端、门禁、文档与归档） | 可执行 |

启动示例：
- 需求原型：`用 lz-workflow 启动 requirements-prototype，任务是首页手机拟物设备卡片重构`
- 特性迭代：`用 lz-workflow 启动 feature-dev，任务是实现设备批量控制面板`
- 发布：`用 lz-workflow 启动 release，发布当前已验证版本`

继续示例：`用 lz-workflow 继续本次运行`。
