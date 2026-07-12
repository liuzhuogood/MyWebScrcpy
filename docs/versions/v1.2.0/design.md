# v1.2.0 设计文档

## 背景

MyWebScrcpy 目前的能力聚焦在设备投屏与控制（`index.html` 设备列表、`player.html` 单设备播放器、`dashboard.html` 大屏）。后端是 Go 单体程序（`main.go`），通过标准库 `net/http` 暴露若干 `/api/*` 接口并内嵌 `web/` 静态资源；项目无数据库，少量用户态数据（如设备别名）仅存在浏览器 localStorage。

用户希望新增「脚本管理」能力：把可复用的 Python 脚本按分类组织起来，支持在 Web 界面新增、编辑、删除脚本。本期为后续两个方向铺路——**脚本执行**（在服务端用 python3 跑脚本，可向设备发 adb 指令）和 **AI 生成脚本**。

## 目标

1. **分类管理**
   - 支持新建、重命名、删除分类。
   - 分类下可挂多个脚本。

2. **脚本管理（CRUD）**
   - 在分类下新建脚本（默认 Python）。
   - 编辑脚本内容（在线代码编辑器）。
   - 重命名、删除脚本。
   - 查看脚本列表（按分类分组）。

3. **为后续版本预留**
   - 元数据结构预留「参数」「描述」，方便后续执行时传参、AI 生成时携带上下文。
   - 脚本以独立 `.py` 文件落地，后续可直接 `python3 xxx.py` 执行。

## 非目标

- 不实现脚本执行（不在服务端运行 python3，不捕获输出）。
- 不接入 AI 生成。
- 不做多用户/权限/鉴权（沿用当前无鉴权模型）。
- 不做脚本版本历史/差异对比（可后续用 Git 或自带历史解决）。
- 不改变现有 `index.html` / `player.html` / `dashboard.html` 的功能。

## 关键决策

### 1. 存储：文件式（已与用户确认）

```
<工作目录>/scripts/
├── 启动配置/                       # 分类 = 子目录，目录名即分类名
│   ├── 启动抖音.py
│   └── 打开微信.py
└── 设备检查/
    ├── 读取电池.py
    └── 查询存储.py
```

- **分类**：`scripts/` 下的子目录。分类名直接用目录名（中文允许）。新建分类 = 建子目录；重命名 = 改目录名；删除 = 删目录（连同其下脚本）。
- **脚本**：分类目录下的一个 `.py` 文件。脚本名 = 文件名（去掉 `.py`）。
- **工作目录**：默认为可执行文件同级的 `./scripts/`；支持用环境变量 `SCRIPTS_DIR` 覆盖，便于容器/部署场景外挂。
- 启动时若 `scripts/` 不存在，自动创建，并写入一个 README 说明和一个默认分类「示例分类」含一个示例脚本，避免空页面。

### 2. 元数据：脚本头部结构化注释

每个 `.py` 文件头部约定一段 YAML-like 注释块，承载元数据，避免引入 sidecar 文件：

```python
# -*- coding: utf-8 -*-
# @name 启动抖音
# @description 通过 adb 拉起抖音包名
# @params serial=设备序列号; mode=fast|slow
# @created 2026-07-12T10:00:00
# @updated 2026-07-12T10:05:00

import subprocess
# ...脚本正文
```

- 解析规则：只读文件开头连续的 `# @key value` 行，遇到第一个非 `#` 开头的行停止。
- 缺失字段给默认值；`@created`/`@updated` 由后端在新建/保存时自动写入，前端不直接编辑。
- 新建脚本时由后端生成模板，包含 `import` 骨架和 `def main():` 入口，便于后续执行和 AI 接入。

### 3. 后端 API（Go，注册到 `main.go` 的 `mux`）

统一前缀 `/api/scripts`，REST 风格：

