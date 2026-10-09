# 阻塞事项跟踪

> 需用户或外部介入才能关闭的事项。关闭时注明结论，不要删除历史。

## 1. 远端缺失，无法 push（待用户）

- 现状：`MyWebScrcpyNew` 仅 `git init` + 本地提交（`0cc5a85` 起），从未配置 origin。
- 解法二选一：用户给远端仓库地址（`git remote add + fetch + push`）；或授权用 `gh` 新建 GitHub 仓库再推。
- 影响：v0.3.0–v0.6.0 四批增量滞留本地。

## 2. 模板定位点击与命中为 0（待后端）

- 现象：`POST /api/templates/click` 返回 `adb screencap disabled: please use real-time stream engine`；`GET /api/templates/matches` 恒为 0。
- 结论：前端链路已按 Go 源码契约验证通过（请求格式正确、错误正常回显）；是该环境后端截屏/匹配能力受限，属老仓库范畴，与本前端仓库无关。
- 跟踪：后端恢复后，只需在投屏页点一次“定位点击”即算复验通过。

## 3. 网络 ADB 偶发 offline（已缓解）

- 现象：两台网络 ADB 曾同时 offline，`adb connect` 超时；重启 ADB daemon 后自动重连恢复。
- 结论：手机端 WiFi/ADB 稳定性问题；复发时先重启 daemon，仍不行再查手机端。
