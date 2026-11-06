#!/usr/bin/env python3
import fcntl
import importlib.util
import os
from pathlib import Path
import tempfile
import unittest
from unittest.mock import patch

spec = importlib.util.spec_from_file_location('runtime_pruner', Path(__file__).with_name('prune-runtime-go-cache.py'))
module = importlib.util.module_from_spec(spec)
spec.loader.exec_module(module)


class RuntimeCacheTest(unittest.TestCase):
    def fixture(self, root):
        owner = root / ('a' * 64)
        cache = owner / 'go-build'
        artifact = cache / 'aa' / ('b' * 64 + '-d')
        artifact.parent.mkdir(parents=True)
        artifact.write_text('compiled artifact')
        (cache / 'README').write_text('This directory holds cached build artifacts from the Go build system.')
        gate = owner / '.build-cache.lock'
        gate.touch(mode=0o600)
        protected = [owner / 'go/pkg/mod/source.go', owner / 'credentials', root / 'workspace/checkpoint']
        for path in protected:
            path.parent.mkdir(parents=True, exist_ok=True)
            path.write_text('preserve')
        return owner, cache, artifact, gate, protected

    def test_pressure_preview_active_lease_and_actual_cleaner_boundary(self):
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp)
            owner, cache, artifact, gate, protected = self.fixture(root)
            cleaned = []
            def cleaner(path, identity):
                self.assertEqual(path, cache)
                self.assertEqual(identity.st_uid, owner.stat().st_uid)
                cleaned.append(path)
                artifact.unlink()
            args = dict(free=lambda _: 2 * module.GIB, in_use=lambda _: False, cleaner=cleaner)
            self.assertEqual(module.prune(root, **args)['eligible'], 1)
            self.assertTrue(artifact.exists())
            with gate.open('r+') as lease:
                fcntl.flock(lease, fcntl.LOCK_SH)
                self.assertEqual(module.prune(root, apply=True, **args)['busy'], 1)
            self.assertTrue(artifact.exists())
            self.assertEqual(module.prune(root, apply=True, **args)['removed'], 1)
            self.assertEqual(cleaned, [cache])
            for path in protected:
                self.assertEqual(path.read_text(), 'preserve')
            self.assertTrue(gate.exists())

    def test_healthy_disk_and_unknown_reader_do_not_clear_warm_cache(self):
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp)
            _, _, artifact, _, _ = self.fixture(root)
            def forbidden(*_):
                self.fail('cleaner called without idle pressure evidence')
            result = module.prune(root, apply=True, free=lambda _: 3 * module.GIB, cleaner=forbidden)
            self.assertFalse(result['pressure'])
            result = module.prune(root, apply=True, free=lambda _: 1, in_use=lambda _: True, cleaner=forbidden)
            self.assertEqual(result['busy'], 1)
            self.assertTrue(artifact.exists())

    def test_missing_and_linked_legacy_gate_and_linked_cache_preserved(self):
        for kind in ['missing gate', 'linked gate', 'linked cache']:
            with self.subTest(kind=kind), tempfile.TemporaryDirectory() as temp:
                root = Path(temp)
                owner, cache, artifact, gate, _ = self.fixture(root)
                if kind == 'missing gate':
                    gate.unlink()
                elif kind == 'linked gate':
                    gate.unlink()
                    gate.symlink_to(owner / 'credentials')
                else:
                    cache.rename(owner / 'saved-cache')
                    cache.symlink_to(owner / 'saved-cache')
                result = module.prune(root, apply=True, free=lambda _: 1, in_use=lambda _: False)
                self.assertEqual(result['removed'], 0)
                self.assertEqual((owner / 'credentials').read_text(), 'preserve')
                self.assertTrue(artifact.exists())

    def test_legacy_process_environment_is_protected(self):
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp)
            process = root / '123456789'
            process.mkdir()
            (process / 'environ').write_bytes(b'GOCACHE=/runtime/owner/go-build\0')
            (process / 'cmdline').write_bytes(b'go\0test\0')
            self.assertTrue(module.referenced(Path('/runtime/owner'), root))

    def test_native_go_command_is_isolated_and_drops_privileges(self):
        identity = type('Identity', (), dict(st_uid=1234, st_gid=1235))()
        with patch.object(module.os, 'geteuid', return_value=0), patch.object(module.subprocess, 'run') as run:
            module.clean(Path('/runtime/owner/go-build'), identity)
        args, kwargs = run.call_args
        self.assertEqual(args[0], ['/usr/bin/go', 'clean', '-cache'])
        self.assertEqual(kwargs['user'], 1234)
        self.assertEqual(kwargs['extra_groups'], [])
        self.assertEqual(kwargs['cwd'], '/')
        self.assertEqual(kwargs['env']['GOTOOLCHAIN'], 'local')
        self.assertNotIn('HOME', kwargs['env'])


if __name__ == '__main__':
    unittest.main()
