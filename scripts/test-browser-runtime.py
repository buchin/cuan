#!/usr/bin/env python3
"""Browser runtime readiness contracts without host changes or downloads."""
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

spec = importlib.util.spec_from_file_location('browser_runtime', Path(__file__).with_name('setup-browser-runtime.py'))
runtime = importlib.util.module_from_spec(spec)
spec.loader.exec_module(runtime)


class BrowserRuntimeTests(unittest.TestCase):
    def invoke(self, check=True, broken=False, missing=False, low_disk=False):
        with tempfile.TemporaryDirectory() as folder:
            home = Path(folder)
            module = home/'.local/share/cuan-browser'/runtime.PLAYWRIGHT_VERSION/'node_modules/playwright-core'
            module.mkdir(parents=True)
            cli = module/'cli.js'
            if not missing:
                cli.write_text('fixture')
            link = home/'.local/bin/cuan-browser'
            link.parent.mkdir(parents=True)
            link.symlink_to(cli)
            calls = []
            def run(command, **kwargs):
                calls.append(command)
                if '-e' in command and broken:
                    raise subprocess.CalledProcessError(1, command, output='PRIVATE_BROWSER_OUTPUT')
                return subprocess.CompletedProcess(command, 0)
            output, error = io.StringIO(), io.StringIO()
            argv = ['setup-browser-runtime.py', '--user', 'fixture'] + (['--check'] if check else [])
            with patch.object(sys, 'argv', argv), patch.object(runtime.pwd, 'getpwnam', return_value=SimpleNamespace(pw_uid=12345,pw_dir=str(home))), patch.object(runtime.os,'geteuid',return_value=12345), patch.object(runtime.shutil,'which',side_effect=lambda name:'/usr/bin/'+name), patch.object(runtime.shutil,'disk_usage',return_value=SimpleNamespace(free=(1 if low_disk else 8)*1024**3)), patch.object(runtime.subprocess,'run',side_effect=run), contextlib.redirect_stdout(output), contextlib.redirect_stderr(error):
                try:
                    runtime.main()
                    code = 0
                except SystemExit as e:
                    code = e.code
            return code,output.getvalue(),error.getvalue(),calls

    def test_check_renders_without_installing(self):
        code,out,_,calls=self.invoke()
        self.assertEqual(code,0)
        self.assertTrue(json.loads(out)['screenshotsVerified'])
        self.assertEqual(len(calls),1)
        self.assertIn('-e',calls[0])
        self.assertIn('page.screenshot()',calls[0][2])

    def test_installs_pinned_official_cli_and_only_headless_browser(self):
        code,_,_,calls=self.invoke(check=False)
        self.assertEqual(code,0)
        self.assertIn('playwright-core@'+runtime.PLAYWRIGHT_VERSION,calls[0])
        self.assertIn('--ignore-scripts',calls[0])
        self.assertEqual(calls[1][-3:],['install','chromium','--only-shell'])
        self.assertIn('-e',calls[2])
        self.assertEqual(calls[-1][0],'ln')

    def test_render_failure_never_claims_ready_or_exposes_output(self):
        code,out,error,_=self.invoke(broken=True)
        self.assertEqual(code,1)
        self.assertNotIn('browserReady',out)
        self.assertNotIn('PRIVATE_',out+error)

    def test_missing_runtime_and_low_disk_fail_closed(self):
        for options in ({'missing':True},{'check':False,'low_disk':True}):
            code,out,_,calls=self.invoke(**options)
            self.assertEqual(code,1)
            self.assertNotIn('browserReady',out)
            self.assertEqual(calls,[])


if __name__ == '__main__':
    unittest.main()
