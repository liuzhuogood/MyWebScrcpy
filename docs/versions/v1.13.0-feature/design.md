# v1.13.0-feature 自动更新机制设计说明

## 1. 需求与目标
在 GitHub 发布新 Release tag 时，前端能够及时感知，通知用户有新版本可用，并支持用户在页面上一键触发自动下载与更新，显示实时下载进度条与速度，更新完成后引导重启或自动刷新。

## 2. 后端设计 (`internal/updater/`)

### 2.1 模块结构
- `internal/updater/checker.go`:
  - 检查 GitHub Releases API (`https://api.github.com/repos/liuzhuogood/MyWebScrcpy/releases/latest`)。
  - 读取当前内置版本号（嵌入的 `VERSION` 文件或全局变量）。
  - 对比 SemVer 版本，返回是否有更新 `has_update`、最新版本号 `latest_version`、更新说明 `release_notes`、发布时间 `published_at`、目标系统适配的文件名及直链等。
  - 支持可选 GitHub 镜像加速（如用户所处网络限制，自动或备用代理）。
- `internal/updater/downloader.go`:
  - 基于当前 `runtime.GOOS` / `runtime.GOARCH`，匹配目标二进制资产名称（如 `mywebscrcpy-darwin-arm64`）。
  - 下载文件并计算实时下载进度、字节数与速率（KB/s）。
  - 校验 GitHub Release 中的 SHA256（Release assets 中带有 digest 或 sha256 校验码）。
- `internal/updater/applier.go`:
  - 原地二进制替换逻辑：
    - 获取当前运行进程的二进制路径 (`os.Executable()`)。
    - 将现有运行文件重命名为 `.old`（Windows / Linux / macOS 通用安全策略）。
    - 将下载好的新可执行文件移动到原二进制路径，并在 Unix 上执行 `chmod 0755`。
    - 触发重启或通知完成（若支持原地 re-exec 则重启自身，否则返回状态等待服务管理器重启）。
- `internal/updater/handlers.go`:
  - `GET /api/version/check`: 查询当前版本与最新版本状态。
  - `POST /api/version/upgrade`: 触发更新下载流程。
  - `GET /api/version/upgrade-stream`: SSE 流，推送 `status`（`downloading` -> `verifying` -> `applying` -> `success` / `error`）、`percent`、`downloaded`、`total`、`speed`、`error`。

## 3. 前端设计 (`frontend/`)

### 3.1 状态管理与 API (`frontend/src/api/updater.ts`, `frontend/src/stores/updater.ts`)
- Pinia Store: `useUpdaterStore`
  - 记录 `currentVersion`、`latestVersion`、`hasUpdate`、`releaseNotes`、`downloadProgress`、`upgradeStatus`。
  - 定时或页面加载时检查更新。

### 3.2 UI 交互 (`frontend/src/components/updater/`)
- `UpdateNoticeDialog.vue` (基于 shadcn-vue `Dialog`, `Progress`, `Button`, `Badge`):
  - 弹窗展示新版本号、发布日期、Release 说明。
  - 点击「立即更新」后切换到进度视图：平滑进度条、下载百分比、已下载 / 总大小、速率展示。
  - 下载完成并应用后显示“更新成功！服务正在重启...”，并自动轮询 ping 服务，探测到新版本就绪后自动刷新页面。
- `AppHeader.vue`:
  - 顶栏显示当前版本号（如 `v1.12.0`）。
  - 若检测到新版本，显示闪烁徽标或「新版本」高亮提示按钮，点击打开更新弹窗。

## 4. 接口契约

### `GET /api/version/check`
Response:
```json
{
  "current_version": "v1.12.0",
  "latest_version": "v1.13.0",
  "has_update": true,
  "release_notes": "# 更新说明...",
  "published_at": "2026-10-10T10:00:00Z",
  "asset_name": "mywebscrcpy-darwin-arm64",
  "asset_size": 10499858
}
```

### `POST /api/version/upgrade`
Response:
```json
{
  "status": "started",
  "message": "Update process started"
}
```

### `GET /api/version/upgrade-stream` (SSE)
```
event: progress
data: {"status":"downloading","percent":45.2,"downloaded":4746200,"total":10499858,"speed":"2.5 MB/s"}

event: progress
data: {"status":"verifying","percent":100,"downloaded":10499858,"total":10499858,"speed":"0 KB/s"}

event: progress
data: {"status":"applying","percent":100,"downloaded":10499858,"total":10499858,"speed":"0 KB/s"}

event: completed
data: {"status":"success","message":"Update completed successfully, restarting..."}
```
