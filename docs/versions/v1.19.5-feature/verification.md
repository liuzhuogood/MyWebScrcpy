# v1.19.5-feature 验证记录

日期：2026-10-13。第二阶段已获得“继续实现吧”授权。以下为正式源码验证；后文保留第一阶段独立原型的历史证据。

## 正式实现验证

| 检查 | 结果与范围 |
| --- | --- |
| 前端 | `pnpm --dir frontend build` 类型检查/构建通过 |
| 后端 | `go test ./...` 通过 |
| 并发 | dependencies / updater / 主程序 race 检查通过 |
| 官方 Pi | 缓存的原始官方锁定归档，真实哈希/解包/版本/帮助/RPC；卸载、重新安装及使用中拦截通过 |
| 系统 Pi | PATH 查找成功；独立目录 fd/rg 自动找到；Pi 自有 bin 缓存自动找到；明确错误路径不回退 |
| 官方 ADB | 锁定 macOS 36.0.2 归档真实安装，版本检查、运行时来源解析与实际卸载通过；未调用真实设备 |
| 下载/解包 | 截断续传、哈希错误、穿越/符号链接/重复文件拒绝、系统文件保护、跨进程锁通过 |
| 保存/更新 | 保存原子写失败保持旧运行配置；成功后重载一致；更新冻结修改，配置备份/恢复创建父目录通过 |
| API | `dependency-service.mjs`：缺依赖启动、桌面既有内部控制、SDK 接受门槛、严格请求结构、错误配置不写入、安装取消与受管理清理通过 |
| 桌面生命周期 | `desktop-service.mjs`：ready、私有控制、旧路径、更新防护、SPA、正常关闭通过 |
| 桌面编译 | `SDKROOT=/Library/Developer/CommandLineTools/SDKs/MacOSX26.5.sdk node scripts/build-desktop.mjs --check --skip-frontend` 通过；默认 SDK27 链接报 TAPI 问题，明确使用已安装 SDK26.5 完成检查 |
| 五目标编译 | CGO=0，Windows AMD64、Linux AMD64/ARM64、macOS AMD64/ARM64 交叉编译通过；不代替目标原生运行 |
| 外置 Pi 回归 | `MYWEBSCRCPY_PI_TEST_PAYLOAD="$PWD/.cache/pi-test" go test -count=1 ./internal/aiprovider ./internal/pibundle` 通过，测试真实 RPC/助手工具/技能，不使用生产内嵌包 |
| 发布契约 | release-manifest 检查及 sync-version 两测试通过；发布前同步 VERSION 为 1.20.1 |
| diff | `git diff --check` 通过 |

真实归档测试命令：

```sh
MYWEBSCRCPY_PI_ARCHIVE_CACHE="$PWD/.cache/pi" \
MYWEBSCRCPY_ADB_ARCHIVE=/tmp/mywebscrcpy-adb-lock/darwin.zip \
go test -count=1 -race ./internal/dependencies ./internal/updater .
```

ADB 的 Darwin/Linux/Windows 36.0.2 原始 ZIP 已下载校验并锁定哈希；只有 macOS ARM64 程序在本机真实执行。许可门槛测试不替用户接受 SDK 协议。

## 正式浏览器验证

本地 `http://127.0.0.1:61013/`，独立临时数据目录；显式缺失 ADB 路径阻止真实设备调用。

