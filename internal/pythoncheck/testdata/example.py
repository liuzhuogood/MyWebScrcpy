import argparse
import os
from mywebscrcpy import Client


def main():
    parser = argparse.ArgumentParser(description="查询设备型号")
    parser.add_argument("serial", help="ADB 设备序列号")
    parser.add_argument("--base-url", default=os.environ.get("MYWEBSCRCPY_API_URL", ""))
    parser.add_argument("--token", default=os.environ.get("MYWEBSCRCPY_API_TOKEN", ""))
    options = parser.parse_args()
    if not options.base_url:
        raise SystemExit("请通过 --base-url 指定 MyWebScrcpy 服务地址")
    headers = {}
    if options.token:
        headers["Authorization"] = "Bearer " + options.token

    with Client(options.base_url, headers=headers) as client:
        with client.device(options.serial) as device:
            print("步骤 1：读取设备型号", flush=True)
            result = device.adb(["shell", "getprop", "ro.product.model"], timeout=10)
            print(result.stdout, end="", flush=True)


if __name__ == "__main__":
    main()
