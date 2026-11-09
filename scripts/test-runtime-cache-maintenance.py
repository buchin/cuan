"""Exercise pinned tools and installed units without touching host services."""
import importlib.util
from pathlib import Path
import tempfile
import unittest
from unittest import mock
from types import SimpleNamespace
import os
import subprocess
import sys

spec = importlib.util.spec_from_file_location('maintenance_install', Path(__file__).with_name('install-runtime-cache-maintenance.py'))
module = importlib.util.module_from_spec(spec)
spec.loader.exec_module(module)

class MaintenanceInstall(unittest.TestCase):
    def test_backup_retention_is_opt_in_and_survives_later_installs(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            source = root / 'source'
            source.mkdir()
            for name in module.TOOLS:
                (source / name).write_text('# fixture')
            base, home, units = root / 'app', root / 'home', root / 'units'
            module.install(base, home, units, source)
            unit = units / 'cuan-runtime-cache-maintenance.service'
            self.assertNotIn('/prune-native-database-backups.py --base', unit.read_text())
            module.install(base, home, units, source, backup_keep=3)
            self.assertIn('--keep 3 --apply', unit.read_text())
            module.install(base, home, units, source)
            self.assertIn('--keep 3 --apply', unit.read_text())
            config = base / 'maintenance/native-backup-keep'
            config.chmod(0o666)
            with self.assertRaises(ValueError):
                module.install(base, home, units, source)

    def test_payload_manifest_can_be_read_without_root_or_installation(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            script = Path(__file__).with_name('install-runtime-cache-maintenance.py').resolve()
            result = subprocess.run([sys.executable, str(script), '--print-tools'], cwd=root, capture_output=True, text=True, check=True)
            self.assertEqual(result.stdout.splitlines(), list(module.TOOLS))
            self.assertEqual(list(root.iterdir()), [])

    def test_installed_units_keep_original_tools_after_legacy_overwrite(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            source = root / 'source'
            source.mkdir()
            for name in module.TOOLS:
                (source / name).write_text('# safe original ' + name)
            units, base, home = root / 'units', root / 'app', root / 'home'
            pinned = module.install(base, home, units, source)
            self.assertEqual(pinned, module.install(base, home, units, source))
            service = (units / 'cuan-runtime-cache-maintenance.service').read_text()
            self.assertIn(str(pinned) + '/prune-runtime-build-cache.py', service)
            self.assertIn('--kind npm --legacy-accounts ' + str(home), service)
            self.assertIn('--kind composer --apply', service)
            self.assertNotIn('/usr/local/bin/cuan-disk-guard', service)
            self.assertEqual(service.count('ExecStart='), 9)
            for snapshots in ('data/projects/verification-snapshots', 'data/workspaces/verification'):
                self.assertIn('--snapshots ' + str(base / snapshots) + ' --under-storage-pressure --apply', service)
                self.assertIn('/compact-verification-source.py --snapshots ' + str(base / snapshots) + ' --apply', service)
            self.assertIn('--under-storage-pressure --apply', service)
            self.assertIn('OnUnitInactiveSec=2min', (units / 'cuan-runtime-cache-maintenance.timer').read_text())
            (source / module.TOOLS[0]).write_text('# legacy overwrite')
            self.assertTrue((pinned / module.TOOLS[0]).read_text().startswith('# safe original'))
            newer = module.install(base, home, units, source)
            self.assertNotEqual(newer, pinned)
            self.assertTrue(pinned.is_dir())
            self.assertIn(str(newer), (units / 'cuan-runtime-cache-maintenance.service').read_text())

    def test_linked_sources_and_corrupt_pinned_tools_are_rejected(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            source = root / 'source'
            source.mkdir()
            for name in module.TOOLS:
                (source / name).write_text('# original')
            pinned = module.install(root / 'app', root / 'home', root / 'units', source)
            (pinned / module.TOOLS[0]).chmod(0o600)
            (pinned / module.TOOLS[0]).write_text('# corrupt')
            with self.assertRaises(ValueError):
                module.install(root / 'app', root / 'home', root / 'units', source)
            (source / module.TOOLS[0]).unlink()
            (source / module.TOOLS[0]).symlink_to(source / module.TOOLS[1])
            with self.assertRaises(ValueError):
                module.install(root / 'app', root / 'home', root / 'units', source)

    def test_trigger_permission_allows_only_the_fixed_worker_and_preserves_prior_on_failure(self):
        with tempfile.TemporaryDirectory() as directory:
            home, rules = Path(directory) / 'home', Path(directory) / 'sudoers'
            home.mkdir()
            with mock.patch.object(module.pwd, 'getpwuid', return_value=SimpleNamespace(pw_name='cuan_service')), mock.patch.object(module.subprocess, 'run') as validate:
                module.install_trigger_access(SimpleNamespace(stat=lambda: SimpleNamespace(st_uid=1000)), rules)
                rule = rules / 'cuan-storage-maintenance'
                expected = 'cuan_service ALL=(root) NOPASSWD: /usr/bin/systemctl start cuan-runtime-cache-maintenance.service\n'
                self.assertEqual(rule.read_text(), expected)
                self.assertEqual(rule.stat().st_mode & 0o777, 0o440)
                self.assertEqual(validate.call_args.args[0][:2], ['/usr/sbin/visudo', '-cf'])
                validate.side_effect = module.subprocess.CalledProcessError(1, 'visudo')
                with self.assertRaises(module.subprocess.CalledProcessError):
                    module.install_trigger_access(SimpleNamespace(stat=lambda: SimpleNamespace(st_uid=1000)), rules)
                self.assertEqual(rule.read_text(), expected)

if __name__ == '__main__':
    unittest.main()
