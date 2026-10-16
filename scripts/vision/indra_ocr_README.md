# Indra OCR 独立视频流测试

独立后台脚本，订阅正在运行的 MyWebScrcpy 视频，识别文字并回传检测框。没有 HTTP 监听端口，不注册到项目服务管理，不修改 OCR 引擎或配置。

打开目标设备的播放器，在播放器设置中开启「显示匹配绿框」，然后运行：

```bash
uv run --python 3.12 scripts/vision/indra_ocr_service.py --serial '设备序列号'
```

脚本包含独立依赖声明，uv 自动准备环境。也可在自己的虚拟环境安装后运行：

```bash
python -m pip install 'indra-ocr==0.3.2' 'av>=14,<20' 'websocket-client>=1.8,<2'
python scripts/vision/indra_ocr_service.py --serial '设备序列号'
```

首次运行通过 Indra 官方逻辑下载 Hugging Face 模型。离线环境可指定 `--model-dir /path/to/models`，或使用官方的 `INDRA_OCR_MODEL_DIR` 环境变量。

```bash
# 指定服务地址，测试 30 秒，最多每秒识别两次
uv run --python 3.12 scripts/vision/indra_ocr_service.py \
  --url http://127.0.0.1:8080 --serial '设备序列号' --interval .5 --seconds 30

# 降低识别分辨率及置信度门槛，便于观察性能和漏识别
uv run --python 3.12 scripts/vision/indra_ocr_service.py \
  --serial '设备序列号' --max-side 1280 --min-confidence .3
```

日志包含来源帧 ID、检测框数量、OCR 耗时以及是否成功发送。`published=False` 表示结果所属画面已旋转、重置，或在画面更新后已超过 600ms，不再显示过期坐标。它不表示 OCR 未执行。服务端拒绝结果会作为错误退出，不会被当作成功。

收帧线程不丢 H.264 预测包，解码后仅保留最新画面。OCR 独占一个子进程，最多一帧处理中，没有待识别帧积压。静止画面仅识别一次，并在该画面仍为当前画面时续期绿框；续期不会改变来源帧 ID 或生成新画面。画面变化后旧结果最多保留来源帧 600ms 的剩余有效期；已发送绿框按其有效期消失。

默认保留原始分辨率，关闭 Indra 自动方向旋转，按 `Line.box / Line.text / Line.conf` 映射成归一化检测框。Indra 的倾斜校正框由其自身映射回输入坐标。非等比坐标猜测、设备点击和系统 OCR 切换不在脚本范围内。

`Ctrl+C` 或 SIGTERM 停止，清空本连接的检测框并回收子进程；断线后绿框按有效期自动消失。模型加载、单帧 OCR、初始画面分别有 300/30/30 秒超时，可用对应命令行参数调整。持续运行期间不会自动重连，断流或解码错误会记录异常并退出。

绿框通道由播放器共享，测试时请暂停同设备的其他框发布脚本，以免互相覆盖。

官方参考：[Python API 与模型说明](https://huggingface.co/datalab-to/indra-ocr)、[Indra 源码](https://github.com/datalab-to/indra)。
