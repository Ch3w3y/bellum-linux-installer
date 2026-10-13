#!/usr/bin/env python3
"""Exercise automatic version selection against real, isolated Git histories."""
import pathlib
import subprocess
import tempfile
import unittest

SCRIPT = pathlib.Path(__file__).with_name("next-release-tag.sh").resolve()


class ReleaseTagTests(unittest.TestCase):
    def setUp(self):
        self.directory = tempfile.TemporaryDirectory()
        self.addCleanup(self.directory.cleanup)
        self.repo = self.directory.name
        self.git("init", "-q")
        self.git("config", "user.name", "Release test")
        self.git("config", "user.email", "release@example.invalid")
        self.commit()

    def git(self, *args):
        return subprocess.check_output(
            ["git", *args], cwd=self.repo, text=True, stderr=subprocess.STDOUT
        ).strip()

    def commit(self):
        self.git("commit", "--allow-empty", "-qm", "Test commit")

    def selected(self):
        return subprocess.check_output([str(SCRIPT)], cwd=self.repo, text=True).strip()

    def test_requires_initial_manual_release(self):
        result = subprocess.run([str(SCRIPT)], cwd=self.repo, capture_output=True, text=True)
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("initial release manually", result.stderr)

    def test_patch_bump(self):
        self.git("tag", "v2.4.0")
        self.commit()
        self.assertEqual(self.selected(), "v2.4.1")

    def test_numeric_order_and_no_minor_rollover(self):
        for tag in ("v2.4.9", "v2.4.10", "v2.3.99"):
            self.git("tag", tag)
        self.commit()
        self.assertEqual(self.selected(), "v2.4.11")

    def test_ignores_candidates_and_unrelated_tags(self):
        for tag in ("v2.4.0", "v3.0.0-rc.1", "unrelated", "v02.4.99"):
            self.git("tag", tag)
        self.commit()
        self.assertEqual(self.selected(), "v2.4.1")

    def test_reuses_tag_after_failed_run(self):
        self.git("tag", "v2.4.1")
        self.assertEqual(self.selected(), "v2.4.1")

    def test_reuses_annotated_tag(self):
        self.git("tag", "-a", "v2.4.1", "-m", "Patch release")
        self.assertEqual(self.selected(), "v2.4.1")

    def test_manual_major_becomes_new_baseline(self):
        self.git("tag", "v2.4.1")
        self.commit()
        self.git("tag", "v3.0.0")
        self.commit()
        self.assertEqual(self.selected(), "v3.0.1")

    def test_old_commit_does_not_reuse_old_version(self):
        self.git("tag", "v2.4.0")
        self.commit()
        self.git("tag", "v2.4.1")
        self.git("checkout", "--detach", "v2.4.0")
        self.assertEqual(self.selected(), "v2.4.2")


if __name__ == "__main__":
    unittest.main()
