#!/usr/bin/env python3
"""Exercise web/system log maintenance without touching host /var/log or journald."""
import importlib.util
from pathlib import Path
import tempfile
import unittest
from unittest import mock

spec = importlib.util.spec_from_file_location('prune_system_logs', Path(__file__).with_name('prune-system-logs.py'))
module = importlib.util.module_from_spec(spec)
spec.loader.exec_module(module)


class SystemLogsMaintenanceTest(unittest.TestCase):
    def test_dry_run_and_apply_rotate_only_oversized_logs(self):
        with tempfile.TemporaryDirectory() as raw:
            root = Path(raw)
            nginx_dir = root / 'nginx'
            nginx_dir.mkdir()
            (nginx_dir / 'access.log').write_bytes(b'x' * (2 * module.MIB))
            caddy_dir = root / 'caddy'
            caddy_dir.mkdir()
            (caddy_dir / 'access.log').write_bytes(b'small')
            nginx_conf = root / 'nginx.conf'
            nginx_conf.write_text('# logrotate fixture\n')
            caddy_conf = root / 'caddy.conf'
            caddy_conf.write_text('# logrotate fixture\n')
            configs = (('nginx', nginx_dir, nginx_conf), ('caddy', caddy_dir, caddy_conf))
            with mock.patch.object(module.shutil, 'which', side_effect=lambda name, **_: f'/usr/bin/{name}'), \
                 mock.patch.object(module.subprocess, 'run') as run:
                preview = module.maintain(apply=False, max_log_mb=1, log_configs=configs)
                self.assertEqual(preview['eligible'], ['nginx'])
                self.assertEqual(preview['rotated'], [])
                self.assertFalse(preview['journal_vacuumed'])
                run.assert_not_called()

                applied = module.maintain(apply=True, max_log_mb=1, journal_size='50M', log_configs=configs)
                self.assertEqual(applied['eligible'], ['nginx'])
                self.assertEqual(applied['rotated'], ['nginx'])
                self.assertTrue(applied['journal_vacuumed'])
                self.assertEqual(run.call_args_list[0].args[0], ['/usr/bin/journalctl', '--vacuum-size=50M'])
                self.assertEqual(run.call_args_list[1].args[0], ['/usr/bin/logrotate', '-f', str(nginx_conf)])


if __name__ == '__main__':
    unittest.main()
