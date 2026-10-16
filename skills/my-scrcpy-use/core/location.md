# 安卓定位控制

使用 [location_client.py](../scripts/location_client.py) 操作手机上的定位服务。所有接口调用都必须通过该脚本完成；不要根据旧记录或示例手工拼接请求。

## 连接与文档

1. 用户给出地址时传 `--base-url`，例如 `http://10.0.0.104:8888`。
2. 未给地址时，脚本从 `adb devices` 选择设备，通过 `adb shell ip -o -4 addr show scope global` 获取候选 IPv4，并逐个探测 `8888`。多台在线设备时必须传 `--serial`。
3. 候选 IP 均不可访问时，脚本为目标设备建立 `adb forward tcp:8888 tcp:8888`，再访问 `http://127.0.0.1:8888`。传 `--adb-forward` 可明确要求直接使用该转发方式。已有冲突映射或本机端口被占用时停止，不覆盖。
4. 每次执行命令前，脚本先请求根页面并解析当前接口文档。端口不通、页面不是定位接口文档，或所需方法与路径未出现在文档中时停止。禁止猜测路径、字段、认证或兼容接口。

只探测连接和查看脚本从实时页面识别出的接口：

```bash
python3 skills/my-scrcpy-use/scripts/location_client.py \
  --base-url http://10.0.0.104:8888 docs

python3 skills/my-scrcpy-use/scripts/location_client.py \
  --serial <adb-serial> docs

python3 skills/my-scrcpy-use/scripts/location_client.py \
  --serial <adb-serial> --adb-forward docs
```

## 调用

先运行 `docs` 阅读当前输出，再选择该页面实际声明的命令和参数。常用形式：

```bash
# 查询当前位置
python3 skills/my-scrcpy-use/scripts/location_client.py \
  --base-url http://10.0.0.104:8888 current

# 指定 WGS84 坐标；修改成功后脚本自动 GET /api/location/current 验证
python3 skills/my-scrcpy-use/scripts/location_client.py \
  --base-url http://10.0.0.104:8888 set \
  --lat 28.2282 --lng 112.9388 --coord-type wgs84 --name 长沙

# 按地名定位；脚本仍会在修改后回读当前位置
python3 skills/my-scrcpy-use/scripts/location_client.py \
  --serial <adb-serial> set --place 长沙南站

# 方向移动或路线任务
python3 skills/my-scrcpy-use/scripts/location_client.py \
  --serial <adb-serial> direction --bearing NE --speed-preset run
python3 skills/my-scrcpy-use/scripts/location_client.py \
  --serial <adb-serial> route --dest-place 长沙南站 --mode walking --speed-mps 1.2
```

脚本还提供 `geocode`、`task`、`cancel`、`address`、`gyro`。用 `command --help` 查看参数；只提交用户要求且当前文档存在的字段，不增加默认抖动、速度或名称。坐标与地名二选一等约束以本次抓到的文档为准，脚本只实现已确认结构。

`set`、`direction`、`route` 和 `cancel` 都属于状态修改。脚本要求修改接口统一响应 `code == 0`，随后强制调用当前文档中的 `GET /api/location/current`，输出 `mutation` 与 `verification.current`。直接坐标且未启用抖动时，还会按对应坐标系比较回读坐标；不一致时非零退出。方向和路线会额外回读当前任务，但不会把“任务已创建”误报为“已到达终点”。

## 交付结论

说明实际连接方式（手机 IP 或 ADB 转发）、调用的方法与路径、修改响应、回读坐标和任务状态。区分：接口接受了请求、当前位置已回读、坐标已匹配、移动任务仍在运行、路线是否完成。不得只凭 HTTP 200、脚本退出码或接口 `msg` 宣称目标已经实现。
