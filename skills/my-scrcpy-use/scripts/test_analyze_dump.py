"""Synthetic XML fixtures only; no device calls or private UI data."""

from copy import deepcopy
import unittest
import xml.etree.ElementTree as ET

from analyze_dump import analyze


def response(xml):
    return {'serial': 'fixture-device', 'captured_at': '2026-10-14T00:00:00Z',
            'display_size': {'width': 100, 'height': 200}, 'xml': xml}


class AnalyzerTests(unittest.TestCase):
    def test_parent_target_duplicate_selectors_and_states(self):
        value = response('''<hierarchy rotation="0">
          <node package="fixture.app" resource-id="fixture.app:id/row"
                clickable="true" enabled="true" bounds="[0,0][100,100]">
            <node package="fixture.app" text="Open" clickable="false" enabled="true"
                  class="android.widget.TextView" bounds="[10,10][90,50]"/>
          </node>
          <node package="fixture.app" resource-id="fixture.app:id/row" text="Open"
                clickable="true" enabled="false" bounds="[0,100][100,200]"/>
        </hierarchy>''')
        report = analyze(value, 'fixture-device')
        self.assertEqual(report['summary']['click_marked_nodes'], 1)
        parent, child, disabled = report['nodes']
        self.assertEqual(parent['center_candidate'], [50, 50])
        self.assertEqual(child['parent_id'], 0)
        self.assertEqual(child['ancestor_ids'], [0])
        self.assertEqual(child['click_target_id'], 0)
        self.assertEqual(child['selector_candidates'][0]['matches_in_snapshot'], 2)
        self.assertEqual(parent['selector_candidates'][0]['matches_in_snapshot'], 2)
        self.assertEqual(disabled['declared_actions'], [])
        self.assertEqual(child['tree_path'], '/hierarchy/node[1]/node[1]')

    def test_missing_display_is_not_inferred_from_bounds(self):
        value = response('<hierarchy><node bounds="[0,0][100,200]"/></hierarchy>')
        del value['display_size']
        report = analyze(value, 'fixture-device')
        self.assertIsNone(report['display_size'])
        self.assertIsNone(report['nodes'][0]['center_candidate'])
        self.assertEqual(report['nodes'][0]['bounds_issue'], 'missing_display_size')
        value['display_size'] = {'width': 100}
        self.assertIsNone(analyze(value, 'fixture-device')['display_size'])

    def test_invalid_bounds_do_not_create_coordinates(self):
        for bounds, issue in [('[0,0][0,20]', 'empty_or_reversed_bounds'),
                              ('[-5,0][20,20]', 'outside_display_size'),
                              ('[0,0][100,201]', 'outside_display_size'),
                              ('bad', 'invalid_bounds')]:
            with self.subTest(bounds=bounds):
                report = analyze(response(f'<hierarchy><node bounds="{bounds}"/></hierarchy>'),
                                 'fixture-device')
                self.assertIsNone(report['nodes'][0]['center_candidate'])
                self.assertEqual(report['nodes'][0]['bounds_issue'], issue)

    def test_missing_package_selector_counts_all_matches(self):
        report = analyze(response('''<hierarchy>
            <node text="Same"/><node text="Same" package="fixture.app"/>
            <node text="Same" package="fixture.other"/>
        </hierarchy>'''), 'fixture-device')
        selectors = [node['selector_candidates'][0] for node in report['nodes']]
        self.assertEqual(selectors[0]['matches_in_snapshot'], 3)
        self.assertEqual(selectors[1]['matches_in_snapshot'], 1)

    def test_required_fields_serial_and_xml_are_strict(self):
        value = response('<hierarchy/>')
        for key in ('serial', 'captured_at', 'xml'):
            invalid = deepcopy(value)
            del invalid[key]
            with self.assertRaises(ValueError):
                analyze(invalid, 'fixture-device')
        with self.assertRaises(ValueError):
            analyze(value, 'other-device')
        with self.assertRaises(ET.ParseError):
            analyze(response('<hierarchy>'), 'fixture-device')
        with self.assertRaises(ValueError):
            analyze(response('<other/>'), 'fixture-device')
        with self.assertRaises(ValueError):
            analyze(response('<!DOCTYPE hierarchy><hierarchy/>'), 'fixture-device')

    def test_system_ui_and_empty_tree_do_not_invent_actions(self):
        report = analyze(response('<hierarchy><node package="com.android.systemui" '
                                  'enabled="true" clickable="false"/></hierarchy>'), 'fixture-device')
        self.assertEqual(report['summary']['click_marked_nodes'], 0)
        self.assertIsNone(report['nodes'][0]['click_target_id'])
        self.assertTrue(any('System UI' in warning for warning in report['warnings']))
        empty = analyze(response('<hierarchy/>'), 'fixture-device')
        self.assertEqual(empty['summary']['node_count'], 0)
        self.assertTrue(empty['warnings'])

    def test_password_text_is_not_used_as_locator(self):
        report = analyze(response('<hierarchy><node text="fixture-private" password="true" '
                                  'resource-id="fixture.app:id/password"/></hierarchy>'), 'fixture-device')
        self.assertEqual(report['nodes'][0]['attributes']['text'], '[redacted]')
        self.assertEqual(report['summary']['text_nodes'], 0)
        conditions = report['nodes'][0]['selector_candidates'][0]['conditions']
        self.assertEqual(conditions, {'resource-id': 'fixture.app:id/password'})

    def test_disabled_or_unknown_child_does_not_suggest_parent_click(self):
        for attribute in ('enabled="false"', ''):
            report = analyze(response(f'<hierarchy><node enabled="true" clickable="true">'
                                      f'<node {attribute} text="Fixture"/></node></hierarchy>'),
                             'fixture-device')
            self.assertIsNone(report['nodes'][1]['click_target_id'])


if __name__ == '__main__':
    unittest.main()
