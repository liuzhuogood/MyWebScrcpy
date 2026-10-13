"""Publish matching results; context exit clears the last batch."""
import argparse
import os
from mywebscrcpy import Client


def main():
    parser = argparse.ArgumentParser(description='持续匹配并发布绿框')
    parser.add_argument('serial')
    parser.add_argument('--base-url', default=os.environ.get('MYWEBSCRCPY_API_URL', ''))
    parser.add_argument('--token', default=os.environ.get('MYWEBSCRCPY_API_TOKEN', ''))
    parser.add_argument('--max-fps', type=float, default=5)
    parser.add_argument('--timeout', type=float, default=10)
    options = parser.parse_args()
    headers = {'Authorization': 'Bearer ' + options.token} if options.token else {}
    try:
        with Client(options.base_url, headers=headers) as client:
            with client.device(options.serial) as device:
                with device.watch_templates(publish=True, max_fps=options.max_fps, timeout=options.timeout) as watch:
                    for matches in watch:
                        print([(m.template_id, m.confidence, m.center) for m in matches], flush=True)
    except KeyboardInterrupt:
        pass


if __name__ == '__main__':
    main()
