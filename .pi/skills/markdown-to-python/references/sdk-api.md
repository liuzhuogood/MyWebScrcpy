# SDK API sdk-v2

使用项目统一的可安装 mywebscrcpy SDK，不导入 mywebscrcpy_runtime。SDK 版本为 0.2.0，接口实现见 packages/python-sdk；API 字段与单位对照冻结 api/openapi.json。视觉操作需要运行环境预先安装识图依赖。

参考 assets/example.py：唯一 main() 解析 serial 位置参数；--base-url 默认读取 MYWEBSCRCPY_API_URL，--token 默认读取 MYWEBSCRCPY_API_TOKEN（可为空）。外部先安装本地包或 wheel，再运行 `python script.py SERIAL --base-url http://HOST:PORT`。Client(headers=...) 接收显式鉴权头，不读取未知凭据。通过 with Client(...) as client / with client.device(...) as device 管理关闭。

| 方法与类型 | 约定 |
| --- | --- |
| Client(base_url, *, headers=None, timeout=30, verify=True) | HTTP(S) 地址，不含凭据、查询或接口路径；默认验证 TLS。生成代码保持 verify=True，不传关闭验证选项 |
| client.device(serial) | 创建绑定设备的基础 HTTP 对象，不启动主服务、匹配器或视频连接 |
| device.adb(args, timeout=30) | 非空字符串数组；不含 adb、-s、--serial、-H、-P、-L。timeout 单位秒，范围大于 0 且不超过 600。SDK 转换为 timeout_ms，调用 POST /api/adb/commands，同步串行执行 |
| CommandResult | stdout/stderr 字符串、exit_code 整数或 None、status 字符串、truncated 布尔。成功必为 completed 和 exit_code=0，失败、截断、超时或协议缺字段抛异常 |
| device.page_info(timeout=10) | GET /api/ui/page-info，严格读取 display.width/height；返回 PageInfo.width/height 正整数，不猜尺寸字段 |

基本动作例子：Home `device.adb(["shell","input","keyevent","3"], timeout=10)`；像素点击 `device.adb(["shell","input","tap","100","200"], timeout=10)`；型号查询 `device.adb(["shell","getprop","ro.product.model"], timeout=10)`。滑动末尾时长为毫秒。比例坐标先取尺寸并换算为明确整数，不猜坐标单位。ASCII 文本中的空格编码为 %s，shell 字符用 shlex.quote；不支持字面 %s 或中文输入。

视觉与动作能力（需按方法实际签名调用）：

| 方法 | 契约 |
| --- | --- |
| device.frame(timeout=10) | Vision 解码后等待调用后到达的新画面；Frame.width/height/frame_id/session_id/device_id/timestamp；不把缓存当新帧 |
| device.match(template_id, timeout=10) | 按已确认 ID 取模板主图片、新画面并匹配，返回 list[Match]；只有正常未命中为 [] |
| device.wait_for(template_id, timeout=10) | 总期限内等待命中，返回最高分 Match；超时抛错 |
| device.tap_match(match, max_age_ms=600, timeout=10) | 校验来源设备、会话、重置与帧龄，点击 center；返回 ActionResult |
| device.tap(point, timeout=10) | point 为归一化 (x,y)，每项 0..1；通过 action.request/result 返回真实结果 |
| device.swipe(start, end, duration_ms=300, timeout=10) | start/end 均为归一化坐标，duration_ms 为毫秒 |
| device.key(keycode, meta_state=0, timeout=10) | Android uint32 keycode；Home=3，Back=4 |
| device.text(text, timeout=10) | 最多 300 UTF-8 字节；实际设备能否输入具体字符必须验证，不承诺所有中文都可输入 |
| device.watch_templates(publish=False, max_fps=5, timeout=10) | 使用 with ... as watch，再在常量 range 内 next(watch)；每批返回 list[Match]；关闭清框，退出设备关闭连接 |

Match 提供 template_id/name/confidence/x/y/w/h/center/frame_id/session_id/device_id/received_at，center 是归一化坐标；匹配方法支持 TM_CCOEFF_NORMED 和主图片，不支持未知算法、多参考图聚合、OCR。模板只能来自 templates.json 的真实 ID，不按名称猜 ID、不把已禁用或他设备模板用于操作。单次与等待匹配不自动点击，只有显式动作才操作设备。ActionResult 提供 request_id/device_id/session_id/accepted/executed/error_code；失败抛错，不重试结果未知的动作。

允许 argparse、os、time、shlex 直接导入。生成唯一 main()，用 with 管理 client/device/watch；允许已列结果属性和 print/str/int/float/len/range/bool/abs/min/max/next/RuntimeError/SystemExit。next 仅用于持续匹配迭代；有限循环使用常量整数 range，每层最多 1000 次。只读取连接环境变量；不读写磁盘、不直接拼 HTTP/WS、不访问内部属性。等待 time.sleep 参数为 0 到 600 秒的常量。识图依赖由运行环境预先安装，不在业务脚本安装。
