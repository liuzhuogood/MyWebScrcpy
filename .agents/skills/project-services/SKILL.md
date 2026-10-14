---
name: project-services
description: 使用 scripts/services.sh 启动、停止、重启 MyWebScrcpy 前后端，或查看服务状态和日志。
---

# 项目服务

在项目根目录按用户要求执行：

```bash
./scripts/services.sh start    # 启动前后端
./scripts/services.sh stop     # 停止前后端
./scripts/services.sh restart  # 重启前后端
./scripts/services.sh status   # 查看状态
./scripts/services.sh logs     # 查看日志
```

仅操作某一服务时，在命令后加 `backend` 或 `frontend`，如 `./scripts/services.sh stop frontend`。

默认前端端口 5173、后端端口 8080；日志在 `.cache/services/`。执行失败时查看对应服务日志，说明原因。
