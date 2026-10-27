#!/usr/bin/env python3
"""Isolated installer contract checks; never starts Docker or modifies the host."""
import os
from pathlib import Path
import shutil
import subprocess
import tempfile
import unittest

ROOT = Path(__file__).resolve().parents[1]

class InstallerTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name)
        (self.root / 'scripts').mkdir()
        (self.root / 'fake').mkdir()
        shutil.copy(ROOT / 'scripts/install.sh', self.root / 'scripts/install.sh')
        shutil.copy(ROOT / '.env.example', self.root / '.env.example')
        self.docker = self.root / 'fake/docker'
        self.docker.write_text('''#!/bin/bash
case "$*" in
  info) exit "${DAEMON_EXIT:-0}";;
  'compose version') exit 0;;
  *'up -d --build') exit "${UP_EXIT:-0}";;
  *'exec -T app'*) exit "${READY_EXIT:-0}";;
esac
exit 0
''')
        self.docker.chmod(0o755)
        sleep = self.root / 'fake/sleep'
        sleep.write_text('#!/bin/sh\nexit 0\n')
        sleep.chmod(0o755)

    def run_install(self, *args, **env):
        values = {**os.environ, 'PATH': str(self.root / 'fake') + ':' + os.environ['PATH'], **env}
        return subprocess.run(['bash', str(self.root / 'scripts/install.sh'), '--skip-preflight', *args], env=values, capture_output=True, text=True)

    def test_dry_run_does_not_write_config(self):
        result = self.run_install('--dry-run')
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertFalse((self.root / '.env').exists())

    def test_fresh_private_config_and_existing_secrets_preserved(self):
        result = self.run_install()
        self.assertEqual(result.returncode, 0, result.stderr)
        config = self.root / '.env'
        original = config.read_bytes()
        self.assertEqual(config.stat().st_mode & 0o777, 0o600)
        self.assertNotIn(b'=replace_with_', original)
        self.assertEqual(self.run_install().returncode, 0)
        self.assertEqual(config.read_bytes(), original)

    def test_unreachable_daemon_does_not_create_config(self):
        self.assertNotEqual(self.run_install(DAEMON_EXIT='1').returncode, 0)
        self.assertFalse((self.root / '.env').exists())

    def test_launch_and_readiness_failures_never_claim_success(self):
        for env in ({'UP_EXIT': '1'}, {'READY_EXIT': '1'}):
            result = self.run_install(**env)
            self.assertNotEqual(result.returncode, 0)
            self.assertNotIn('installation completed successfully', result.stdout)
            self.assertTrue((self.root / '.env').exists())

    def test_existing_placeholders_rejected(self):
        shutil.copy(self.root / '.env.example', self.root / '.env')
        result = self.run_install()
        self.assertNotEqual(result.returncode, 0)
        self.assertIn('placeholder', result.stderr)

    def test_staging_domain_and_port(self):
        result = self.run_install('--staging', '--domain', 'staging.example.com', '--port', '9081')
        self.assertEqual(result.returncode, 0, result.stderr)
        config = (self.root / '.env.staging').read_text()
        self.assertIn('CUAN_STAGING_DOMAIN=staging.example.com', config)
        self.assertIn('PORT=9081', config)

    def test_invalid_arguments(self):
        for args in [('--domain',), ('--email',), ('--port',), ('--port', 'abc'), ('--port', '65536')]:
            self.assertNotEqual(self.run_install(*args).returncode, 0)

if __name__ == '__main__':
    unittest.main()
