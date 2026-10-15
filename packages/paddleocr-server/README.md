# paddleocr-server

MyWebScrcpy 的本地 PaddleOCR sidecar 服务。

## 功能与返回契约

- 提供与 macOCR `/upload` 相同契约的 HTTP 接口，接收 multipart 上传的 PNG 图像。
- 返回识别结果纯文本 `ocr_result`、文字框列表 `ocr_boxes`（包含像素坐标与置信度）以及图像宽高。
- 采用主线程串行推理架构，避免 Paddle CPU 在多线程环境下的死锁问题，自带图像级结果缓存。
- 使用 PaddleOCR v3 API (`PaddleOCR.predict`)，依赖 `paddlepaddle>=3.0.0,<4` 与 `paddleocr>=3.0.0,<4`（支持 Linux/Windows/macOS 官方 wheel）。
- 固定按 `rec_texts`、`rec_scores`、`rec_polys` 字段提取，兼容 list 与 NumPy ndarray 结构。

## 安装与启动

根据 MyWebScrcpy 运行配置所选的 Python 解释器（默认在 `dependencies/python/bin/python3.12` 或系统 Python），使用该解释器安装依赖：

```bash
# 使用所选 Python 解释器安装依赖
<所选 Python 路径> -m pip install -r requirements.txt

# 启动服务（默认端口 8031）
<所选 Python 路径> server.py --host 0.0.0.0 --port 8031
```

## 接口

- `GET /health`：模型加载完成返回 `{"status": "ok", "ready": true}`；初始化期间返回 HTTP 503。
- `POST /upload`：图片文字识别（参数 `file`: PNG 二进制 multipart/form-data）