| 方法 | 路径 | 说明 |
|------|------|------|
| GET    | `/api/scripts/categories`            | 列出所有分类（子目录名） |
| POST   | `/api/scripts/categories`            | 新建分类（body: `{name}`） |
| PATCH  | `/api/scripts/categories/{cat}`      | 重命名分类（body: `{name}`） |
| DELETE | `/api/scripts/categories/{cat}`      | 删除分类（含其下脚本） |
| GET    | `/api/scripts?category={cat}`        | 列出该分类下所有脚本（文件名 + 元数据） |
| GET    | `/api/scripts/{cat}/{name}`          | 读取脚本完整内容（含元数据） |
| POST   | `/api/scripts/{cat}`                 | 新建脚本（body: `{name, description, params, content}`） |
| PUT    | `/api/scripts/{cat}/{name}`          | 全量更新脚本（内容 + 可改 name/description/params，改名时移动文件） |
| DELETE | `/api/scripts/{cat}/{name}`          | 删除脚本 |

- 路径参数做严格白名单校验：分类名/脚本名只允许中文、字母、数字、`_`、`-`，禁止 `..` / `/`，杜绝目录穿越。
- 所有响应 `Content-Type: application/json`；写操作成功返回 `{ok:true}` 或新资源。
- 放在新的 `internal/scripts` 包，保持 `main.go` 干净。

### 4. 前端

- 新增页面 `web/scripts.html`，在 `index.html` 头部增加「脚本管理」入口按钮（与「大屏模式」并列）。
- 左右两栏布局：左侧分类列表（可新建/重命名/删除），右侧当前分类下的脚本列表 + 选中后进入编辑器。
- 在线编辑器：**CodeMirror 6 via CDN**（Python 语法高亮、行号、基础缩进）。体积可控、无构建步骤、与本项目「原生 HTML/JS、无打包」风格一致。若 CDN 不可用，降级为 `<textarea>`。
- 脚本元数据（描述、参数）在编辑器上方用小表单编辑，内容变更触发「保存」按钮高亮；保存调用 PUT 接口。

### 5. 前后端目录归属

- 脚本内容落盘到**运行时工作目录**的 `scripts/`（非内嵌 `web/`），因为是用户数据。
- 前端 `web/scripts.html` + `web/js/scripts.js` + 复用 `web/css/style.css`。

## 风险与权衡

### 风险

1. **目录穿越 / 任意文件读写**：路径参数若不校验，可读到 `scripts/` 之外。
   - 缓解：严格的分类名/脚本名白名单正则；拒绝包含路径分隔符或 `..` 的输入；后端最终路径再做一次「是否仍在 scripts/ 根目录内」的 `filepath.Rel` 校验。
2. **并发写**：两个浏览器同时编辑同一脚本会互相覆盖。
   - 缓解：本版不做乐观锁，保存时带 `updated` 做轻量 If-Match 校验（不一致则 409，前端提示刷新）。可选，若时间紧可推迟。
3. **删除分类连删脚本**：误删风险。
   - 缓解：前端删除分类做二次确认，后端 DELETE 默认要求带 `confirm=true`。
4. **CodeMirror CDN 依赖**：离线/内网环境加载不到。
   - 缓解：`<script onerror>` 降级到 textarea；后续可把 CDN 资源下载到 `web/vendor/` 内嵌。

### 权衡

- 选文件式而非 SQLite：牺牲了「按元数据复杂查询」的能力，换来零依赖、可 Git、可直接执行、对 AI 生成更友好（模型直接产 .py 文件）。
- 编辑器选 CodeMirror 6 而非 Monaco：Monaco 体积大、依赖 AMD 打包，不符合本项目无构建的风格。

## 迁移 / 回滚

- 全部新增，不改动现有代码逻辑（仅在 `main.go` 注册新路由 + `index.html` 加按钮）。
- 回滚：删除新增路由、新增页面、新增 `internal/scripts` 包；运行时 `scripts/` 目录是用户数据，回滚不影响已投屏功能，目录可保留或手动删除。

## 待确认问题

1. 在线编辑器是否接受 CodeMirror 6 via CDN？（默认：是，离线降级 textarea。）
2. 并发覆盖保护（`updated` If-Match）本版是否要做？（默认：做轻量版，时间不够可推迟到下版。）
