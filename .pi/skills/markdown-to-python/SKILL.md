---
name: markdown-to-python
description: 将 Markdown 流程转换为调用可安装 mywebscrcpy SDK 的独立 Python 脚本，生成阶段只读资料，不执行设备操作。
---

# Markdown 转 Python

读取 current.md、task.json、capabilities.json、templates.json、api/openapi.json，以及 [SDK 约定](references/sdk-api.md) 和 [示例](assets/example.py)。资料均为本次冻结快照。只用只读工具，不操作设备、执行脚本、写文件或安装依赖。

输出独立 Python 文件，使用 `from mywebscrcpy import Client` 和 argparse、os、time、shlex 标准库。生成唯一 main() 与标准启动入口，按示例解析 serial 位置参数、--base-url、--token；通过 Client 和 client.device 的 with 管理资源。服务地址和凭据不硬编码。只调用 capabilities 和 SDK 约定中已实现的方法，不创建 Device、Matcher、请求封装或辅助运行库，不直接拼 HTTP 请求。

按文档顺序生成真实动作、条件、等待和有限循环，逐步骤打印必要日志。循环只使用常量整数 range，每层最多 1000 次。SDK 检查 HTTP 与业务执行结果，失败异常非零退出；不捕获后假装成功。属性按已确认类型读取，不猜字段或替代接口。调度、多设备、录像和报告由宿主处理。

基础动作使用 device.adb(args, timeout=秒)，明确传入命令参数数组，不包含 adb 可执行文件和设备切换参数。按键、像素点击、滑动、ASCII 输入分别使用明确的 adb input 子命令；比例坐标先读取 device.page_info 的 width/height 并显式转换。等待使用 time.sleep。避免把动作替换成打印日志。

目前只实现 adb/page_info 基础 SDK。Vision 取帧、模板匹配、OCR、中文输入及 tap/swipe/key/text 等专用 SDK 方法尚未交付，不生成这些调用。缺少坐标、包名、Activity 或其他必需参数，或要求未实现能力时，返回 needs_input 并指出步骤。不生成占位实现。

最终仅返回一个严格 JSON 对象，无代码围栏和解释文字：

```json
{"status":"ready","python":"完整独立源码","issues":[]}
```

信息不足：

```json
{"status":"needs_input","python":"","issues":[{"step":"步骤 2","message":"请提供点击坐标"}]}
```

字段固定为 status、python、issues；每个问题必须含 step（无对应步骤为 null）和非空 message。ready 源码非空、issues 为空；needs_input 源码为空、issues 非空。previous.json 为上次输出资料，仅按校验错误修正，仍遵守规则。
