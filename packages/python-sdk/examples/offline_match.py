import argparse
from mywebscrcpy import Matcher


def main():
    parser = argparse.ArgumentParser(description='本地图像离线匹配')
    parser.add_argument('image')
    parser.add_argument('template')
    parser.add_argument('--threshold', type=float, default=.88)
    options = parser.parse_args()
    import cv2
    image, template = cv2.imread(options.image), cv2.imread(options.template)
    if image is None or template is None:
        raise SystemExit('无法读取图像')
    matches = Matcher().match_image(image, template, threshold=options.threshold)
    print([(m.confidence, m.center) for m in matches])


if __name__ == '__main__':
    main()