- 真实 Pi 官方下载任务展示进度/取消入口，安装后显示 0.86.1 与卸载；后台安装文件与哈希检查通过。
- 系统来源切换触发自动检测；只提供 pi 路径，无“搜索工具”明细及 fd/rg 输入框。单元测试以真实 Pi 检查 PATH 与 Pi 自有工具缓存；浏览器先验证隔离 PATH 缺失提示，再继承本机 PATH，路径留空自动找到 `/Users/liuzhuo/.nvm/versions/node/v24.18.0/bin/pi`，实际帮助/RPC/搜索工具检查通过，显示“可用 · 0.86.1”。
- ADB 安装确认包含官方 SDK 链接、未选中的接受框；未接受时按钮禁用，取消返回，不替用户接受协议。
- 卸载确认可取消，取消后仍为已安装；实际文件删除/重装在隔离临时目录自动化测试完成。
- 三主题、持久选择与自动模式检查通过；CDP 模拟系统配色偏好 dark/light 后，系统按钮保持选中且 HTML 明暗切换，恢复模拟。未改变或实际切换 macOS 系统设置。
- 桌面 1280×800、小屏 390×844 实拍；小屏 document clientWidth/scrollWidth 均390，无横向溢出。滚动内容、底部关闭/保存、入口均可访问。
- 删除额外的“主机配置令牌”表单和解锁功能，进一步移除服务 RemoteAddr/Host/Origin/Sec-Fetch-Site 限制及前端权限禁用；不同来源请求进入实际检测，类型检查与桌面既有控制回归通过。
- 正式运行服务使用不同 Host/Origin、Sec-Fetch-Site=cross-site 验证：检测成功200，保存/安装/卸载/取消请求进入相应业务流程而非401/403；GET 返回 configurable=true。证据：[request-source-checks.json](request-source-checks.json)。
- 当前浏览器控制台 error/warn 捕获为空。
- 截图：`implementation-desktop.png`、`implementation-mobile.png`、`implementation-system-pi.png`、`implementation-uninstall-confirm.png`、`implementation-no-token.png`。

预览进程元数据保存在 `/tmp/mywebscrcpy-system-config-session.json`；仅本机回环监听，不接入原型公网隧道。

## 同参数体积

使用 macOS ARM64、`go build -trimpath -ldflags='-s -w'`，前后大小为 50,185,858 → 16,505,458 bytes，减少 33,680,400 bytes（67.11%）；SHA-256 见 [build-sizes.json](build-sizes.json)。减重主要来自移除约32 MiB 的 Pi 运行压缩包；Jar 继续内嵌。测试资源独立存在 `.cache/pi-test/`，不属于生产嵌入资源。此比例仅对应主程序，未测完整 DMG/MSI 安装包压缩比例。

## 尚未验收

未运行 Windows/Linux 原生测试、远程 CI、正式桌面安装包、Docker 容器或真实手机完整回归。Docker daemon 本机不可用，未启动或改变它。本轮用户明确要求 VERSION/tag 为 v1.20.1，所有 Git 改动提交推送，包含 v1.21.0-feature 设计记录。发布产物与远程 CI 结果单独验收。开发分支为 `codex/v1.19.5-system-config`，最终 Git 交付以分支与 tag 远端核对为准。

---

# 第一阶段：独立原型验证

日期：2026-10-13。范围：独立 Vue 原型；不运行后端依赖安装、主机 PATH 检测、实际文件校验或设备操作。

## 检查

- `pnpm --dir frontend proto:check`：通过。
- `pnpm --dir frontend proto:build`：通过，产物仅在 `prototypes/dist/`。
- 最终方案：`prototypes/cases/system-config-v2/`，复用 v1 的纯模拟状态模块。三页签 v1 保留供对比。
- 安装/卸载/PATH/路径检查均为内存模拟。增加卸载后，v2 默认模拟 Pi/ADB 内置副本已安装，可直接查看卸载入口。Pi 卸载后的首次下载故意失败以演示重试；再次安装可成功。手动路径仅演示绝对路径及文件名校验，不能证明真实程序存在或可运行。
- ADB 模拟系统路径 `/opt/homebrew/bin/adb`；Pi 模拟 PATH 缺失；版本/大小是设计占位与当前资源参考，不是已完成下载。

## 公网预览

