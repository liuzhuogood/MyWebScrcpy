# 本项目预览配置

公网映射使用 `lz-plugin/lz-dev/lz-demo`：加载已安装的 `lz-dev` 技能，再读取其 `modules/lz-demo/MODULE.md`。SSH 创建、复用、验证、记录和停止流程由该模块维护，本文件只提供项目参数。

| 参数 | 已确认值 |
| --- | --- |
| 项目根目录 | 包含 `frontend/package.json` 和 `prototypes/` 的 MyWebScrcpy 仓库 |
| 开发预览命令 | 项目根目录运行 `pnpm --dir frontend proto` |
| 本机预览地址 | `http://127.0.0.1:5174`，每次核对实际监听；复用已确认的本项目实例 |
| SSH 目标 | `root@wskj.work`，使用已有免密认证 |
| 服务器绑定 | `127.0.0.1:15173`，固定端口与网站上游一致 |
| 网站反向代理 | 1Panel 网站 `demo.wskj.work` → `http://127.0.0.1:15173`，由用户维护 HTTPS 与 WebSocket 配置 |
| 公网原型入口 | `https://demo.wskj.work/#<原型目录名>` |
| 具体放行域名 | `demo.wskj.work`，配置在 `frontend/vite.prototype.config.ts` |
| 运行资料目录 | `prototypes/artifacts/remote-preview/<本次会话>/` |

原型继续使用本地模拟数据，不连接真实业务接口、设备或正式 Store。页面、主要交互、桌面与小屏、键盘和主题验收按项目原型技能执行。`proto:preview` 是构建预览，不提供开发热更新；需要修改即时同步时使用 `proto`。
