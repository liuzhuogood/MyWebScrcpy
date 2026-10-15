#!/usr/bin/env python3
"""小米 MIUI / 快手极速版：解锁、检查充电弹窗、进入去赚钱（无 OCR）。"""

import argparse
import json
import os
from pathlib import Path
import re
import tempfile
import time
import xml.etree.ElementTree as ET

from mywebscrcpy import Client


PACKAGE = "com.kuaishou.nebula"
COMPONENT = PACKAGE + "/com.yxcorp.gifshow.HomeActivity"
EARN_XPATH = (
    ".//node[@package='com.kuaishou.nebula'][@content-desc='去赚钱']"
    "[@clickable='true'][@enabled='true']"
)


def keyguard(device):
    policy = device.adb(["shell", "dumpsys", "window", "policy"], timeout=10).stdout
    showing = re.findall(r"^\s*mIsShowing=(true|false)\s*$", policy, re.MULTILINE)
    secure = re.findall(r"^\s*secure=(true|false)\s*$", policy, re.MULTILINE)
    if len(showing) != 1 or len(secure) != 1:
        raise RuntimeError("无法唯一读取 MIUI 锁屏状态；停止操作")
    return showing[0] == "true", secure[0] == "true"


def focused_component(device):
    result = device.adb(["shell", "dumpsys", "window"], timeout=10)
    components = re.findall(
        r"mCurrentFocus=Window\{\S+ u\d+ ([^\s}]+/[^\s}]+)\}", result.stdout
    )
    if len(components) != 1:
        raise RuntimeError("无法唯一读取 mCurrentFocus；停止操作")
    return components[0]


def snapshot(device, directory, label, remote_file):
    before = focused_component(device)
    display = device.page_info(timeout=10)
    # 先移除本次运行自己的文件；dump 即使错误地返回 0 也不能读到旧快照。
    for attempt in range(2):
        device.adb(["shell", "rm", "-f", remote_file], timeout=10)
        result = device.adb(["shell", "uiautomator", "dump", remote_file], timeout=30)
        output = result.stdout + result.stderr
        (directory / (label + "-dump-" + str(attempt) + ".txt")).write_text(output, encoding="utf-8")
        if "ERROR: could not get idle state." in output:
            print("UI 尚未空闲，重试只读采集", flush=True)
            time.sleep(1)
            continue
        if "UI hierchary dumped to: " + remote_file not in output:
            raise RuntimeError("UIAutomator 未确认生成新 XML；详见 dump 日志")
        break
    else:
        raise RuntimeError("界面持续动画，无法取得新 XML；停止，禁止使用旧快照")
    xml = device.adb(["shell", "cat", remote_file], timeout=10).stdout
    after = focused_component(device)
    (directory / (label + ".xml")).write_text(xml, encoding="utf-8")
    (directory / (label + ".json")).write_text(json.dumps({
        "component_before": before, "component_after": after,
        "width": display.width, "height": display.height,
        "captured_at": time.time(),
    }, ensure_ascii=False, indent=2), encoding="utf-8")
    if before != after:
        raise RuntimeError("采集期间前台 Activity 改变；停止，保留快照供检查")
    root = ET.fromstring(xml)
    if root.tag != "hierarchy" or not root.findall(".//node"):
        raise RuntimeError("没有有效 UI 节点；停止操作")
    if root.attrib["rotation"] != "0":
        raise RuntimeError("此流程仅验证过竖屏 rotation=0；停止操作")
    return root, display, after


def task_center_state(activity_dump):
    # 此类名、状态字段与任务中心的对应关系已由实机截图及 dump 交叉验证。
    states = re.findall(
        r"(?m)^ +CenterTaskLazyLoadFragmentContainerNew\{[^\n]+\n((?: +[^\n]*\n){6})",
        activity_dump,
    )
    if len(states) > 1:
        raise RuntimeError("任务中心 Fragment 状态不唯一")
    if not states:
        return False
    state = states[0]
    return all(value in state for value in (
        "mAdded=true mRemoving=false", "mHidden=false mDetached=false mMenuVisible=true",
        "mUserVisibleHint=true",
    ))


