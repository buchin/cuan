#!/usr/bin/env python3
import fcntl
import importlib.util
from pathlib import Path
import tempfile
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

    def test_unknown_modules_and_missing_gate_are_preserved(self):
        with tempfile.TemporaryDirectory() as raw:
            root = Path(raw)
            account, toolchain, _ = self.fixture(root)
            unknown = toolchain.parent / 'private-library@v1.0.0'
            unknown.mkdir()
            (unknown / 'saved.txt').write_text('preserve')
            (account / '.execution.lock').unlink()
            self.assertEqual(module.prune(root, apply=True, now=10**12)['removed'], 0)
            self.assertTrue(toolchain.exists())
            self.assertTrue((unknown / 'saved.txt').exists())

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
