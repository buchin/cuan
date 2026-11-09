#!/usr/bin/env python3
import fcntl
import importlib.util
from pathlib import Path
import tempfile
import shutil
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
        telemetry = account / 'home/.config/go/telemetry'
        telemetry.mkdir(parents=True)
        (telemetry / 'mode').write_text('off\n')
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

    def test_idle_download_archives_preserve_checksums_locks_and_saved_data(self):
        with tempfile.TemporaryDirectory() as raw:
            root = Path(raw)
            account, _, credentials = self.fixture(root)
            directory = account / 'home/go/pkg/mod/cache/download/golang.org/toolchain/@v'
            directory.mkdir(parents=True)
            archive = directory / 'v0.0.1-go1.27.0.linux-amd64.zip'
            archive.write_bytes(b'fixture downloaded distribution')
            marker = archive.with_suffix('.ziphash')
            marker.write_text('h1:' + 'a' * 43 + '=')
            lock = archive.with_suffix('.lock')
            lock.touch()
            inode = lock.stat().st_ino
            warm = account / 'home/.cache/go-build/object'
            warm.parent.mkdir(parents=True)
            warm.write_text('warm compiled cache')
            now = archive.stat().st_mtime + 25*3600
            preview = module.prune(root, now=now, in_use=lambda _: False)
            self.assertEqual(preview['download_eligible'], 1)
            self.assertTrue(archive.exists())
            result = module.prune(root, apply=True, now=now, in_use=lambda _: False)
            self.assertEqual(result['download_removed'], 1)
            self.assertEqual(result['download_bytes_removed'], len(b'fixture downloaded distribution'))
            self.assertFalse(archive.exists())
            self.assertTrue(marker.exists())
            self.assertEqual(lock.stat().st_ino, inode)
            self.assertEqual(warm.read_text(), 'warm compiled cache')
            self.assertEqual(credentials.read_text(), 'private fixture')

    def test_unproven_or_busy_downloads_are_kept(self):
        for scenario in ('account busy', 'download busy', 'open descriptor', 'fresh',
                         'bad hash', 'missing lock', 'linked archive', 'unknown archive'):
            with self.subTest(scenario=scenario), tempfile.TemporaryDirectory() as raw:
                root = Path(raw)
                account, _, credentials = self.fixture(root)
                directory = account / 'home/go/pkg/mod/cache/download/golang.org/toolchain/@v'
                directory.mkdir(parents=True)
                archive = directory / 'v0.0.1-go1.27.0.linux-amd64.zip'
                archive.write_bytes(b'fixture')
                marker = archive.with_suffix('.ziphash')
                marker.write_text('h1:' + 'a'*43 + '=')
                lock = archive.with_suffix('.lock')
                lock.touch()
                now = archive.stat().st_mtime + 25*3600
                held = None
                if scenario == 'account busy':
                    held = (account / '.execution.lock').open('r+')
                    fcntl.flock(held, fcntl.LOCK_SH)
                elif scenario == 'download busy':
                    held = lock.open('r+')
                    fcntl.flock(held, fcntl.LOCK_EX)
                elif scenario == 'open descriptor':
                    held = archive.open('rb')
                elif scenario == 'fresh':
                    os.utime(archive, (now, now))
                elif scenario == 'bad hash':
                    marker.write_text('unproven artifact')
                elif scenario == 'missing lock':
                    lock.unlink()
                elif scenario == 'linked archive':
                    archive.unlink()
                    archive.symlink_to(credentials)
                else:
                    archive.rename(directory / 'private-library.zip')
                try:
                    result = module.prune(root, apply=True, now=now,
                                          in_use=module.process_uses if scenario == 'open descriptor' else lambda _: False)
                    self.assertEqual(result['download_removed'], 0)
                    self.assertTrue(archive.exists() or (directory / 'private-library.zip').exists())
                    self.assertEqual(credentials.read_text(), 'private fixture')
                finally:
                    if held is not None:
                        held.close()

    def test_official_module_cleanup_keeps_credentials_and_busy_accounts(self):
        with tempfile.TemporaryDirectory() as raw:
            root = Path(raw)
            account, toolchain, credentials = self.fixture(root)
            cache = account / 'home/go/pkg/mod'
            now = cache.stat().st_mtime + 25 * 3600
            with (account / '.execution.lock').open('r+b') as held:
                fcntl.flock(held, fcntl.LOCK_SH)
                result = module.prune(root, apply=True, now=now, in_use=lambda _: False, all_modules=True)
                self.assertEqual(result['module_caches_removed'], 0)
                self.assertTrue(toolchain.exists())
            result = module.prune(root, apply=True, now=now, in_use=lambda _: True, all_modules=True)
            self.assertEqual(result['module_caches_removed'], 0)
            result = module.prune(root, apply=False, now=now, in_use=lambda _: False, all_modules=True)
            self.assertEqual(result['module_caches_eligible'], 1)
            self.assertTrue(toolchain.exists())
            with mock.patch.object(module, 'GO_BINARY', shutil.which('go')):
                result = module.prune(root, apply=True, now=now, in_use=lambda _: False, all_modules=True)
            self.assertEqual(result['module_caches_removed'], 1)
            self.assertFalse(cache.exists())
            self.assertEqual(credentials.read_text(), 'private fixture')
            self.assertTrue((account / '.execution.lock').exists())


if __name__ == '__main__':
    unittest.main()
