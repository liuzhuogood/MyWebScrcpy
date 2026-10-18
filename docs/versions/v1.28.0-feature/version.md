# v1.28.0-feature：OCR 防抖与并行模式

状态：功能已实现，自动化与浏览器组件验收通过；真实设备验收待指定设备。

1. OCR 接口新增 `stabilization_ms`（整数毫秒，0～5000，默认 0），在设备画面采样窗口内多次识别并综合；上传单张图片继续支持单次识别，正时长须通过 serial 连续取帧。
2. 保留 auto 按 Indra OCR → macOCR → PaddleOCR 串行尝试；新增独立 parallel 模式，有界并行调用三个配置引擎，融合有效结果并报告部分失败。
3. 按区域与文字票数去重、取稳定坐标，并限定最新轮可见文字；新增 samples、engines_used，覆盖超时、取消、并发容量、旋转与迟到标注防护。
4. 同步 OpenAPI、Python SDK、系统文档和现有引擎选择入口；Python SDK 新增按时长的新帧识别及有界后台容量。

正式 `VERSION` 保持 v1.27.2，尚未合回 main、部署或发布。具体提交与验证结果见 [会话记录](session.md)。
