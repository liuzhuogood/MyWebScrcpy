# SDK API sdk-v1

使用 pip 安装的 mywebscrcpy-sdk，版本与主程序一致，不导入 mywebscrcpy_runtime；API 字段与单位对照冻结 api/openapi.json。完整 Vision 与匹配接口仍属于 v1.22 后续工作，不将设计 API 当成已实现能力。

参考 assets/example.py：唯一 main() 解析 serial 位置参数；--base-url 默认读取 MYWEBSCRCPY_API_URL，--token 默认读取 MYWEBSCRCPY_API_TOKEN（可为空）。先在所选 Python 环境安装与主程序版本一致的 mywebscrcpy-sdk，再运行 `python script.py SERIAL --base-url http://HOST:PORT`。Client(headers=...) 接收显式鉴权头，不读取未知凭据。通过 with Client(...) as client / with client.device(...) as device 管理关闭。

| 方法与类型 | 约定 |
| --- | --- |
| Client(base_url, *, headers=None, timeout=30, verify=True) | HTTP(S) 地址，不含凭据、查询或接口路径；默认验证 TLS。生成代码保持 verify=True，不传关闭验证选项 |
| client.device(serial) | 创建绑定设备的基础 HTTP 对象，不启动主服务、匹配器或视频连接 |
| device.adb(args, timeout=30) | 非空字符串数组；不含 adb、-s、--serial、-H、-P、-L。timeout 单位秒，范围大于 0 且不超过 600。SDK 转换为 timeout_ms，调用 POST /api/adb/commands，同步串行执行 |
| CommandResult | stdout/stderr 字符串、exit_code 整数或 None、status 字符串、truncated 布尔。成功必为 completed 和 exit_code=0，失败、截断、超时或协议缺字段抛异常 |
| device.page_info(timeout=10) | GET /api/ui/page-info，严格读取 display.width/height；返回 PageInfo.width/height 正整数，不猜尺寸字段 |

基本动作例子：Home `device.adb(["shell","input","keyevent","3"], timeout=10)`；像素点击 `device.adb(["shell","input","tap","100","200"], timeout=10)`；型号查询 `device.adb(["shell","getprop","ro.product.model"], timeout=10)`。滑动末尾时长为毫秒。比例坐标先取尺寸并换算为明确整数，不猜坐标单位。ASCII 文本中的空格编码为 %s，shell 字符用 shlex.quote；不支持字面 %s 或中文输入。

允许 argparse、os、time、shlex 直接导入，os 仅访问上述连接环境变量。业务逻辑只调用 Client/client.device/device.adb/device.page_info、time.sleep（0 到 600 秒）、shlex.quote 和 print/str/int/float/len/range/bool/abs/min/max/RuntimeError/SystemExit。允许解析器 add_argument/parse_args，结果属性只使用本表字段。有限循环采用常量整数 range，最多 1000 次。需要业务失败时 raise RuntimeError，不吞异常。
