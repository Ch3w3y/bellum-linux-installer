#!/usr/bin/env python3
import importlib.util
from pathlib import Path
import unittest

spec = importlib.util.spec_from_file_location('notes', Path(__file__).with_name('release-notes.py'))
notes = importlib.util.module_from_spec(spec)
spec.loader.exec_module(notes)


def release(tag, body=None, **flags):
    return dict(tag_name=tag, body=body or ('Notes for ' + tag), **flags)


class ReleaseNotesTests(unittest.TestCase):
    def setUp(self):
        self.pins = notes.pins((Path(__file__).resolve().parents[1] / 'pkg/config/versions.go').read_text())

    def test_unchanged_pins_do_not_fetch_upstream(self):
        def forbidden(path):
            self.fail('unexpected upstream API call: ' + path)
        text = notes.render(self.pins, self.pins, 'Installer fix', forbidden)
        self.assertIn('Installer fix', text)
        self.assertIn('(unchanged)', text)
        self.assertIn(self.pins['ProtonSHA256'], text)

    def test_complete_proton_range_excludes_other_variants(self):
        old = dict(self.pins, ProtonVer='proton-cachyos-11.0-20261005-slr-x86_64')
        releases = [release('cachyos-11.0-20261015-slr', 'Latest fixes'),
                    release('cachyos-11.0-20261014', 'Native only'),
                    release('cachyos-11.0-20261014-slr', 'DXVK and vkd3d updates'),
                    release('cachyos-11.0-20261005-slr', 'Old notes')]
        text = notes.render(self.pins, old, 'Pin update', lambda path: releases)
        self.assertIn('Latest fixes', text)
        self.assertIn('DXVK and vkd3d updates', text)
        self.assertNotIn('Native only', text)
        self.assertNotIn('Old notes', text)
        self.assertLess(text.index('DXVK and vkd3d updates'), text.index('Latest fixes'))

    def test_umu_range_and_new_launcher_builds(self):
        current = dict(self.pins, UMUVersion='1.4.6', launcher=self.pins['launcher'] + ['a' * 64])
        releases = [release('1.4.6'), release('1.4.5'), release('1.4.4')]
        text = notes.render(current, self.pins, '', lambda path: releases)
        self.assertIn('Notes for 1.4.5', text)
        self.assertIn('Notes for 1.4.6', text)
        self.assertIn('a' * 64, text)

    def test_missing_baseline_does_not_silently_omit_changes(self):
        with self.assertRaises(ValueError):
            notes.upstream_notes('repo', 'old', 'new', [release('new')], lambda tag: True)

    def test_downgrade_requires_manual_review(self):
        with self.assertRaises(ValueError):
            notes.upstream_notes('repo', 'new', 'old', [release('new'), release('old')], lambda tag: True)

    def test_baseline_uses_published_stable_numeric_order(self):
        releases = [release('v2.4.10'), release('v2.4.9'),
                    release('v2.4.11', draft=True), release('v2.5.0-rc.1', prerelease=True)]
        self.assertEqual(notes.baseline('v2.4.12', releases), 'v2.4.10')

    def test_changed_content_at_same_tag_is_explicit(self):
        current = dict(self.pins, ProtonSHA256='b' * 64)
        self.assertIn('checksum changed', notes.render(current, self.pins, ''))

    def test_malformed_pins_fail(self):
        with self.assertRaises(ValueError):
            notes.pins('')


if __name__ == '__main__':
    unittest.main()
