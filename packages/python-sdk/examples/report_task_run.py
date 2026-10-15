"""外部任务上报与设备执行示例。

演示如何使用 TaskReporter 创建外部任务运行、实时采集标准输出/错误输出并向 MyWebScrcpy 上报，
支持后台心跳、日志批量上传及 Markdown 结果提交。
"""

import argparse
import os
import time
from mywebscrcpy import Client, TaskReporter


def main():
    parser = argparse.ArgumentParser(description="外部脚本执行与任务上报示例")
    parser.add_argument("serial", help="ADB 设备序列号")
    parser.add_argument("--base-url", default=os.environ.get("MYWEBSCRCPY_API_URL", "http://127.0.0.1:8080"), help="MyWebScrcpy 服务地址")
    options = parser.parse_args()

    print(f"开始执行外部任务: 设备={options.serial}, 服务={options.base_url}")

    reporter = TaskReporter(
        base_url=options.base_url,
        name="外部自动化集成示例",
        device_serial=options.serial,
        source="codex",
    )

    # 使用上下文管理器自动处理生命周期、心跳与异常
    with reporter:
        with reporter.capture_output():
            print("阶段 1：初始化连接并查询设备状态")
            with Client(options.base_url) as client:
                matches = [item for item in client.devices.list() if item.serial == options.serial]
                if len(matches) != 1 or matches[0].state != "device":
                    raise RuntimeError("指定设备未在线")
                print(f"设备已连接: 产品型号={matches[0].model}, 状态={matches[0].state}")
                with client.device(options.serial) as device:

                    print("阶段 2：执行设备操作 (获取当前屏幕电量/电源状态)")
                    res = device.adb(["shell", "dumpsys", "power"], timeout=10)
                    lines = [l for l in res.stdout.splitlines() if "Display Power" in l or "mWakefulness" in l]
                    for line in lines:
                        print(f"  > {line.strip()}")

                    time.sleep(1)
                    print("阶段 3：执行完毕，准备提交成功报告")

        # 显式提交终态报告
        summary = "设备操作成功完成，已读取电源指标。"
        report_md = f"""# 自动化执行报告

- **目标设备**: `{options.serial}`
- **执行来源**: `codex`
- **执行结果**: 成功

## 步骤说明
1. 连接 MyWebScrcpy 服务并验证设备在线。
2. 调用 `adb dumpsys power` 成功采集前台电源指标。
3. 经由 Python SDK 将执行日志实时上报至任务面板。
"""
        reporter.finish(status="succeeded", exit_code=0, summary=summary, report_markdown=report_md)

    print("外部任务上报成功完成。")


if __name__ == "__main__":
    main()
