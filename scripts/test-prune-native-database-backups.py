#!/usr/bin/env python3
import fcntl
import importlib.util
import os
from pathlib import Path
import subprocess
import tempfile
import unittest
from unittest.mock import patch

spec = importlib.util.spec_from_file_location('retention', Path(__file__).with_name('prune-native-database-backups.py'))
module = importlib.util.module_from_spec(spec)
spec.loader.exec_module(module)


class NativeBackupRetentionTest(unittest.TestCase):
    now = 1800000000

    def fixture(self, root):
        (root / 'backups').mkdir()
        (root / 'releases').mkdir()
        (root / '.release.lock').touch(mode=0o600)
        for number, link in ((1, 'current'), (2, 'previous')):
            target = root / 'releases' / f'{number:040x}'
            target.mkdir()
            (root / link).symlink_to(target)
        paths = []
        for number in range(1, 81):
            path = root / 'backups' / f'{number:040x}.dump'
            path.write_bytes(b'PGDMP' + b'fixture' * 100)
            path.chmod(0o600)
            # Four backups daily; none newer than 24 hours in this fixture.
            when = self.now - (22 - number / 4) * 86400
            os.utime(path, (when, when))
            paths.append(path)
        return paths

    def test_preview_then_apply_preserves_three_newest_and_rollback_points(self):
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp)
            paths = self.fixture(root)
            entries, eligible, _ = module.plan(root)
            expected = {path for path, _ in eligible}
            preview = module.prune(root)
            self.assertGreater(preview['eligible'], 0)
            self.assertTrue(all(path.exists() for path in paths))
            for number in (1, 2, 78, 79, 80):
                self.assertNotIn(paths[number-1], expected)
            validated = []
            result = module.prune(root, apply=True, validate=validated.append)
            self.assertEqual(result['removed'], len(expected))
            self.assertEqual({path for path in paths if not path.exists()}, expected)
            self.assertEqual(set(validated), expected | {entries[0][0], entries[1][0]})

    def test_active_release_and_failed_recent_restore_probe_delete_nothing(self):
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp)
            paths = self.fixture(root)
            with (root / '.release.lock').open('r+') as gate:
                fcntl.flock(gate, fcntl.LOCK_EX)
                self.assertEqual(module.prune(root, apply=True)['skipped'], 'release_busy')
            def broken(_):
                raise subprocess.CalledProcessError(1, 'pg_restore')
            with self.assertRaises(subprocess.CalledProcessError):
                module.prune(root, apply=True, validate=broken)
            self.assertTrue(all(path.exists() for path in paths))

    def test_unknown_linked_public_and_changed_files_are_retained(self):
        for kind in ('symlink', 'hardlink', 'public', 'unknown', 'recent', 'changed'):
            with self.subTest(kind=kind), tempfile.TemporaryDirectory() as temp:
                root = Path(temp)
                paths = self.fixture(root)
                path = paths[2]
                if kind == 'symlink':
                    path.unlink()
                    path.symlink_to(paths[0])
                elif kind == 'hardlink':
                    os.link(path, root / 'saved')
                elif kind == 'public':
                    path.chmod(0o644)
                elif kind == 'unknown':
                    path.write_bytes(b'forensic data')
                elif kind == 'recent':
                    os.utime(path, (self.now, self.now))
                def validate(target):
                    if kind == 'changed' and target == path:
                        path.write_bytes(b'PGDMPchanged')
                if kind == 'changed':
                    with self.assertRaises(ValueError):
                        module.prune(root, apply=True, validate=validate)
                else:
                    module.prune(root, apply=True, validate=validate)
                self.assertTrue(path.exists())

    def test_validation_only_lists_dump_with_no_ambient_database_credentials(self):
        with patch.object(module.subprocess, 'run') as run:
            module.validate_dump(Path('/private/backup.dump'))
        args, kwargs = run.call_args
        self.assertEqual(args[0], ['pg_restore', '--list', '/private/backup.dump'])
        self.assertEqual(set(kwargs['env']), {'PATH', 'LC_ALL'})


if __name__ == '__main__':
    unittest.main()
