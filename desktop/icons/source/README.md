# 应用 Logo 源图

`logo-user-multiscreen.png` 为用户于 2026-11-05 通过附件最终确认的图案：白色/浅色圆角底板、三个蓝色叠放屏幕。保留原始 PNG，不重绘、不改色、不裁切。

原始交付文件：`/Users/liuzhuo/.codex/generated_images/01a1aa09-d75e-7421-8b63-7f17864ac1b2/exec-c169aba6-d447-4dc2-92ad-b49e2f898176.png`。项目构建只依赖本目录的副本。

在仓库根目录使用已有 Tauri CLI 生成资源：

```sh
pnpm --dir desktop exec tauri icon icons/source/logo-user-multiscreen.png --fit contain --output ../.cache/logo-selected-generated
cp .cache/logo-selected-generated/icon.png desktop/icons/icon.png
cp .cache/logo-selected-generated/icon.icns desktop/icons/icon.icns
cp .cache/logo-selected-generated/icon.ico desktop/icons/icon.ico
cp .cache/logo-selected-generated/icon.png frontend/public/app-icon.png
```

只采用当前桌面配置消费的三种资源；网页 favicon 和顶栏共用 `frontend/public/app-icon.png`。设备当前运行应用的图标不属于本资源。
