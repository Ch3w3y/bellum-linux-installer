#!/usr/bin/env python3
"""Run the actual workflow shell step against successful and failing checkers."""
import os
from pathlib import Path
import subprocess
import tempfile
import textwrap
import unittest

WORKFLOW = Path(__file__).resolve().parents[1] / '.github/workflows/update-pins.yml'


def step_script(name):
    text = WORKFLOW.read_text()
    step = text.split('      - name: ' + name + '\n', 1)[1].split('\n      - ', 1)[0]
    return textwrap.dedent(step.split('        run: |\n', 1)[1])


class PinCheckStatusTests(unittest.TestCase):
    def run_checker(self, status, writes_summary=True):
        with tempfile.TemporaryDirectory() as root:
            root = Path(root)
            checker = root / 'pinupdate'
            checker.write_text('#!/bin/bash\n' + (
                'printf "verified candidate\\n" > "$RUNNER_TEMP/pins.md"\n'
                if writes_summary else '') + f'exit {status}\n')
            checker.chmod(0o700)
            go = root / 'go'
            go.write_text('#!/bin/bash\nexit 0\n')
            go.chmod(0o700)
            output, summary = root / 'output', root / 'summary'
            env = dict(os.environ, RUNNER_TEMP=str(root), GITHUB_OUTPUT=str(output),
                       GITHUB_STEP_SUMMARY=str(summary), PATH=str(root) + ':' + os.environ['PATH'])
            run = subprocess.run(['bash', '-e', '-o', 'pipefail', '-c',
                                  step_script('Check upstream releases')], env=env,
                                 capture_output=True, text=True)
            return run.returncode, output.read_text(), summary.read_text() if summary.exists() else ''

    def test_success(self):
        status, output, summary = self.run_checker(0)
        self.assertEqual(status, 0)
        self.assertIn('status=0', output)
        self.assertIn('verified candidate', summary)

    def test_verification_failure_is_not_swallowed(self):
        status, output, summary = self.run_checker(2)
        self.assertEqual(status, 2)
        self.assertIn('status=2', output)
        self.assertIn('verified candidate', summary)

    def test_crash_without_summary_still_fails(self):
        status, output, _ = self.run_checker(1, writes_summary=False)
        self.assertNotEqual(status, 0)
        self.assertIn('status=1', output)

    def test_failure_without_summary_can_still_be_reported(self):
        with tempfile.TemporaryDirectory() as root:
            root = Path(root)
            gh = root / 'gh'
            gh.write_text('#!/bin/bash\nexit 0\n')
            gh.chmod(0o700)
            env = dict(os.environ, RUNNER_TEMP=str(root),
                       PATH=str(root) + ':' + os.environ['PATH'])
            run = subprocess.run(['bash', '-e', '-o', 'pipefail', '-c',
                                  step_script('Report failed checks')], env=env,
                                 capture_output=True, text=True)
            self.assertEqual(run.returncode, 0, run.stderr)
            self.assertIn('failed before writing a summary',
                          (root / 'pins.md').read_text())

    def test_reporting_and_final_gate_run_after_failure(self):
        text = WORKFLOW.read_text()
        self.assertRegex(text, r'id: pins\n(?:[^\n]*\n)*?        continue-on-error: true')
        report = text.split('      - name: Report failed checks\n', 1)[1]
        self.assertIn("if: always() && (steps.pins.outputs.failed == 'true' || steps.pins.outcome == 'failure')", report)
        gate = text.split('      - name: Fail when upstream verification failed\n', 1)[1]
        self.assertIn("if: always() && steps.pins.outcome == 'failure'", gate)
        self.assertIn('run: exit 1', gate)


if __name__ == '__main__':
    unittest.main()
