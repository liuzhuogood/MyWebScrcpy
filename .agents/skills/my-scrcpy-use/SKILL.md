---
name: my-scrcpy-use
description: 通过当前 OpenAPI 查询并调用 MyWebScrcpy API；统一处理服务地址、HTTP 请求、Token 配置和证书校验，不封装设备或其他业务功能。
---

# MyWebScrcpy API 使用

使用本技能的 [通用请求客户端](scripts/request_client.py) 发送 HTTP 请求。仅封装请求，不新增业务方法，不保存静态接口列表或响应模型。

## 查询当前接口

1. 使用用户提供的服务根地址；未提供时核对当前会话或项目配置，不猜地址和端口。地址可通过 `--base-url` 或环境变量 `MYWEBSCRCPY_API_URL` 配置。
2. 先读取 `/api/openapi.json` 与 `/api-docs.html`。在项目根目录可执行：

   ```bash
   python3 .agents/skills/my-scrcpy-use/scripts/request_client.py \
     --base-url http://127.0.0.1:1989 /api/openapi.json --output /tmp/my-scrcpy-openapi.json
   python3 .agents/skills/my-scrcpy-use/scripts/request_client.py \
     --base-url http://127.0.0.1:1989 /api-docs.html --output /tmp/my-scrcpy-api-docs.html
   ```

   上述地址来自创建技能时的会话；后续使用以已核实的服务地址替换。
3. 从规范的 `paths` 查看路径、方法、摘要和标签，按任务筛选。例如列出已下载规范中的接口：

   ```bash
   python3 - <<'PY'
   import json
   with open('/tmp/my-scrcpy-openapi.json') as source:
       spec = json.load(source)
   for path, item in spec['paths'].items():
       for method, operation in item.items():
           if method in {'get', 'post', 'put', 'patch', 'delete', 'head', 'options', 'trace'}:
               print(method.upper(), path, operation.get('summary', ''))
   PY
   ```

   读取命中操作的参数、请求体、响应状态和 schema，并沿 `$ref` 读取对应定义。接口目录页从同一规范加载，页面可能过滤部分标签，以 JSON 规范为准。
4. 严格按当前定义构造请求并解析实际响应，不猜路径、字段、响应层级或认证方式。定义缺失、与实测不符或响应缺字段时报告具体差异，停止依赖该假设的调用。不要用旧规范作为失败兜底。
5. 用户授权范围内默认先做只读查询；会改变设备或服务状态的操作先说明影响，尚未授权时取得确认。通用命令接口也按实际命令判断影响。异步任务按规范提供的状态接口查询，不自动重发原始请求。

## 通用请求与配置

`python3 .agents/skills/my-scrcpy-use/scripts/request_client.py --help` 查看参数。根据刚读取的定义设置 `--method`、接口路径及重复的 `--query 名称=值`。请求体用 `--json-file 文件` 或 `--body-file 文件 --content-type 类型`，文件为 `-` 时从标准输入读取。额外请求头用 `--header 名称=值`。这些参数不对应任何业务功能。

- 标准输出为原始响应体，JSON、HTML、文本和二进制都不转换；`--output` 将响应体写入文件。HTTP 非 2xx 返回非零退出码，并保留错误响应体；诊断信息写入标准错误。
- `--timeout` 设置请求超时秒数，默认 30 秒。客户端不自动重试、不跟随重定向，避免重复执行或将认证信息发送到其他地址。
- Python 调用方也可导入脚本里的 `ApiClient`，使用唯一的 `request(method, path, query=..., body=..., headers=...)` 方法，返回状态码、响应头和原始字节。调用方自行按已确认的 schema 解析。

## Token 与证书

认证配置集中在请求客户端，不散落到每个调用中：

| 环境变量 | 用途 |
|---|---|
| `MYWEBSCRCPY_API_URL` | 服务根地址 |
| `MYWEBSCRCPY_API_TOKEN` | Token 值，未配置时不注入认证请求头 |
| `MYWEBSCRCPY_API_TOKEN_HEADER` | 服务明确要求的请求头名称；配置 Token 时必填 |
| `MYWEBSCRCPY_API_TOKEN_PREFIX` | 服务明确要求的前缀；默认空，非空时与 Token 之间加一个空格 |

创建技能时当前规范没有声明认证方案。未来启用 Token 后，先核对当前 `security` / `components.securitySchemes` 或用户明确提供的认证约定，再设置请求头名称与前缀；不默认假设 `Authorization`、`Bearer` 或 `api-token`。Token 仅从环境读取，不写入技能、请求文件、命令参数或日志。若环境已配置 Token 而头名称未知，先确定认证约定，不通过清空 Token 绕过认证。

默认校验 TLS 证书。只有用户明确授权且网络可信时，才为本次调用加 `--insecure`；该参数在客户端关闭证书校验，等价于 curl 的 `-k/--insecure`。遇到证书错误不要自动降级。客户端直接连接配置的服务，不读取环境代理。
