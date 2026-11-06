#!/usr/bin/env python3
"""Exercise dependency-only cleanup with real Git, flock and process readers."""
import fcntl
import importlib.util
import json
import os
import shlex
from pathlib import Path
import subprocess
import sys
import tempfile
import time
import unittest
from unittest.mock import patch

spec = importlib.util.spec_from_file_location('pruner', Path(__file__).with_name('prune-verification-dependencies.py'))
pruner = importlib.util.module_from_spec(spec)
spec.loader.exec_module(pruner)


class VerificationDependencyPrunerTest(unittest.TestCase):
    def fixture(self, base):
        snapshots = base / 'verification-snapshots'
        root = snapshots / ('a' * 64)
        root.mkdir(parents=True)
        self.git(root, 'init', '--quiet')
        (root / '.gitignore').write_text('node_modules/\n')
        (root / 'package.json').write_text('{"name":"verification-fixture","private":true}')
        (root / 'package-lock.json').write_text(json.dumps({'lockfileVersion': 3, 'packages': {
            '': {}, 'node_modules/example': {'resolved': 'https://registry.npmjs.org/example/-/example-1.0.0.tgz'}}}))
        (root / 'report.md').write_text('saved evidence')
        self.git(root, 'add', '.')
        self.git(root, 'commit', '--quiet', '-m', 'fixture source')
        commit = self.git(root, 'rev-parse', 'HEAD').decode().strip()
        (root / '.git/cuan-verification.json').write_text(json.dumps({
            'source': str(base / 'source'), 'source_commit': commit, 'target_commit': commit}))
        gate = root / '.git/cuan-verification.lock'
        gate.touch(mode=0o600)
        modules = root / 'node_modules'
        modules.mkdir()
        (modules / 'generated.js').write_text('regenerable dependency')
        old = time.time() - 7200
        os.utime(modules, (old, old))
        proc = base / 'empty-proc'
        proc.mkdir()
        return snapshots, root, modules, gate, proc

    def git(self, root, *args):
        env = dict(os.environ, GIT_AUTHOR_NAME='Test fixture', GIT_AUTHOR_EMAIL='fixture@localhost',
                   GIT_COMMITTER_NAME='Test fixture', GIT_COMMITTER_EMAIL='fixture@localhost')
        return subprocess.check_output(['git', '-C', str(root), *args], env=env, stderr=subprocess.DEVNULL)

    def commit_binding(self, root):
        self.git(root, 'commit', '--quiet', '-am', 'updated fixture')
        commit = self.git(root, 'rev-parse', 'HEAD').decode().strip()
        receipt = root / '.git/cuan-verification.json'
        value = json.loads(receipt.read_text())
        value.update(source_commit=commit, target_commit=commit)
        receipt.write_text(json.dumps(value))

    def test_preview_then_removal_preserves_source_evidence_and_lease_inode(self):
        with tempfile.TemporaryDirectory() as tmp:
            snapshots, root, modules, gate, proc = self.fixture(Path(tmp))
            outside = Path(tmp) / 'outside.txt'
            outside.write_text('keep outside link target')
            (modules / 'external').symlink_to(outside)
            os.utime(modules, (time.time()-7200,)*2)
            inode = gate.stat().st_ino
            report = pruner.prune(snapshots, proc=proc)
            self.assertEqual(len(report['eligible']), 1)
            self.assertTrue(modules.exists())
            report = pruner.prune(snapshots, apply=True, proc=proc)
            self.assertEqual(len(report['removed']), 1)
            self.assertFalse(modules.exists())
            self.assertEqual(outside.read_text(), 'keep outside link target')
            self.assertEqual((root / 'report.md').read_text(), 'saved evidence')
            self.assertEqual(gate.stat().st_ino, inode)
            self.assertEqual(self.git(root, 'status', '--porcelain'), b'')

    def test_live_lease_blocks_cleanup(self):
        with tempfile.TemporaryDirectory() as tmp:
            snapshots, _, modules, gate, proc = self.fixture(Path(tmp))
            with gate.open('r+b') as lease:
                fcntl.flock(lease, fcntl.LOCK_EX)
                report = pruner.prune(snapshots, apply=True, proc=proc)
                self.assertEqual(report['busy'], 1)
                self.assertTrue(modules.exists())

    def test_actual_process_reader_blocks_cleanup_without_cooperative_lease(self):
        with tempfile.TemporaryDirectory() as tmp:
            snapshots, _, modules, _, proc = self.fixture(Path(tmp))
            child = subprocess.Popen([sys.executable, '-c',
                'import sys,time; f=open(sys.argv[1]); print("ready",flush=True); time.sleep(30)',
                str(modules / 'generated.js')], stdout=subprocess.PIPE, text=True)
            try:
                self.assertEqual(child.stdout.readline().strip(), 'ready')
                (proc / str(child.pid)).symlink_to(Path('/proc') / str(child.pid), target_is_directory=True)
                (proc / str(os.getpid())).symlink_to(Path('/proc') / str(os.getpid()), target_is_directory=True)
                self.assertEqual(pruner.prune(snapshots, apply=True, proc=proc)['busy'], 1)
                self.assertTrue(modules.exists())
            finally:
                child.terminate()
                child.wait(timeout=5)
                child.stdout.close()
            # The pruner's own gate descriptor must not make every tree busy.
            self.assertEqual(len(pruner.prune(snapshots, apply=True, proc=proc)['removed']), 1)

    def test_unproven_or_changed_snapshots_are_preserved(self):
        for scenario in ('missing gate', 'linked gate', 'linked modules', 'linked metadata',
                         'fresh dependencies', 'dirty source', 'unknown saved file',
                         'tracked dependency', 'unsupported lock', 'wrong commit', 'invalid receipt'):
            with self.subTest(scenario=scenario), tempfile.TemporaryDirectory() as tmp:
                snapshots, root, modules, gate, proc = self.fixture(Path(tmp))
                if scenario == 'missing gate':
                    gate.unlink()
                elif scenario == 'linked gate':
                    gate.unlink()
                    gate.symlink_to(root / 'report.md')
                elif scenario == 'linked modules':
                    modules.rename(root / 'saved-dependencies')
                    modules.symlink_to(root / 'saved-dependencies', target_is_directory=True)
                elif scenario == 'linked metadata':
                    metadata = root / '.git'
                    metadata.rename(root / 'saved-git')
                    metadata.symlink_to(root / 'saved-git', target_is_directory=True)
                elif scenario == 'fresh dependencies':
                    os.utime(modules, None)
                elif scenario == 'dirty source':
                    (root / 'report.md').write_text('new saved work')
                elif scenario == 'unknown saved file':
                    (root / 'pending.txt').write_text('uncommitted work')
                elif scenario == 'tracked dependency':
                    self.git(root, 'add', '--force', 'node_modules/generated.js')
                    self.commit_binding(root)
                elif scenario == 'unsupported lock':
                    lock = root / 'package-lock.json'
                    lock.write_text('{"lockfileVersion":1}')
                    self.commit_binding(root)
                elif scenario == 'wrong commit':
                    receipt = root / '.git/cuan-verification.json'
                    value = json.loads(receipt.read_text())
                    value['target_commit'] = 'b' * 40
                    receipt.write_text(json.dumps(value))
                else:
                    (root / '.git/cuan-verification.json').write_text('not json')
                self.assertEqual(pruner.prune(snapshots, apply=True, proc=proc)['removed'], [])
                self.assertTrue(modules.exists())

    def test_unknown_reader_and_nested_mount_are_not_safe_to_prune(self):
        with tempfile.TemporaryDirectory() as tmp:
            snapshots, _, modules, _, proc = self.fixture(Path(tmp))
            process = proc / '1234'
            process.mkdir()
            command = process / 'cmdline'
            original = Path.read_bytes
            def read(path):
                if path == command:
                    raise PermissionError('reader cannot be inspected')
                return original(path)
            with patch.object(Path, 'read_bytes', read):
                self.assertEqual(pruner.prune(snapshots, apply=True, proc=proc)['busy'], 1)
            self.assertTrue(modules.exists())
            process.rmdir()
            with patch.object(pruner, 'mounted_dependency', return_value=True):
                self.assertEqual(pruner.prune(snapshots, apply=True, proc=proc)['removed'], [])
            self.assertTrue(modules.exists())

    @unittest.skipUnless(os.geteuid() == 0, 'native root maintenance contract')
    def test_root_maintenance_checks_only_the_explicit_service_owned_repository(self):
        with tempfile.TemporaryDirectory() as tmp:
            snapshots, root, modules, gate, proc = self.fixture(Path(tmp))
            (root / '.gitattributes').write_text('report.md filter=maintenance-contract\n')
            self.git(root, 'add', '.gitattributes')
            self.commit_binding(root)
            marker = root / '.git/filter-uid'
            self.git(root, 'config', 'filter.maintenance-contract.clean', 'id -u > ' + shlex.quote(str(marker)) + '; cat')
            os.utime(root / 'report.md', None)
            Path(tmp).chmod(0o755)
            snapshots.chmod(0o755)
            for path in (root, root / '.git', gate, modules, modules / 'generated.js'):
                os.chown(path, 65534, 65534)
            report = pruner.prune(snapshots, apply=True, proc=proc)
            self.assertEqual(len(report['removed']), 1)
            self.assertFalse(modules.exists())
            self.assertEqual(marker.read_text().strip(), '65534')


if __name__ == '__main__':
    unittest.main()
