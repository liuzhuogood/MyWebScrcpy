---
name: dingtalk-punch
description: 钉钉自动化打卡技能：打开“我的笔记”APP 启动定位服务、使用 android-location-control 注入并校验指定城市经纬度、返回桌面通过钉钉小组件去打卡、轮询打卡按钮并点击、通过 OCR 全文判断打卡结果。
---

# 钉钉自动化打卡

结合 MyWebScrcpy 的 OCR 能力与手机端模拟定位服务，在 Android 设备上自动化执行完整的考勤打卡流程。

## 执行流程

```mermaid
flowchart TD
    Start([开始打卡流程]) --> S1[步骤 1: 桌面 OCR 识别并点击 '我的笔记']
    S1 --> S2[步骤 2: 等待 8888 端口就绪，设置目标经纬度]
    S2 --> S3{步骤 3: 逆地理编码确认是否为'长沙市'?}
    S3 -- 否 --> AbortLoc[终止: 城市不匹配，防止误打卡]
    S3 -- 是 --> S4[步骤 4: 发送 KEYCODE_HOME 返回桌面，OCR 点击 '去打卡']
    S4 --> S5[步骤 5: 轮询等待 '上班打卡' 或 '下班打卡' 按钮 (上限 60s)]
    S5 --> S5_Check{检测到按钮?}
    S5_Check -- 超时未出现 --> AbortTimeout[终止: 打卡按钮未出现或页面异常]
    S5_Check -- 出现异常提示(如不在范围) --> AbortScope[终止: 提示不在考勤范围]
    S5_Check -- 出现打卡按钮 --> S6[步骤 6: 点击对应打卡按钮]
    S6 --> S7[步骤 7: 获取新画面 OCR 识别结果]
    S7 --> S7_Judge{识别到成功标识?}
    S7_Judge -- 是 (打卡成功/已打卡) --> Success([打卡成功，记录时间戳])
    S7_Judge -- 否 (打卡失败/人脸/异常) --> Fail([打卡失败，保存现场证据])
```

## 步骤说明

1. **打开“我的笔记”APP**：
   - 处于解锁桌面时，抓取屏幕画面调用 `/api/vision/ocr` 寻找“我的笔记”文本（实测坐标约为 `[801, 401][1017, 653]`），点击后等待后台 8888 端口服务就绪。
2. **设置模拟定位**：
   - 调用 `android-location-control` 技能的 `location_client.py` 发送目标坐标（默认 `28.15439276212, 112.93746766578`，坐标系 `wgs84`）。
3. **确认目标城市**：
   - 调用 `address` 接口回读逆地理编码信息，确认包含目标城市（如“长沙”）。
4. **返回桌面并点击“去打卡”**：
   - 发送 `input keyevent KEYCODE_HOME` 回到桌面。
   - OCR 识别钉钉桌面小组件（`com.alibaba.android.rimet:id/tv_attendance`）上的“去打卡”文字并点击。
5. **等待打卡按钮（最多 1 分钟）**：
   - 循环轮询识别 `["上班打卡", "下班打卡"]`，若检测到“不在考勤范围”等异常提示则提前终止。
6. **点击打卡按钮**：
   - 点击匹配到的“上班打卡”或“下班打卡”按钮中心。
7. **校验打卡结果**：
   - 抓取打卡后页面全文 OCR，检测“打卡成功”、“已打卡”及打卡时间，输出结构化结果。

## 脚本使用

脚本路径：`scripts/dingtalk_punch.py`

```bash
python3 .agents/skills/dingtalk-punch/scripts/dingtalk_punch.py \
  --backend-url http://127.0.0.1:8080 \
  --serial 10.0.0.104:5555 \
  --location-url http://10.0.0.104:8888 \
  --lat 28.15439276212 --lng 112.93746766578 \
  --coord-type wgs84 --expected-city 长沙
```

单元测试：
```bash
python3 .agents/skills/dingtalk-punch/scripts/test_dingtalk_punch.py
```
