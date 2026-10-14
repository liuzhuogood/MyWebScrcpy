# Markdown 生成 Python

先在系统配置的「依赖」中准备 Pi、搜索程序及 Python，并配置模型。Python 可使用系统解释器或下载内置解释器，点击 Python 卡片的「安装依赖」安装与主程序版本一致的 SDK 及 HTTPX。也可在选定环境中手动安装：

```bash
/path/to/python -m pip install 'mywebscrcpy-sdk==1.20.1'
```

在任务工作台保存 Markdown 和目标设备，再点击「生成 Python」。Pi 根据 markdown-to-python 技能与冻结 API/SDK 资料生成代码；活动区显示思考、工具和结果，支持停止。信息不足时按提示补全文档再生成。

当前支持明确的基础 ADB 动作、页面尺寸、等待与有限判断/循环。点击坐标、包名、Activity 等必须明确；Vision、模板匹配、OCR 和中文输入尚未提供。SDK 的专用 tap/swipe/key/text 接口也尚未提供，基础动作使用 device.adb 的明确参数。

只有通过校验并保存后才能运行。编辑文档或相关上下文会让旧代码过期，旧源码保留以便查看；手动与定时执行均需先重新生成。生成失败或停止不会清除最后成功产物，不自动回退旧生成器。

## 外部运行

安装 SDK 后，把生成代码复制成 Python 文件：

```bash
python -m pip install mywebscrcpy-sdk
python script.py DEVICE_SERIAL --base-url http://HOST:PORT
```

可使用 SDK wheel 安装，无需项目源码目录或应用任务入口。远程服务与设备仍须可达。地址也可由 MYWEBSCRCPY_API_URL 提供；可选连接令牌从 --token 或 MYWEBSCRCPY_API_TOKEN 提供，不写入源码。应用内任务自动注入连接信息，直接使用所选 Python 环境中安装的 SDK，不再释放 SDK 源码。

SDK 使用说明见 [Python SDK](../packages/python-sdk/README.md)，基础动作例子见 [device_query.py](../packages/python-sdk/examples/device_query.py)。

## 生成 API

`POST /api/md-scripts/{id}/compile` 现在返回 SSE（text/event-stream），路径沿用已有入口。每条 data 是 JSON：status、thinking_*、tool_* 与错误事件沿用助手形式；最终 done 包含已保存 script 和 generation 来源信息。预检失败为 HTTP JSON 错误，流中失败为 error 事件。旧 JSON 编译响应已随用户确认的生成器替换移除。

单次生成最多五分钟、两次校验修正；同任务重复生成拒绝，不同任务独立。请求取消会回收 Pi。资料字段和方法按明确契约读取，不从模型文本中猜代码块或字段别名。

脚本记录的 needs_generation 表示产物缺失、无来源或上下文/代码哈希不符。来源记录含 source_hash、code_hash、provider、model、contract_version、generated_at。模型字段用于追溯；来源指纹包含实际 SDK、校验器、API 文档和技能。

校验不执行生成源码，不构成进程沙箱，也不能替代真机验收。第一阶段的受控模型、受控设备接口和本机软件验证见 [版本记录](versions/v1.21.0-feature/session.md)。
