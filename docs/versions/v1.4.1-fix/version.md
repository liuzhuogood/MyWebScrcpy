# v1.4.1-fix 版本说明

1. 修复 Vision WebSocket 在握手限帧和静止画面场景下的写入超时处理，并记录写入错误调试事件。
2. 补充 Python Vision 实时视频流对接说明，明确输入为内存中的 H.264 解码帧，不依赖本地截图文件。
3. 播放器增加“原始尺寸窗口”，便于按视频帧原始像素查看投屏与识别框。
4. 验证：Go 测试、`go vet`、前端 JavaScript 语法检查、Python 语法检查和 `git diff --check`。