- [合并方案 v2](https://percent-window-types-server.trycloudflare.com/#system-config-v2)
- [三页签方案 v1](https://percent-window-types-server.trycloudflare.com/#system-config-v1)
- 本地：`http://127.0.0.1:61012/#system-config-v2`。
- 启动构建预览，使用本次独立 `preview.config.mts` 仅放行上述具体公网域名。主服务与正式 Vite 配置均未改动；不终止已有 5174/5175 服务。
- PID、PGID、命令、进程身份和日志记录在 `prototypes/artifacts/remote-preview/20261013-system-config/session.json`。
- 电脑、预览与隧道需要保持运行；Quick Tunnel 是临时地址。

停止本次预览与隧道（脚本先检查进程身份，保留其他会话服务）：

```sh
bash /Users/liuzhuo/code/MyWebScrcpy/prototypes/artifacts/remote-preview/20261013-system-config/stop.sh
```

## 浏览器实测

全部通过最终公网地址操作，视口为桌面 1280×800、小屏 390×844。

| 项目 | 观察结果 |
| --- | --- |
| 顶部入口 | 桌面图标+文字，小屏图标；Enter 打开弹窗 |
| 两页签 | 仅主题/依赖；ArrowRight 切换；旧三页签 v1 保留 |
| Jar | 固定随应用内置，无安装按钮及来源选择 |
| 主题 | 系统/明亮/黑暗可选，明暗实拍；OS 实际主题变化事件未实测 |
| 安装 | 显示下载进度、校验与完成，期间锁来源和保存 |
| 取消/重试 | ADB 取消后回到未安装；Pi 首次模拟中断，重试成功 |
| 关闭重开 | 安装继续，状态保留；未保存的来源草稿恢复已保存值 |
| 来源独立 | ADB 安装完成后仍能使用系统来源，不因安装而自动保存内置来源 |
| 缺依赖 | Pi 内置未安装时保存被拦截，说明对应依赖 |
| 系统检测 | ADB 模拟自动路径可用；Pi 模拟 PATH 缺失 |
| 路径表单 | 相对路径报错；绝对 pi 路径模拟通过后可保存；重开保持已保存路径 |
| 键盘退出 | Escape 关闭并返回顶部配置入口 |
| 小屏 | 根宽度和 scrollWidth 均 390px，无横向溢出；双系统来源内容可滚动，页签和底部操作可见 |
| 资源/请求 | 最终一轮捕获 17 项响应，0 加载失败，0 正式 API/WS 请求；控制台无警告/错误 |

网络证据：`prototypes/artifacts/remote-preview/20261013-system-config/browser-network-v2.json`。截图保存在本版本 `prototypes/`，最终方案文件名带 `v2`；[设计](design.md) 已嵌入主要状态。

持续可用性复查：启动工具退出后超过 10 秒，再次核对本次 preview/tunnel 进程身份，均匹配；本地页面、公网页面和公网 CSS 均返回 200 且 Content-Type 正确。最终检查保存在 `session.json` 的 `final_checks`。`git diff --check` 通过；只增加原型、版本文档和目录索引，没有修改正式前后端或发布配置。

## 卸载增量验证

- 最终原型重新执行 `proto:build`，包含类型检查与构建，通过。
- 1280×800、390×844 实拍：Pi 内置来源显示卸载，ADB 系统来源显示应用副本卸载；Jar 没有卸载。
- Pi 卸载确认可取消，取消后仍为已安装；确认后恢复未安装与安装按钮，来源仍为内置，保存被缺依赖检查拦截。
- Pi 卸载后重装，模拟首次下载失败，重试后恢复已安装与卸载入口。开始重装会清除先前缺依赖错误，避免安装完成后显示旧提示。
- ADB 当前为系统来源，卸载确认说明系统依赖仍可使用；完成后应用副本状态行消失，系统来源、检测路径及配置仍保留，保存成功，关闭重开保持一致。
- 删除期间来源与保存操作锁定；实测看到“内置副本卸载中…”状态。
- 截图：`uninstall-v2-desktop.png`、`uninstall-v2-mobile.png`、`uninstall-confirm-v2-desktop.png`、`uninstall-confirm-v2-mobile.png`、`uninstalled-v2-desktop.png`、`uninstalled-system-v2-desktop.png`。
- 主机文件实际删除、使用中拦截与删除残留失败均属于后续正式实施，未在此模拟原型中验证。

## 边界

只验证原型交互和模拟状态。尚未实现或验证后端安装、真实 PATH 和执行能力、持久配置、ADB 权限、依赖平台资源、主程序拆包、多平台运行、最终产物减重及发布。原型刷新会还原状态；不会在主机安装依赖或操作设备。
