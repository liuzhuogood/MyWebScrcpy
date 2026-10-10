# Vue 原型

在项目根目录运行：

```sh
pnpm --dir frontend proto
```

打开 <http://127.0.0.1:5174/>。每个原型可以用地址中的 `#<目录名>` 直接访问，例如 `http://127.0.0.1:5174/#task-center-v1`。

复用 `frontend/node_modules`，没有安装依赖时先运行 `pnpm --dir frontend install --frozen-lockfile`。预览使用独立 Vue 实例和端口，不加载正式应用、Store 或后台代理；这属于开发运行隔离，不是执行任意代码的安全沙盒。

## 使用技能

在 Codex 中输入：`用 $ui-prototype 做一个设备分组页面，沿用现有组件和主题，先给我看原型。`

技能位于 `.agents/skills/ui-prototype/SKILL.md`，作用域是本项目。目录未出现在技能列表时，重新打开项目会话。

## 添加方案

1. 新建 `prototypes/cases/<功能名>-v1/`。
2. 添加 `meta.ts`，默认导出 `{ title: '页面名称' }`；可参考已有示例的 `PrototypeMeta` 类型。
3. 添加 `Prototype.vue`，使用 `@/components/ui/*` 等已有组件；数据与交互保存在原型内。
4. 下拉列表自动发现新目录，无需修改正式路由。不同方案使用不同目录名。

`index.html` 只负责挂载 Vue，界面写在 `.vue` 中。主题直接引用 `frontend/src/style.css`；原型入口单独声明 Tailwind 扫描来源，不修改正式主题。

## 检查与构建

```sh
pnpm --dir frontend proto:check
pnpm --dir frontend proto:build
pnpm --dir frontend proto:preview
```

`proto:build` 输出到 `prototypes/dist/`，不写入正式的 `web/dist/`。`proto:preview` 用于检查构建后的页面，与开发预览共用 5174 端口，运行前先停掉开发预览。源码保留版本记录，`dist/`、`artifacts/` 和本地环境文件忽略。

定稿后另行接入正式页面、路由与 API，保留原型方案。原型支持本地模拟交互，不代表对应业务功能已经实现。
