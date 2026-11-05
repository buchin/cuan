#!/usr/bin/env python3
import fcntl
import importlib.util
from pathlib import Path
import tempfile
import os
from unittest import mock
import unittest

spec = importlib.util.spec_from_file_location('cache_prune', Path(__file__).with_name('prune-provider-go-cache.py'))
module = importlib.util.module_from_spec(spec)
spec.loader.exec_module(module)


class CachePruneTest(unittest.TestCase):
    def fixture(self, root, number=1):
        account = root / ('acct_' + f'{number:032x}')
        account.mkdir()
        (account / '.execution.lock').touch()
        toolchain = account / 'home/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.27.0.linux-amd64'
        (toolchain / 'bin').mkdir(parents=True)
        (toolchain / 'bin/go').write_text('fixture')
        (toolchain / 'VERSION').write_text('go1.27.0')
        credentials = account / 'home/.gemini/credentials.json'
        credentials.parent.mkdir()
        credentials.write_text('private fixture')
        return account, toolchain, credentials

    def test_only_old_idle_toolchains_are_deleted(self):
        with tempfile.TemporaryDirectory() as raw:
            root = Path(raw)
            account, toolchain, credentials = self.fixture(root)
            now = toolchain.stat().st_mtime + 25 * 3600
            self.assertEqual(module.prune(root, now=now, in_use=lambda _: False)['eligible'], 1)
            self.assertTrue(toolchain.exists())
            result = module.prune(root, apply=True, now=now, in_use=lambda _: False)
            self.assertEqual(result['removed'], 1)
            self.assertEqual(credentials.read_text(), 'private fixture')
            self.assertTrue((account / '.execution.lock').exists())

    def test_live_lease_recent_cache_and_running_binary_are_preserved(self):
        with tempfile.TemporaryDirectory() as raw:
            root = Path(raw)
            account, toolchain, _ = self.fixture(root)
            now = toolchain.stat().st_mtime + 25 * 3600
            with (account / '.execution.lock').open('r+') as gate:
                fcntl.flock(gate, fcntl.LOCK_SH)
                self.assertEqual(module.prune(root, apply=True, now=now)['busy_accounts'], 1)
            self.assertEqual(module.prune(root, apply=True, now=now, in_use=lambda _: True)['removed'], 0)
            self.assertEqual(module.prune(root, apply=True, now=now - 24 * 3600, in_use=lambda _: False)['removed'], 0)
            self.assertTrue(toolchain.exists())

    def test_legacy_idle_gate_is_created_by_owner_and_unknown_modules_survive(self):
        with tempfile.TemporaryDirectory() as raw:
            root = Path(raw)
            account, toolchain, _ = self.fixture(root)
            unknown = toolchain.parent / 'private-library@v1.0.0'
            unknown.mkdir()
            (unknown / 'saved.txt').write_text('preserve')
            (account / '.execution.lock').unlink()
            self.assertEqual(module.prune(root, apply=True, now=10**12)['removed'], 1)
            self.assertFalse(toolchain.exists())
            gate = account / '.execution.lock'
            self.assertEqual(gate.stat().st_uid, account.stat().st_uid)
            self.assertEqual(gate.stat().st_mode & 0o777, 0o600)
            self.assertTrue((unknown / 'saved.txt').exists())

    def test_only_owner_can_create_a_missing_gate(self):
        with tempfile.TemporaryDirectory() as raw:
            root = Path(raw)
            account, toolchain, credentials = self.fixture(root)
            gate = account / '.execution.lock'
            gate.unlink()
            with mock.patch.object(module.os, 'geteuid', return_value=account.stat().st_uid + 1):
                self.assertEqual(module.prune(root, apply=True, now=10**12)['removed'], 0)
            self.assertFalse(gate.exists())
            self.assertTrue(toolchain.exists())
            self.assertTrue(credentials.exists())

    def test_new_gate_blocks_runtime_lease_and_symlinks_are_preserved(self):
        with tempfile.TemporaryDirectory() as raw:
            root = Path(raw)
            account, toolchain, credentials = self.fixture(root)
            gate = account / '.execution.lock'
            gate.unlink()
            with module.idle_account(account) as idle:
                self.assertTrue(idle)
                with gate.open('r+') as runtime:
                    with self.assertRaises(BlockingIOError):
                        fcntl.flock(runtime, fcntl.LOCK_SH | fcntl.LOCK_NB)
            gate.unlink()
            gate.symlink_to(credentials)
            self.assertEqual(module.prune(root, apply=True, now=10**12)['removed'], 0)
            self.assertTrue(toolchain.exists())
            self.assertEqual(credentials.read_text(), 'private fixture')

    def test_owner_can_remove_readonly_go_distribution_without_following_symlinks(self):
        with tempfile.TemporaryDirectory() as raw:
            root = Path(raw)
            account, toolchain, credentials = self.fixture(root)
            (toolchain / 'outside').symlink_to(credentials.parent, target_is_directory=True)
            for directory in [toolchain / 'bin', toolchain]:
                os.chmod(directory, 0o555)
            result = module.prune(root, apply=True, now=10**12, in_use=lambda _: False)
            self.assertEqual(result['removed'], 1)
            self.assertFalse(toolchain.exists())
            self.assertEqual(credentials.read_text(), 'private fixture')

    def test_symlinked_parent_is_preserved(self):
        with tempfile.TemporaryDirectory() as raw:
            root = Path(raw)
            account, toolchain, credentials = self.fixture(root)
            outside = root / 'saved-home'
            (account / 'home').rename(outside)
            (account / 'home').symlink_to(outside)
            self.assertEqual(module.prune(root, apply=True, now=10**12, in_use=lambda _: False)['removed'], 0)
            self.assertTrue((outside / 'go/pkg/mod/golang.org' / toolchain.name).exists())
            self.assertTrue((outside / '.gemini/credentials.json').exists())


if __name__ == '__main__':
    unittest.main()
