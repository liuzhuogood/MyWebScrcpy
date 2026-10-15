import argparse
import os
from mywebscrcpy import Client


def main():
    parser = argparse.ArgumentParser(description="等待指定模板并点击")
    parser.add_argument("serial")
    parser.add_argument("--base-url", default=os.environ.get("MYWEBSCRCPY_API_URL", ""))
    parser.add_argument("--token", default=os.environ.get("MYWEBSCRCPY_API_TOKEN", ""))
    options = parser.parse_args()
    if not options.base_url:
        raise SystemExit("请通过 --base-url 指定服务地址")
    headers = {}
    if options.token:
        headers["Authorization"] = "Bearer " + options.token
    with Client(options.base_url, headers=headers) as client:
        with client.device(options.serial) as device:
            print("步骤 1：等待已确认的目标", flush=True)
            target = device.wait_for(template_id="CONFIRMED_TEMPLATE_ID", timeout=10)
            print("步骤 2：点击目标", flush=True)
            device.tap_match(target, max_age_ms=600, timeout=5)


if __name__ == "__main__":
    main()
