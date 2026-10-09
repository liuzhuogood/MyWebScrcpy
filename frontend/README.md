# MyWebScrcpy 前端

Vue 3 + TypeScript + Vite，源码由 `MyWebScrcpyNew` 迁入。组件沿用 shadcn-vue。

## 开发

在仓库根目录执行：

```bash
pnpm --dir frontend install --frozen-lockfile
pnpm --dir frontend dev
```

浏览器打开 `http://localhost:5173/`。Go 后端需运行在 `http://localhost:8080`。
`/api`、`/ws` 和 API 文档均通过 Vite 代理，生产环境使用 Go 提供的同源接口。

## 构建

```bash
bash scripts/build.sh
```

产物位于 `web/dist/`，随 Go 程序内嵌，访问地址为 `http://localhost:8080/`。
`pnpm --dir frontend build` 只更新前端产物，需重新编译 Go 才能更新程序中的页面。

首页直接提供 Vue 界面，不再提供旧 HTML 页面或 `/app/` 入口。API 文档位于 `/api-docs.html`。
新界面尚未提供脚本管理页面，后端脚本 API 保留。
前端历史版本记录位于 `frontend/docs/versions/`。
