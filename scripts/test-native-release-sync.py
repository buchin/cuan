#!/usr/bin/env python3
"""Execute the maintained deployment sync gate without running deployment."""
import os
from pathlib import Path
import subprocess
import tempfile
import unittest

SCRIPT = Path(__file__).with_name('deploy.sh').read_text()
GATE = SCRIPT[SCRIPT.index('local_sha="$(git rev-parse HEAD)"'):SCRIPT.index('if [ ! -d "web/dist" ]')]


class NativeReleaseSyncTest(unittest.TestCase):
    def test_exact_native_context_skips_fetch_and_invalid_context_stops(self):
        for scenario in ('native', 'missing-sha', 'missing-branch', 'wrong-sha', 'wrong-branch', 'ordinary', 'remote-changed'):
            with self.subTest(scenario=scenario), tempfile.TemporaryDirectory() as temporary:
                root = Path(temporary)
                log = root / 'calls'
                stub = root / 'git'
                stub.write_text('''#!/bin/bash
printf '%s\\n' "$*" >> "$SYNC_TEST_LOG"
case "$*" in
 'rev-parse HEAD') printf '%s\\n' "$SYNC_TEST_HEAD";;
 'rev-parse origin/dev') printf '%s\\n' "$SYNC_TEST_REMOTE";;
 'fetch origin dev --quiet') exit 0;;
 *) exit 97;;
esac
''')
                stub.chmod(0o700)
                sha = 'a' * 40
                env = {key: value for key, value in os.environ.items() if not key.startswith('CUAN_NATIVE_RELEASE_SYNC_')}
                env.update(PATH=str(root) + os.pathsep + env['PATH'], SYNC_TEST_LOG=str(log), SYNC_TEST_HEAD=sha, SYNC_TEST_REMOTE=('b' * 40 if scenario == 'remote-changed' else sha))
                if scenario not in ('ordinary', 'remote-changed'):
                    env.update(CUAN_NATIVE_RELEASE_SYNC_SHA=sha, CUAN_NATIVE_RELEASE_SYNC_BRANCH='dev')
                    if scenario == 'missing-sha': del env['CUAN_NATIVE_RELEASE_SYNC_SHA']
                    if scenario == 'missing-branch': del env['CUAN_NATIVE_RELEASE_SYNC_BRANCH']
                    if scenario == 'wrong-sha': env['CUAN_NATIVE_RELEASE_SYNC_SHA'] = 'b' * 40
                    if scenario == 'wrong-branch': env['CUAN_NATIVE_RELEASE_SYNC_BRANCH'] = 'other'
                result = subprocess.run(['bash', '-c', 'set -euo pipefail\napproved_branch=dev\n' + GATE], env=env, text=True, capture_output=True, timeout=5)
                self.assertEqual(result.returncode == 0, scenario in ('native', 'ordinary'), result.stderr)
                calls = log.read_text()
                self.assertEqual('fetch origin dev --quiet' in calls, scenario in ('ordinary', 'remote-changed'))


if __name__ == '__main__':
    unittest.main()
