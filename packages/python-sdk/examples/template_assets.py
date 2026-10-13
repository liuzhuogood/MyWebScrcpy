import argparse
import os
from pathlib import Path
from mywebscrcpy import Client


def main():
    parser = argparse.ArgumentParser(description='列出设备模板，或从 PNG 创建模板')
    parser.add_argument('serial')
    parser.add_argument('--base-url', default=os.environ.get('MYWEBSCRCPY_API_URL', ''))
    parser.add_argument('--token', default=os.environ.get('MYWEBSCRCPY_API_TOKEN', ''))
    parser.add_argument('--image', type=Path)
    parser.add_argument('--name')
    options = parser.parse_args()
    if options.image and not options.name:
        parser.error('--image 需要 --name')
    headers = {'Authorization': 'Bearer ' + options.token} if options.token else {}
    with Client(options.base_url, headers=headers) as client:
        if options.image:
            print(client.templates.create(options.name, options.image.read_bytes(), serial=options.serial))
        else:
            for template in client.templates.list(serial=options.serial):
                print(template.id, template.name, template.enabled, template.method)


if __name__ == '__main__':
    main()
