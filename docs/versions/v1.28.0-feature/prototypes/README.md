# OCR 并行选项验证

2026-10-18，工作区 `/Users/liuzhuo/.codex/worktrees/ocr-consensus-v1-28/MyWebScrcpy`，分支 `lz/v1.28.0-feature`。本节记录浏览器验收时点，最终 Git 状态见版本会话；未部署。

## 正式改动

- `frontend/src/api/ocr.ts`：响应增加 `samples?: number`、`engines_used?: string[]`。
- `frontend/src/inspector/OcrTab.vue`：既有 Select 增加 `parallel=并行`，结果来源 `parallel` 显示“并行”。
- `frontend/src/components/system/OcrServicePanel.vue`：既有默认引擎按钮组增加“并行”。
- 未增加时长输入或技术文案，保留自动与单引擎含义。

## 本地预览与隔离

| 用途 | 地址 | 本次监听 PID | 启动命令 |
|---|---|---|---|
| 最小原型 | http://127.0.0.1:5175/#ocr-parallel-v128 | 12874 | `pnpm --dir frontend proto --port 5175` |
| 真实 OcrTab + fixture | http://127.0.0.1:5176/ | 17518 | `node prototypes/cases/ocr-parallel-v128/serve-fixture.mjs` |

PID 仅对应本次运行，重启后须重新核对。停止本次预览可分别终止上述已核实的 PID。未操作主工作区服务，未创建公网发布或隧道。

原型源码为 `prototypes/cases/ocr-parallel-v128/{meta.ts,Prototype.vue}`。专用 fixture 为同目录 `serve-fixture.mjs` 和 `verification/{index.html,main.ts,Fixture.vue}`；它直接挂载真实 `OcrTab.vue` 和 `provideInspector`，使用本地 canvas，没有设备画面。专用 middleware 仅模拟 `POST /api/vision/ocr`，其余 `/api/` 返回 404，不转发真实后端。

fixture 未注册正式路由，未改正式入口、Vite 配置或主题，也未被正式前端入口导入。正式构建产物定向输出至已忽略的 `prototypes/cases/ocr-parallel-v128/artifacts/frontend-build/`，未写入正式发布目录。

## 浏览器证据

浏览器真实点击正式 Select 的“并行”，选择后点击“开始识别”，专用服务实际收到 `[OCR fixture] engine=parallel mark=false`，页面显示“并行 · 48 ms · 1 处”。再通过键盘 Home / Return 切回自动，旧结果清空；重新识别收到 `engine=auto`，页面显示 `Indra OCR`。返回的 `samples`、`engines_used` 未额外显示。

| 截图 | 视口 / 主题 / 状态 |
|---|---|
| [正式组件并行](ocr-parallel-v128-formal-select.jpg) | 1280×720，浅色，真实组件 Select 与结果来源均为并行 |
| [正式组件小屏并行](ocr-parallel-v128-formal-select-mobile.jpg) | 390×844，浅色，真实组件并行结果 |
| [正式组件自动回归](ocr-parallel-v128-formal-auto-mobile.jpg) | 390×844，浅色，自动选择、Indra OCR 来源 |
| [原型桌面浅色](ocr-parallel-v128-desktop-light.jpg) | 1280×800，并行选择、默认引擎并行 |
| [原型小屏浅色](ocr-parallel-v128-mobile-light.jpg) | 390×844，并行选择、默认引擎并行 |
| [原型小屏深色](ocr-parallel-v128-mobile-dark.jpg) | 390×844，并行选择、默认引擎并行 |
| [原型桌面深色自动](ocr-parallel-v128-desktop-dark-auto.jpg) | 1280×800，恢复自动 |

桌面及 390px 视口的 DOM `scrollWidth` 等于 `clientWidth`。原型与正式 fixture 页控制台均未发现 warn/error。原型结果、fixture 接口响应均为模拟数据，截图不证明真实 OCR 融合效果。

## 已完成检查

- `pnpm --dir frontend proto:build`（含 `proto:check`）通过。
- `pnpm --dir frontend build --outDir ../prototypes/cases/ocr-parallel-v128/artifacts/frontend-build` 通过；Vite 提示既有大 chunk 警告。
- `node frontend/src/inspector/roi_test.cjs` 通过，包括错误与旧结果处理回归。
- `node frontend/src/composables/player_windows_test.cjs` 通过。
- `node --test frontend/tests/screenPower.test.mjs`：17/17 通过。
- `node --check prototypes/cases/ocr-parallel-v128/serve-fixture.mjs` 和本次文件 `git diff --check` 通过。

未验证真实设备、真实 OCR 服务融合、真实系统配置持久化；系统配置按钮组的交互与主题、小屏在本地原型中验证。未触碰其他 Agent 的 Go、SDK 或版本记录文件。
