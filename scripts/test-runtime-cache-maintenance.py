"""Exercise pinned tools and installed units without touching host services."""
import importlib.util
from pathlib import Path
import tempfile
import unittest

spec = importlib.util.spec_from_file_location('maintenance_install', Path(__file__).with_name('install-runtime-cache-maintenance.py'))
module = importlib.util.module_from_spec(spec)
spec.loader.exec_module(module)

class MaintenanceInstall(unittest.TestCase):
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
            self.assertNotIn('/usr/local/bin/cuan-disk-guard', service)
            self.assertEqual(service.count('ExecStart='), 3)
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

if __name__ == '__main__':
    unittest.main()
