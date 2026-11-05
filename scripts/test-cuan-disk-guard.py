#!/usr/bin/env python3
"""Exercise the scheduled entry point with real pruners and a live account gate."""
import fcntl
import json
import os
from pathlib import Path
import shutil
import subprocess
import tempfile
import time
import unittest

SCRIPTS = Path(__file__).resolve().parent


class DiskGuardTest(unittest.TestCase):
    def fixture(self, directory):
        base, home = directory / 'installation', directory / 'service home'
        maintenance = base / 'maintenance'
        maintenance.mkdir(parents=True)
        for name in ('prune-release-artifacts.py', 'prune-provider-go-cache.py'):
            shutil.copyfile(SCRIPTS / name, maintenance / name)
        releases = base / 'releases'
        releases.mkdir()
        for number in range(2):
            release = releases / f'{number:040x}'
            release.mkdir()
            (release / 'release.json').write_text(json.dumps({'sha': release.name}))
            (base / ('current' if number == 0 else 'previous')).symlink_to(release)
        account = home / '.local/share/cuan/accounts' / ('acct_' + 'a' * 32)
        account.mkdir(parents=True)
        gate = account / '.execution.lock'
        gate.touch()
        toolchain = account / 'home/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.27.0.linux-amd64'
        (toolchain / 'bin').mkdir(parents=True)
        (toolchain / 'bin/go').write_text('regenerable distribution')
        (toolchain / 'VERSION').write_text('go1.27.0')
        old = time.time() - 25 * 3600
        os.utime(toolchain, (old, old))
        preserved = [account / 'home/.cache/go-build/object',
                     account / 'home/.gemini/antigravity-cli/conversations/history',
                     account / 'home/.gemini/credentials.json',
                     home / '.local/share/cuan-releases/operator-server',
                     base / 'data/workspaces/pending/checkpoint']
        for file in preserved:
            file.parent.mkdir(parents=True, exist_ok=True)
            file.write_text('saved fixture')
            os.utime(file, (old, old))
        history_age = time.time() - 14 * 86400
        os.utime(preserved[1], (history_age, history_age))
        return base, home, gate, toolchain, preserved

    def run_guard(self, base, home):
        return subprocess.run(['bash', str(SCRIPTS / 'cuan-disk-guard.sh'),
                               '--base', str(base), '--home', str(home)],
                              capture_output=True, text=True, timeout=10)

    def test_active_gate_and_all_saved_data_survive_scheduled_cleanup(self):
        with tempfile.TemporaryDirectory() as tmp:
            base, home, gate, toolchain, preserved = self.fixture(Path(tmp))
            with gate.open('r+') as lock:
                fcntl.flock(lock, fcntl.LOCK_SH)
                result = self.run_guard(base, home)
                self.assertEqual(result.returncode, 0, result.stderr)
                self.assertTrue(toolchain.exists())
            result = self.run_guard(base, home)
            self.assertEqual(result.returncode, 0, result.stderr)
            self.assertFalse(toolchain.exists())
            self.assertTrue(gate.exists())
            for file in preserved:
                self.assertEqual(file.read_text(), 'saved fixture')

    def test_missing_rollback_target_stops_cleanup_without_deleting_cache(self):
        with tempfile.TemporaryDirectory() as tmp:
            base, home, _, toolchain, preserved = self.fixture(Path(tmp))
            (base / 'previous').unlink()
            result = self.run_guard(base, home)
            self.assertNotEqual(result.returncode, 0)
            self.assertTrue(toolchain.exists())
            self.assertTrue(all(file.exists() for file in preserved))


if __name__ == '__main__':
    unittest.main()
