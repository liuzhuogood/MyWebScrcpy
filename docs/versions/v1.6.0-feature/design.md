# v1.6.0-feature 设计：全量 API 文档与 AI 技能提示词

## 背景

MyWebScrcpy 的接口按功能分散在设备管理、投屏 WebSocket、录像、ADB、按键、文件、脚本、UI XML 与 Vision 模块中。使用者无法从单一入口确认接口范围、参数、错误与实时消息协议，也无法稳定地让 AI 根据当前项目生成可维护的操作技能。

## 目标

1. 新增一个站内 API 接口说明页面，覆盖当前项目对外可调用的全部 HTTP 与 WebSocket 接口。
2. 页面正文的第一个内容区固定为可复制提示词，明确要求 AI 创建或更新命令名为 `my-scrcpy-use` 的技能。
3. 提示词和接口说明均以同一份机器可读 API 清单为依据，避免不同页面、README 与技能中的接口描述互相漂移。
4. 文档完整说明请求方法、路径、参数、请求/响应示例、状态码、设备副作用、鉴权/部署前提与 WebSocket 消息语义。

## 非目标

1. 不在本版本新增、删除或改变现有业务接口的行为。
2. 不把静态页面、嵌入资源、证书和内部 Go 函数误列为 API。
3. 不承诺技能自动执行重启、删除文件、ADB 指令等有副作用操作；技能必须先展示影响并取得用户确认。
4. 不接入外部在线文档服务，也不将项目接口定义发送给第三方。

## 页面与数据设计

新增 `/api-docs.html` 作为站内入口，并从首页提供“API 文档”导航。页面按下列顺序展示：

1. **AI 技能提示词**：首个内容区，提供“复制提示词”按钮和复制成功反馈。提示词随版本控制保存，不把内部开发说明写入普通界面文案。
2. **使用前提**：服务地址、HTTP/HTTPS、当前无内建身份认证的部署风险、设备 `serial` 的来源，以及会改变设备/文件的接口警告。
3. **接口目录与分组**：设备与控制、投屏/实时通讯、录像、ADB/按键、文件、脚本、UI 检查、Vision。
4. **接口详情**：每个接口均有方法、路径、输入、输出、错误、示例和副作用；WebSocket 另列连接参数、文本/二进制帧、会话生命周期和重连规则。

页面数据来自仓库内受版本控制的 OpenAPI 3.1 JSON/YAML 清单。实现时由 Go 服务以 `/api/openapi.json` 只读暴露该清单，`/api-docs.html` 使用项目现有原生 HTML/CSS/JavaScript 渲染；不引入运行时 CDN 依赖。接口变动须同步更新该清单，并用自动化校验比较已注册路由与清单，防止遗漏。

## 首段提示词

页面首段的可复制内容如下；实际实现时保持命令名、只读优先和副作用确认要求不变：

```text
请为 MyWebScrcpy 创建或更新一个名为 my-scrcpy-use 的技能。先读取本服务的 /api/openapi.json 和 API 文档页面，严格以当前接口定义为准，不要猜测不存在的路径、字段或响应。

技能应帮助用户安全地使用设备列表、投屏实时连接、旋转/屏幕状态/重启、录像、ADB 命令、按键、文件管理、脚本管理、UI XML 和 Vision 接口。为每项能力说明适用条件、必填参数、可复制请求示例、成功与错误响应及结果校验方式。

默认先执行只读操作。对重启设备、发送按键或 ADB 指令、开始或停止录像、上传/移动/重命名/删除文件、修改脚本等有副作用操作，先说明目标与影响，获得用户确认后才调用。不要输出或保存令牌、私钥、设备隐私内容或文件内容；不要把静态页面和内部实现当作 API。接口清单发生变化时，更新本技能并保留仍有效的行为说明。
```

## 接口覆盖基线

实现时的清单至少覆盖以下已注册接口，并以源码及测试核对具体 schema：

| 分组 | 接口 |
| --- | --- |
| 设备与控制 | `GET /api/devices`、`GET /api/rotate`、`GET /api/screen-state`、`POST /api/reboot` |
| 实时投屏 | `GET /ws`（WebSocket） |
| 录像 | `POST /api/recordings`、`GET /api/recordings`、`GET /api/recordings/{recording_id}`、`POST /api/recordings/{recording_id}/stop`、`GET /api/recordings/{recording_id}/download`、`GET /api/recordings/download`、`DELETE /api/recordings/{recording_id}` |
| ADB 与按键 | `POST /api/adb/commands`、`GET /api/adb/commands/{command_id}`、`POST /api/sendkey` |
| 文件 | `GET /api/files`、`GET /api/files/download`、`POST /api/files/upload`、`POST /api/files/folders`、`POST /api/files/move`、`POST /api/files/rename`、`POST /api/files/delete`、`POST /api/files/undo` |
| 脚本 | `GET/POST /api/scripts/categories`、`PATCH/DELETE /api/scripts/categories/{cat}`、`GET /api/scripts`、`GET /api/scripts/{cat}/{name}`、`POST /api/scripts/{cat}`、`PUT/DELETE /api/scripts/{cat}/{name}` |
| UI 检查 | `GET /api/ui/xml` |
| Vision | `GET /api/vision/stream`（WebSocket）、`GET /api/vision/results`、`GET /api/vision/stats`、`GET /api/vision/debug` |

## 关键决策

1. **OpenAPI 清单为文档事实来源。** 页面只是清单的展示层；路由或 schema 改动不能只修改页面文本。
2. **提示词放在页面第一个内容区。** 用户打开页面即可复制，不必先浏览完整接口目录。
3. **WebSocket 与 HTTP 同等纳入。** 投屏和 Vision 是项目关键接口，不能因其非 REST 形式而缺席。
4. **明确风险语义。** 删除、重启和任意 ADB 命令必须标识为有副作用，避免 AI 技能将其当作安全查询。
5. **接口范围以服务端注册路由为准。** 历史 README 示例、未注册处理函数和前端内部请求不得自动加入清单。

## 风险、迁移与回滚

- 风险：接口定义可能随代码演进而滞后；以路由覆盖测试阻断遗漏，并在每次接口改动的评审中同步清单。
- 风险：完整示例可能暴露真实设备标识或私有文件路径；文档一律使用占位符，页面不记录用户输入。
- 迁移：首次发布时补齐现有路由的 schema 和示例；后续新接口将“更新 OpenAPI 清单与文档校验”作为新增路由的固定步骤。
- 回滚：移除 API 文档页面入口及只读清单路由即可，不影响任何已有业务 API。

## 验收标准

1. 用户可从站内导航打开 API 文档，首个内容区可一键复制上述提示词。
2. API 清单覆盖所有已注册 HTTP 与 WebSocket 路由；路由覆盖校验对遗漏定义失败。
3. 每个接口展示真实方法、路径、参数、示例、响应/错误与副作用；示例不含真实敏感数据。
4. `my-scrcpy-use` 提示词能让 AI 以 `/api/openapi.json` 为准创建或更新技能，并要求副作用操作先确认。
5. 页面在离线或受限网络环境仍可使用，不依赖外部 CDN。
