"""Page identity and capture correlation checks using synthetic responses."""

from copy import deepcopy
import unittest

from analyze_dump import analyze
from page_context import correlate
from test_analyze_dump import response


def page(time):
    return {'package_name': 'fixture.app', 'activity': '.Fixture',
            'component': 'fixture.app/.Fixture', 'window_name': 'FixtureWindow',
            'display': {'width': 100, 'height': 200, 'density_dpi': 320, 'rotation': 0},
            'fetched_at': time}


class PageTests(unittest.TestCase):
    def setUp(self):
        self.analysis = analyze(response('<hierarchy><node package="fixture.app"/></hierarchy>'),
                                'fixture-device')
        self.before = page('2026-10-13T23:59:59Z')
        self.after = page('2026-10-14T00:00:01Z')

    def test_focus_and_dump_are_correlated(self):
        report = correlate(self.analysis, self.before, self.after)
        self.assertEqual(report['status'], 'focus_observed_in_dump')
        self.assertTrue(report['focus_package_in_dump'])
        self.assertTrue(report['display_size_matches'])
        self.assertEqual(report['observation_window_seconds'], 2)

    def test_system_overlay_does_not_become_foreground_business_page(self):
        self.analysis['summary']['packages'] = ['com.android.systemui']
        report = correlate(self.analysis, self.before, self.after)
        self.assertEqual(report['status'], 'focus_absent_from_dump')
        self.assertFalse(report['focus_package_in_dump'])

    def test_changed_page_or_screen_is_reported(self):
        for category, key in (('page', 'component'), ('display', 'rotation')):
            after = deepcopy(self.after)
            if category == 'page':
                after[key] = 'fixture.app/.Other'
            else:
                after['display'][key] = 90
            report = correlate(self.analysis, self.before, after)
            self.assertEqual(report['status'], 'page_observations_changed')
            self.assertIn(key, report[f'changed_{category}_fields'])

    def test_mixed_capture_times_are_not_accepted(self):
        self.before['fetched_at'] = '2026-10-14T00:00:05Z'
        report = correlate(self.analysis, self.before, self.after)
        self.assertEqual(report['status'], 'capture_order_invalid')

    def test_dimensions_are_checked_without_fallback(self):
        self.before['display']['height'] = 180
        self.after['display']['height'] = 180
        report = correlate(self.analysis, self.before, self.after)
        self.assertEqual(report['status'], 'display_size_mismatch')
        self.analysis['display_size'] = None
        self.assertIsNone(correlate(self.analysis, self.before, self.after)['display_size_matches'])

    def test_missing_fields_and_empty_package_are_not_guessed(self):
        invalid = deepcopy(self.after)
        del invalid['package_name']
        with self.assertRaises(ValueError):
            correlate(self.analysis, self.before, invalid)
        self.before['package_name'] = ''
        self.after['package_name'] = ''
        report = correlate(self.analysis, self.before, self.after)
        self.assertEqual(report['status'], 'focus_unknown')
        self.assertIsNone(report['focus_package_in_dump'])


if __name__ == '__main__':
    unittest.main()
