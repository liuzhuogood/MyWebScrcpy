"""Match a confirmed template; --tap explicitly opts in to a device action."""
import argparse
import os
from mywebscrcpy import Client


def main():
    parser = argparse.ArgumentParser(description='匹配指定模板')
    parser.add_argument('serial')
    parser.add_argument('template_id')
    parser.add_argument('--base-url', default=os.environ.get('MYWEBSCRCPY_API_URL', ''))
    parser.add_argument('--token', default=os.environ.get('MYWEBSCRCPY_API_TOKEN', ''))
    parser.add_argument('--timeout', type=float, default=10)
    parser.add_argument('--tap', action='store_true', help='等待目标后点击匹配中心')
    options = parser.parse_args()
    headers = {'Authorization': 'Bearer ' + options.token} if options.token else {}
    with Client(options.base_url, headers=headers) as client:
        with client.device(options.serial) as device:
            if options.tap:
                target = device.wait_for(options.template_id, timeout=options.timeout)
                print(device.tap_match(target), flush=True)
            else:
                print(device.match(options.template_id, timeout=options.timeout), flush=True)


if __name__ == '__main__':
    main()
