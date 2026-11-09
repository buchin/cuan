#!/usr/bin/env python3
"""Real Git and OS lease/process tests for accepted-source materialization."""
import datetime
import fcntl
import importlib.util
import json
import os
from pathlib import Path
import shutil
import stat
import subprocess
import sys
import tempfile
import time
import unittest
from unittest import mock

def load(name, filename):
    spec = importlib.util.spec_from_file_location(name, Path(__file__).with_name(filename))
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module

compactor = load('source_compactor', 'compact-verification-source.py')
fixtures = load('cache_fixtures', 'test-prune-verification-dependencies.py')


class SourceCompactorTest(unittest.TestCase):
    git = fixtures.VerificationDependencyPrunerTest.git

    def fixture(self, base):
        snapshots, root, modules, gate, proc = fixtures.VerificationDependencyPrunerTest.fixture(self, base)
        shutil.rmtree(modules)  # Fixture-only regenerable dependency.
        binding = json.loads((root / '.git/cuan-verification.json').read_text())
        accepted = dict(policy='retained-git-source-v1', task_id='accepted-task',
                        source_commit=binding['source_commit'], target_commit=binding['target_commit'],
                        accepted_at=datetime.datetime.fromtimestamp(time.time() - 72 * 3600, datetime.timezone.utc).isoformat(),
                        generated_configs={})
        (root / '.git/cuan-verification-accepted.json').write_text(json.dumps(accepted))
        old = time.time() - 72 * 3600
        os.utime(root, (old, old))
        return snapshots, root, gate, proc

    def test_preview_compaction_keeps_git_objects_reports_and_lease_then_restores(self):
        with tempfile.TemporaryDirectory() as directory:
            snapshots, root, gate, proc = self.fixture(Path(directory))
            before = gate.stat()
            objects = self.git(root, 'count-objects', '-v')
            preview = compactor.compact(snapshots, proc=proc)
            self.assertEqual(preview['eligible'], [root.name])
            self.assertEqual(preview['compacted'], [])
            self.assertTrue((root / 'report.md').exists())
            report = compactor.compact(snapshots, apply=True, proc=proc)
            self.assertEqual(report['compacted'], [root.name])
            self.assertGreater(report['reclaimed_materialized_bytes'], 0)
            self.assertFalse((root / 'report.md').exists())
            self.assertEqual(self.git(root, 'show', 'HEAD:report.md'), b'saved evidence')
            self.assertEqual(self.git(root, 'count-objects', '-v'), objects)
            self.assertEqual(before.st_ino, gate.stat().st_ino)
            self.assertEqual(compactor.compact(snapshots, apply=True, proc=proc)['compacted'], [])
            self.git(root, 'sparse-checkout', 'disable')
            self.assertEqual((root / 'report.md').read_text(), 'saved evidence')

    def test_unaccepted_clean_old_snapshot_with_default_mcp_config_is_compacted_and_restores(self):
        with tempfile.TemporaryDirectory() as directory:
            snapshots, root, gate, proc = self.fixture(Path(directory))
            grant = root / '.git/cuan-verification-accepted.json'
            grant.unlink()
            (root / '.git/info/exclude').write_bytes(b'.agents/\n')
            (root / '.agents').mkdir(exist_ok=True)
            (root / '.agents/mcp_config.json').write_bytes(compactor.DEFAULT_CONFIG)
            old = time.time() - 72 * 3600
            os.utime(root, (old, old))
            before = gate.stat()
            objects = self.git(root, 'count-objects', '-v')

            preview = compactor.compact(snapshots, proc=proc)
            self.assertEqual(preview['eligible'], [root.name])
            self.assertEqual(preview['compacted'], [])
            self.assertTrue((root / 'report.md').exists())

            report = compactor.compact(snapshots, apply=True, proc=proc)
            self.assertEqual(report['compacted'], [root.name])
            self.assertGreater(report['reclaimed_materialized_bytes'], 0)
            self.assertFalse((root / 'report.md').exists())
            self.assertTrue((root / '.git/cuan-verification-cold.json').exists())
            self.assertFalse(grant.exists())
            self.assertEqual((root / '.agents/mcp_config.json').read_bytes(), compactor.DEFAULT_CONFIG)
            self.assertEqual(self.git(root, 'show', 'HEAD:report.md'), b'saved evidence')
            self.assertEqual(self.git(root, 'count-objects', '-v'), objects)
            self.assertEqual(before.st_ino, gate.stat().st_ino)

            self.git(root, 'sparse-checkout', 'disable')
            self.assertEqual((root / 'report.md').read_text(), 'saved evidence')
            self.assertEqual((root / '.agents/mcp_config.json').read_bytes(), compactor.DEFAULT_CONFIG)

    def test_unaccepted_warm_changed_or_unique_work_is_kept(self):
        for mode in ('unaccepted warm', 'unaccepted custom config', 'warm', 'changed', 'unique report', 'ignored unknown', 'wrong binding'):
            with self.subTest(mode=mode), tempfile.TemporaryDirectory() as directory:
                snapshots, root, _, proc = self.fixture(Path(directory))
                grant = root / '.git/cuan-verification-accepted.json'
                if mode == 'unaccepted warm':
                    grant.unlink()
                    os.utime(root, None)
                elif mode == 'unaccepted custom config':
                    grant.unlink()
                    (root / '.git/info/exclude').write_bytes(b'.agents/\n')
                    (root / '.agents').mkdir(exist_ok=True)
                    (root / '.agents/mcp_config.json').write_text('{"mcpServers": {"custom": {}}}')
                elif mode == 'warm':
                    os.utime(root, None)
                elif mode == 'changed':
                    (root / 'report.md').write_text('unique changed source')
                elif mode == 'unique report':
                    (root / 'actual-verification-report.txt').write_text('unique evidence')
                elif mode == 'ignored unknown':
                    (root / 'node_modules').mkdir()
                    (root / 'node_modules/unknown.txt').write_text('unique unknown data')
                else:
                    data = json.loads(grant.read_text())
                    data['target_commit'] = 'b' * 40
                    grant.write_text(json.dumps(data))
                if mode not in ('warm', 'unaccepted warm'):
                    old = time.time() - 72 * 3600
                    os.utime(root, (old, old))
                report = compactor.compact(snapshots, apply=True, proc=proc)
                self.assertEqual(report['compacted'], [])
                self.assertTrue((root / 'report.md').exists())
                self.assertFalse((root / '.git/cuan-verification-cold.json').exists())

    def test_active_lease_and_actual_reader_prevent_compaction(self):
        with tempfile.TemporaryDirectory() as directory:
            snapshots, root, gate, proc = self.fixture(Path(directory))
            with gate.open('r+b') as lease:
                fcntl.flock(lease, fcntl.LOCK_EX)
                self.assertEqual(compactor.compact(snapshots, apply=True, proc=proc)['busy'], 1)
            child = subprocess.Popen([sys.executable, '-c', 'import sys,time;f=open(sys.argv[1]);print("ready",flush=True);time.sleep(30)', str(root / 'report.md')], stdout=subprocess.PIPE, text=True)
            try:
                self.assertEqual(child.stdout.readline().strip(), 'ready')
                (proc / str(child.pid)).symlink_to(Path('/proc') / str(child.pid), target_is_directory=True)
                self.assertEqual(compactor.compact(snapshots, apply=True, proc=proc)['busy'], 1)
                self.assertTrue((root / 'report.md').exists())
            finally:
                child.terminate()
                child.wait(timeout=5)
                child.stdout.close()
            self.assertEqual(compactor.compact(snapshots, apply=True, proc=proc)['compacted'], [root.name])

    def test_existing_foreign_hardlink_with_setgid_is_rejected_without_mutation_under_root(self):
        with tempfile.TemporaryDirectory() as directory:
            snapshots, root, gate, proc = self.fixture(Path(directory))
            gate.unlink()
            outside = Path(directory) / 'outside_sensitive_file'
            outside.write_bytes(b'sensitive payload')
            os.chmod(outside, 0o2755)
            os.link(outside, gate)

            before = outside.stat()
            self.assertEqual(stat.S_IMODE(before.st_mode), 0o2755)
            self.assertEqual(before.st_nlink, 2)

            with mock.patch('os.geteuid', return_value=0), \
                 mock.patch('os.fchown') as mock_fchown, \
                 mock.patch('os.fchmod', create=True) as mock_fchmod:
                report = compactor.compact(snapshots, apply=True, proc=proc)

            mock_fchown.assert_not_called()
            mock_fchmod.assert_not_called()
            self.assertEqual(report['compacted'], [])
            self.assertTrue((root / 'report.md').exists())

            after = outside.stat()
            self.assertEqual(stat.S_IMODE(after.st_mode), 0o2755)
            self.assertEqual(after.st_uid, before.st_uid)
            self.assertEqual(after.st_gid, before.st_gid)
            self.assertEqual(after.st_size, before.st_size)
            self.assertEqual(after.st_ino, before.st_ino)

    def test_missing_gate_creates_new_exclusive_lease_and_compacts(self):
        with tempfile.TemporaryDirectory() as directory:
            snapshots, root, gate, proc = self.fixture(Path(directory))
            gate.unlink()
            self.assertFalse(gate.exists())

            report = compactor.compact(snapshots, apply=True, proc=proc)
            self.assertEqual(report['compacted'], [root.name])
            self.assertTrue(gate.exists())
            info = gate.stat()
            self.assertEqual(stat.S_IMODE(info.st_mode), 0o600)
            self.assertEqual(info.st_nlink, 1)

if __name__ == '__main__':
    unittest.main()
