"""XPath behavior checks against synthetic dumps."""

import html
import json
from pathlib import Path
import subprocess
import sys
import tempfile
import unittest

from test_analyze_dump import response
from xpath_tools import run


class XPathTests(unittest.TestCase):
    def selected(self, xml, target):
        return run(response(xml), 'fixture-device', target_id=target)['xpath_locators'][0]

    def test_id_before_text_and_text_when_id_is_repeated(self):
        locator = self.selected('<hierarchy><node resource-id="fixture:id/button" text="Open"/>'
                                '</hierarchy>', 0)
        self.assertEqual(locator['selected_xpath']['expression'], "//node[@resource-id='fixture:id/button']")
        locator = self.selected('<hierarchy><node resource-id="same" text="First"/>'
                                '<node resource-id="same" text="Second"/></hierarchy>', 1)
        self.assertEqual(locator['selected_xpath']['expression'], "//node[@text='Second']")

    def test_adjacent_feature_resolves_duplicate_id(self):
        xml = '''<hierarchy><node>
          <node text="Account A"/><node resource-id="fixture:id/open"/>
          <node text="Account B"/><node resource-id="fixture:id/open"/>
        </node></hierarchy>'''
        result = self.selected(xml, 4)['selected_xpath']
        self.assertEqual(result['strategy'], 'adjacent_feature')
        self.assertIn('preceding-sibling::node[1]', result['expression'])
        self.assertIn('Account B', result['expression'])
        self.assertEqual(result['matched_node_ids'], [4])

    def test_container_feature_resolves_nested_target(self):
        xml = '''<hierarchy><node>
          <node><node text="Account A"/><node><node resource-id="same"/></node></node>
          <node><node text="Account B"/><node><node resource-id="same"/></node></node>
        </node></hierarchy>'''
        result = self.selected(xml, 8)['selected_xpath']
        self.assertEqual(result['strategy'], 'container_feature')
        self.assertEqual(result['matched_node_ids'], [8])

    def test_bounds_only_after_semantic_and_relation_candidates(self):
        result = self.selected('<hierarchy><node class="Button" bounds="[0,0][50,50]"/>'
                               '<node class="Button" bounds="[0,50][50,100]"/></hierarchy>', 1)
        self.assertEqual(result['selected_xpath']['basis'], 'geometry')
        self.assertEqual(result['selected_xpath']['matched_node_ids'], [1])
        invalid = self.selected('<hierarchy><node bounds="[0,0][500,500]"/></hierarchy>', 0)
        self.assertIsNone(invalid['selected_xpath'])

    def test_both_quote_types_are_escaped(self):
        text = '''Bob's "Open" button'''
        xml = '<hierarchy><node text="' + html.escape(text, quote=True) + '"/></hierarchy>'
        result = self.selected(xml, 0)['selected_xpath']
        self.assertIn('concat(', result['expression'])
        self.assertEqual(result['matched_node_ids'], [0])

    def test_duplicate_identical_nodes_remain_unresolved(self):
        xml = '<hierarchy><node resource-id="same"/><node resource-id="same"/></hierarchy>'
        result = self.selected(xml, 0)
        self.assertIsNone(result['selected_xpath'])
        self.assertTrue(all(test['match_count'] == 2 for test in result['tested_candidates']))

    def test_tests_report_zero_multiple_and_wrong_unique_target(self):
        value = response('<hierarchy><node text="A"/><node text="B"/></hierarchy>')
        report = run(value, 'fixture-device', target_id=0, expressions=(
            '//node[@text="Missing"]', '//node', '//node[@text="B"]'))
        tests = report['xpath_tests']
        self.assertEqual([test['match_count'] for test in tests], [0, 2, 1])
        self.assertFalse(tests[2]['verified_target'])
        with tempfile.TemporaryDirectory() as folder:
            capture = Path(folder) / 'capture.json'
            capture.write_text(json.dumps(value))
            result = subprocess.run([sys.executable, str(Path(__file__).with_name('xpath_tools.py')),
                str(capture), '--expected-serial', 'fixture-device', '--node-id', '0',
                '--xpath', '//node[@text="B"]'], capture_output=True, check=False)
            self.assertEqual(result.returncode, 1)
            self.assertFalse(json.loads(result.stdout)['xpath_tests'][0]['verified_target'])

    def test_syntax_and_scalar_results_are_rejected(self):
        for expression in ('//*[', 'count(//node)', '//node/@text', '/hierarchy'):
            with self.subTest(expression=expression), self.assertRaises(ValueError):
                run(response('<hierarchy><node text="A"/></hierarchy>'), 'fixture-device',
                    expressions=(expression,))


if __name__ == '__main__':
    unittest.main()
