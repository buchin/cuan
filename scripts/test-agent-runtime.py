#!/usr/bin/env python3
"""Targeted native runtime readiness contracts; no downloads or provider calls."""
import contextlib
import importlib.util
import io
import json
from pathlib import Path
import subprocess
import sys
import tempfile
from types import SimpleNamespace
import unittest
from unittest.mock import patch

spec = importlib.util.spec_from_file_location('runtime_setup', Path(__file__).with_name('setup-agent-runtime.py'))
runtime = importlib.util.module_from_spec(spec)
spec.loader.exec_module(runtime)


class RuntimeReadinessTests(unittest.TestCase):
    def invoke(self, missing=False, broken=False):
        with tempfile.TemporaryDirectory() as folder:
            home = Path(folder)
            cli = home/'agy'
            cli.write_text('fixture')
            bwrap = home/'bwrap'
            bwrap.write_text('fixture')
            calls = []
            def run(command, **kwargs):
                calls.append(command)
                if command[-1] == 'command -v agy':
                    return subprocess.CompletedProcess(command, 1 if missing else 0, '' if missing else str(cli)+'\n', '')
                if command[-1] == 'command -v bwrap':
                    return subprocess.CompletedProcess(command, 0, str(bwrap)+'\n', '')
                if command[-1] == '--version':
                    if broken:
                        return subprocess.CompletedProcess(command, 1, 'PRIVATE_PROVIDER_OUTPUT', 'PRIVATE_ERROR')
                    return subprocess.CompletedProcess(command, 0, 'fixture-version', '')
                return subprocess.CompletedProcess(command, 0, '', '')
            output, error = io.StringIO(), io.StringIO()
            account = SimpleNamespace(pw_uid=12345, pw_dir=str(home))
            with patch.object(sys, 'argv', ['setup-agent-runtime.py', '--user', 'fixture', '--check']), \
                 patch.object(runtime.pwd, 'getpwnam', return_value=account), \
                 patch.object(runtime.os, 'geteuid', return_value=12345), \
                 patch.object(runtime.subprocess, 'run', side_effect=run), \
                 contextlib.redirect_stdout(output), contextlib.redirect_stderr(error):
                try:
                    runtime.main()
                    code = 0
                except SystemExit as e:
                    code = e.code
            return code, output.getvalue(), error.getvalue(), calls

    def test_ready_requires_sandboxed_cli_without_network_or_account_home(self):
        code, output, _, calls = self.invoke()
        self.assertEqual(code, 0)
        self.assertTrue(json.loads(output)['cliStartsInSandbox'])
        self.assertTrue(all('install-antigravity.py' not in ' '.join(c) for c in calls))
        sandbox = calls[-1]
        self.assertIn('--unshare-net', sandbox)
        self.assertIn('--clearenv', sandbox)
        self.assertIn('/tmp/runtime-home', sandbox)
        self.assertNotIn('--bind', sandbox)
        self.assertNotIn('antigravity-cli', ' '.join(sandbox))
        self.assertIn('--check', calls[0])

    def test_missing_cli_is_not_ready(self):
        code, output, error, calls = self.invoke(missing=True)
        self.assertEqual(code, 1)
        self.assertNotIn('runtimeReady', output)
        self.assertIn('AI CLI is missing', error)
        self.assertFalse(any(c[-1] == '--version' for c in calls))

    def test_cli_failure_is_not_ready_and_output_is_private(self):
        code, output, error, _ = self.invoke(broken=True)
        self.assertEqual(code, 1)
        self.assertNotIn('runtimeReady', output)
        self.assertIn('could not start inside the sandbox', error)
        self.assertNotIn('PRIVATE_', output+error)


if __name__ == '__main__':
    unittest.main()