def tap_node(device, node, display):
    if node.attrib["enabled"] != "true" or node.attrib["clickable"] != "true":
        raise RuntimeError("目标未明确标记为可点击且启用")
    match = re.fullmatch(r"\[(\d+),(\d+)\]\[(\d+),(\d+)\]", node.attrib["bounds"])
    if match is None:
        raise RuntimeError("目标 bounds 格式无效")
    left, top, right, bottom = map(int, match.groups())
    if not (0 <= left < right <= display.width and 0 <= top < bottom <= display.height):
        raise RuntimeError("目标 bounds 超出屏幕基准")
    device.adb(["shell", "input", "tap", str((left + right) // 2),
                str((top + bottom) // 2)], timeout=10)


def charging_cancel(root):
    triggers = root.findall(".//node[@text='仅限充电']")
    if not triggers:
        return None
    if len(triggers) != 1:
        raise RuntimeError("仅限充电节点不唯一；停止操作")
    trigger = triggers[0]
    # 仅在同一 UI 窗口、同一实际包名内查找取消，不点击其他应用的同名按钮。
    windows = [node for node in root if trigger in node.iter()]
    if len(windows) != 1:
        raise RuntimeError("无法确定充电弹窗所属窗口")
    package = trigger.attrib["package"]
    if not package:
        raise RuntimeError("充电节点缺少 package")
    candidates = [node for node in windows[0].findall(".//node[@text='取消']")
                  if node.attrib["package"] == package]
    if len(candidates) != 1:
        raise RuntimeError("充电窗口中的取消按钮不唯一或不存在")
    node = candidates[0]
    parents = {child: parent for parent in windows[0].iter() for child in parent}
    for attempt in range(32):
        if node.attrib["enabled"] != "true":
            raise RuntimeError("取消按钮或祖先被禁用")
        if node.attrib["clickable"] == "true":
            return node
        if node not in parents:
            break
        node = parents[node]
    raise RuntimeError("取消文字没有已启用的可点击祖先")


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("serial", help="已确认的 ADB serial")
    parser.add_argument("--base-url", default=os.environ.get("MYWEBSCRCPY_API_URL", ""))
    parser.add_argument("--token", default=os.environ.get("MYWEBSCRCPY_API_TOKEN", ""))
    parser.add_argument("--token-header", default=os.environ.get("MYWEBSCRCPY_API_TOKEN_HEADER", ""))
    parser.add_argument("--token-prefix", default=os.environ.get("MYWEBSCRCPY_API_TOKEN_PREFIX", ""))
    parser.add_argument("--artifacts", type=Path, help="本次新建的证据目录，必须尚不存在")
    options = parser.parse_args()
    if not options.base_url:
        parser.error("请通过 --base-url 或 MYWEBSCRCPY_API_URL 指定服务")
    headers = {}
    if options.token:
        if not options.token_header:
            parser.error("使用 token 时必须明确 --token-header")
        prefix = options.token_prefix + " " if options.token_prefix else ""
        headers[options.token_header] = prefix + options.token
    if options.artifacts is None:
        directory = Path(tempfile.mkdtemp(prefix="kuaishou-earn-"))
    else:
        directory = options.artifacts
        directory.mkdir(parents=True, exist_ok=False, mode=0o700)
    print("证据目录：" + str(directory.resolve()), flush=True)
    remote_file = "/data/local/tmp/mywebscrcpy-kuaishou-" + str(time.time_ns()) + ".xml"

    with Client(options.base_url, headers=headers) as client:
        with client.device(options.serial) as device:
            try:
                print("步骤 1：唤醒并解除无密码锁屏", flush=True)
                device.adb(["shell", "input", "keyevent", "KEYCODE_WAKEUP"], timeout=10)
                time.sleep(1)
                showing, secure = keyguard(device)
                if showing and secure:
                    raise RuntimeError("设备有安全锁屏，请手动解锁后重跑；不输入或猜测密码")
                device.adb(["shell", "wm", "dismiss-keyguard"], timeout=10)
                for attempt in range(8):
                    time.sleep(0.5)
                    showing, secure = keyguard(device)
                    if not showing:
                        break
                else:
                    raise RuntimeError("解除锁屏后状态仍为锁定")

                if focused_component(device) == COMPONENT:
                    print("当前焦点在快手，先回桌面，避免活动动画阻塞弹窗检查", flush=True)
                    device.adb(["shell", "input", "keyevent", "KEYCODE_HOME"], timeout=10)
                    time.sleep(1)

                print("步骤 2：用 UI XML 检查仅限充电弹窗", flush=True)
                root, display, component = snapshot(device, directory, "charging", remote_file)
                cancel = charging_cancel(root)
                cancelled = cancel is not None
                if cancelled:
                    tap_node(device, cancel, display)
                    root, display, component = snapshot(device, directory, "after-cancel", remote_file)
                    if root.findall(".//node[@text='仅限充电']"):
                        raise RuntimeError("点击取消后充电弹窗仍存在；不重复点击")
                    print("已取消充电弹窗", flush=True)
                else:
                    print("未发现仅限充电节点", flush=True)

                print("步骤 3：冷启动快手极速版，统一回到启动页面", flush=True)
                result = device.adb([
                    "shell", "am", "start", "-W", "-S", "-a", "android.intent.action.MAIN",
                    "-c", "android.intent.category.LAUNCHER", "-n", COMPONENT,
                ], timeout=30)
                (directory / "launch.txt").write_text(result.stdout + result.stderr, encoding="utf-8")
                if "Status: ok" not in result.stdout:
                    raise RuntimeError("am start 未返回 Status: ok")

                print("步骤 4：等待并唯一定位去赚钱", flush=True)
                for attempt in range(6):
                    root, display, component = snapshot(device, directory, "home-" + str(attempt), remote_file)
                    if component != COMPONENT:
                        raise RuntimeError("前台不是已验证的快手 HomeActivity")
                    targets = root.findall(EARN_XPATH)
                    if len(targets) > 1:
                        raise RuntimeError("去赚钱匹配到多个控件；停止操作")
                    if len(targets) == 1:
                        tap_node(device, targets[0], display)
                        break
                    time.sleep(1)
                else:
                    raise RuntimeError("限定次数内没有可点击的去赚钱控件")

                for attempt in range(6):
                    before = focused_component(device)
                    result = device.adb(["shell", "dumpsys", "activity", "top"], timeout=15)
                    (directory / ("earn-" + str(attempt) + ".txt")).write_text(result.stdout, encoding="utf-8")
                    after = focused_component(device)
                    if before != COMPONENT or after != COMPONENT:
                        raise RuntimeError("点击后前台离开快手 HomeActivity")
                    if task_center_state(result.stdout):
                        break
                    time.sleep(1)
                else:
                    raise RuntimeError("点击后未观察到可见的任务中心 Fragment")
                (directory / "result.json").write_text(json.dumps({
                    "status": "completed", "serial": options.serial,
                    "charging_cancelled": cancelled, "earn_clicked": True,
                    "task_center_fragment_visible": True,
                    "verification_limit": "RN 内容文字不由 UI XML 暴露；未验证余额、任务或活动内容",
                }, ensure_ascii=False, indent=2), encoding="utf-8")
                print("已点击去赚钱，并确认任务中心 Fragment 可见；任务内容文字不由 XML 暴露。", flush=True)
            except Exception as error:
                (directory / "error.txt").write_text(str(error), encoding="utf-8")
                raise
            finally:
                # 只清理本次运行创建的设备临时 XML，不影响本地失败证据。
                try:
                    device.adb(["shell", "rm", "-f", remote_file], timeout=10)
                except Exception as error:
                    print("临时文件清理失败：" + type(error).__name__, flush=True)


if __name__ == "__main__":
    main()
