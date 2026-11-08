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
        for name in ('prune-release-artifacts.py', 'prune-provider-go-cache.py', 'prune-verification-dependencies.py', 'prune-runtime-build-cache.py', 'prune-native-release-caches.py'):
            shutil.copyfile(SCRIPTS / name, maintenance / name)
        (base / 'data/projects/verification-snapshots').mkdir(parents=True)
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

    def test_legacy_idle_account_without_gate_is_cleaned_without_touching_data(self):
        with tempfile.TemporaryDirectory() as tmp:
            base, home, gate, toolchain, preserved = self.fixture(Path(tmp))
            gate.unlink()
            result = self.run_guard(base, home)
            self.assertEqual(result.returncode, 0, result.stderr)
            self.assertFalse(toolchain.exists())
            self.assertEqual(gate.stat().st_uid, gate.parent.stat().st_uid)
            self.assertEqual(gate.stat().st_mode & 0o777, 0o600)
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

    @unittest.skipUnless(os.geteuid() == 0, 'native scheduled root maintenance contract')
    def test_scheduled_snapshot_cleanup_honors_runner_lease(self):
        with tempfile.TemporaryDirectory() as tmp:
            base, home, _, _, preserved = self.fixture(Path(tmp))
            root = base / 'data/projects/verification-snapshots' / ('a' * 64)
            root.mkdir()
            environment = dict(os.environ, GIT_AUTHOR_NAME='Test fixture', GIT_AUTHOR_EMAIL='fixture@localhost',
                               GIT_COMMITTER_NAME='Test fixture', GIT_COMMITTER_EMAIL='fixture@localhost')
            def git(*args):
                return subprocess.check_output(['git', '-C', str(root), *args], env=environment, stderr=subprocess.DEVNULL)
            git('init', '--quiet')
            (root / '.gitignore').write_text('node_modules/\n')
            (root / 'package.json').write_text('{"private":true}')
            (root / 'package-lock.json').write_text('{"lockfileVersion":3,"packages":{"":{}}}')
            (root / 'report.md').write_text('saved verification evidence')
            git('add', '.')
            git('commit', '--quiet', '-m', 'snapshot fixture')
            commit = git('rev-parse', 'HEAD').decode().strip()
            (root / '.git/cuan-verification.json').write_text(json.dumps({'source': str(home), 'source_commit': commit, 'target_commit': commit}))
            gate = root / '.git/cuan-verification.lock'
            gate.touch(mode=0o600)
            modules = root / 'node_modules'
            modules.mkdir()
            (modules / 'dependency.js').write_text('generated dependency')
            os.utime(modules, (time.time()-7200,)*2)
            with gate.open('r+b') as lease:
                fcntl.flock(lease, fcntl.LOCK_EX)
                result = self.run_guard(base, home)
                self.assertEqual(result.returncode, 0, result.stderr)
                self.assertTrue(modules.exists())
            result = self.run_guard(base, home)
            self.assertEqual(result.returncode, 0, result.stderr)
            self.assertFalse(modules.exists())
            self.assertEqual((root / 'report.md').read_text(), 'saved verification evidence')
            self.assertTrue(gate.exists())
            self.assertTrue(all(file.exists() for file in preserved))


if __name__ == '__main__':
    unittest.main()
