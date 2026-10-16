---
name: my-scrcpy-use
description: MyWebScrcpy 使用的二级入口：查询和调用 API，按任务读取自动化脚本、任务执行、UI XML、安卓定位及任务上报规范。
---

# MyWebScrcpy 使用

## 规则

1. 从用户、当前会话或项目配置确认服务根地址；涉及设备时确认 `serial`，不猜地址、端口或设备。
2. 先读取 `{base_url}/api/openapi.json`，按当前定义确认方法、路径、参数、响应和认证，不猜字段或兼容接口。
3. API 查询与调用使用 `curl`，设置超时并检查 HTTP 状态与响应；设备操作后验证实际页面或业务结果。
4. 执行前按[环境规范](core/env.md)统一注册环境并创建临时执行目录，运行产物均保存在该目录内；其余 `core/` 规范按需读取，二级技能放在 `modules/<名称>/MODULE.md`。

## 规范

| 任务 | 参考文档 |
|---|---|
| 注册环境、创建执行目录 | [执行环境规范](core/env.md) |
| 编写、修复自动化脚本 | [自动化脚本规范](core/automation.md) |
| 执行自动化任务或业务技能 | [任务执行规范](core/execution.md) |
| 分析 UI XML、生成和验证 XPath | [UI XML 规范](core/ui-xml.md) |
| 查询或修改安卓模拟定位 | [安卓定位规范](core/location.md) |
| 上报任务状态、日志和结果 | [任务上报规范](core/task-reporting.md) |

`core/` 中引用的 SDK、辅助脚本和项目文档从当前项目目录读取，缺失时明确报告。
