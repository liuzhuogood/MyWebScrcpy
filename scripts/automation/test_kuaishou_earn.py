"""合成节点只验证拒绝误点击逻辑，不代替真实充电弹窗测试。"""

from types import SimpleNamespace
from pathlib import Path
import tempfile
import unittest
from unittest.mock import patch
import xml.etree.ElementTree as ET

from kuaishou_earn import charging_cancel, snapshot, tap_node, task_center_state


class SafetyTests(unittest.TestCase):
    def test_no_dialog_does_not_select_unrelated_cancel(self):
        root = ET.fromstring('<hierarchy><node text="取消"/></hierarchy>')
        self.assertIsNone(charging_cancel(root))

    def test_cancel_is_scoped_to_dialog_window(self):
        root = ET.fromstring('''<hierarchy>
          <node package="usb"><node text="仅限充电" package="usb"/>
            <node text="取消" package="usb" clickable="true" enabled="true"/></node>
          <node package="other"><node text="取消" package="other"/></node>
        </hierarchy>''')
        self.assertIs(charging_cancel(root), root[0][1])

    def test_multiple_cancel_buttons_stop(self):
        root = ET.fromstring('''<hierarchy><node package="usb">
          <node text="仅限充电" package="usb"/>
          <node text="取消" package="usb"/><node text="取消" package="usb"/>
        </node></hierarchy>''')
        with self.assertRaises(RuntimeError):
            charging_cancel(root)

    def test_cancel_text_uses_enabled_clickable_parent(self):
        root = ET.fromstring('''<hierarchy><node package="usb">
          <node text="仅限充电" package="usb"/>
          <node package="usb" enabled="true" clickable="true">
            <node text="取消" package="usb" enabled="true" clickable="false"/>
          </node></node></hierarchy>''')
        self.assertIs(charging_cancel(root), root[0][1])

    def test_invalid_bounds_never_send_tap(self):
        node = ET.fromstring('<node enabled="true" clickable="true" bounds="[0,0][1081,2400]"/>')
        with self.assertRaises(RuntimeError):
            tap_node(None, node, SimpleNamespace(width=1080, height=2400))

    def test_idle_failure_never_reads_old_xml(self):
        commands = []

        def adb(args, timeout):
            commands.append(args)
            if args[1:3] == ['dumpsys', 'window']:
                return SimpleNamespace(stdout='mCurrentFocus=Window{abc u0 example/.Main}')
            return SimpleNamespace(stdout='ERROR: could not get idle state.', stderr='')

        device = SimpleNamespace(adb=adb, page_info=lambda timeout: SimpleNamespace(width=1080, height=2400))
        with tempfile.TemporaryDirectory() as directory, patch('kuaishou_earn.time.sleep'):
            with self.assertRaisesRegex(RuntimeError, '禁止使用旧快照'):
                snapshot(device, Path(directory), 'test', '/data/local/tmp/example.xml')
        self.assertFalse(any(args[1] == 'cat' for args in commands))

    def test_hidden_task_fragment_is_not_success(self):
        # 字段布局来自已验证 dump；不包含真实账号或页面资料。
        state = '''    CenterTaskLazyLoadFragmentContainerNew{example}, frag=example
        mFragmentId=#800002 mContainerId=#800002 mTag=null
        mState=4 mWho=example mBackStackNesting=0
        mAdded=true mRemoving=false mFromLayout=false mInLayout=false
        mHidden=false mDetached=false mMenuVisible=true mHasMenu=false
        mRetainInstance=false mUserVisibleHint=true
        mFragmentManager=FragmentManager{example}
'''
        self.assertTrue(task_center_state(state))
        self.assertFalse(task_center_state(state.replace('mMenuVisible=true', 'mMenuVisible=false')))
        self.assertFalse(task_center_state(state.replace('mUserVisibleHint=true', 'mUserVisibleHint=false')))
        self.assertFalse(task_center_state(''))


if __name__ == '__main__':
    unittest.main()
