# v1.11.0-feature 任务清单

## 实现

- ✅ 移除 Python/PyAV/OpenCV 模板 Worker、嵌入脚本和相关依赖。
- ✅ 移除 Go 模板匹配器、实时帧消费者、匹配结果发布和模板自动点击。
- ✅ 保留模板资产 CRUD、图片变体、下载、导入导出和存储目录。
- ✅ 移除匹配状态、检测、命中、点击和前端 feed 接口。
- ✅ 将播放器模板入口改为模板管理，移除匹配开关、阈值、命中状态和点击目标操作。
- ✅ 更新 Docker、README、Vision 文档与 OpenAPI。

## 测试与验证

- ✅ `go test ./...`
- ✅ `go vet ./...`
- ✅ `go build ./...`
- ✅ `node --check web/js/template-match.js`
- ✅ `git diff --check`
- ⬜ 在真实设备上对比移除前后的投屏坏帧、CPU 和帧连续性。

## 发布

- ⬜ 根据用户授权提交本版本相关代码与文档。
- ⬜ 根据用户授权部署并验证运行中的服务。
