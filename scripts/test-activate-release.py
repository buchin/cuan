"""Targeted release safety tests; no system service/database mutations."""
import importlib.util
import json
import tempfile
import subprocess
import unittest
from pathlib import Path
from unittest.mock import patch

spec = importlib.util.spec_from_file_location('activation', Path(__file__).with_name('activate-release.py'))
a = importlib.util.module_from_spec(spec)
spec.loader.exec_module(a)

class Response:
    def __init__(self, body): self.body = body
    def __enter__(self): return self
    def __exit__(self, *args): pass
    def read(self): return self.body

class Gates(unittest.TestCase):
    def test_current_schema_and_exact_html_required_for_old_and_new_code(self):
        with tempfile.TemporaryDirectory() as directory:
            a.BASE = Path(directory)
            html = a.BASE / 'current/web/dist/index.html'
            html.parent.mkdir(parents=True)
            html.write_bytes(b'candidate')
            for schema, status, database, served, expected in [
                (a.SCHEMA_COUNT, 'ready', 'connected', b'candidate', True),
                (a.SCHEMA_COUNT-1, 'ready', 'connected', b'candidate', False),
                (a.SCHEMA_COUNT, 'not_ready', 'connected', b'candidate', False),
                (a.SCHEMA_COUNT, 'ready', 'disconnected', b'candidate', False),
                (a.SCHEMA_COUNT, 'ready', 'connected', b'stale', False),
            ]:
                with self.subTest(schema=schema, status=status, served=served):
                    responses = [Response(b'{}'), Response(json.dumps(dict(migrations_count=schema, status=status, database=database)).encode()), Response(served)]
                    with patch.object(a.urllib.request, 'urlopen', side_effect=responses):
                        self.assertEqual(a.healthy(), expected)
    def test_failed_systemd_restart_enters_rollback_path(self):
        with patch.object(a.subprocess, 'run', side_effect=subprocess.CalledProcessError(1, ['systemctl'])):
            self.assertFalse(a.restart_and_check())
    def test_switch_is_atomic_and_retains_previous_payload(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            old, new = root/'old', root/'new'
            old.mkdir(); new.mkdir()
            a.switch(root/'current', old)
            a.switch(root/'previous', old)
            a.switch(root/'current', new)
            self.assertEqual((root/'current').resolve(), new)
            self.assertEqual((root/'previous').resolve(), old)
            self.assertFalse((root/'current.next').exists())
    def test_waiting_launch_blocks_release_and_rollback(self):
        with patch.object(a,'database_environment',return_value={}), patch.object(a,'database_scalar',side_effect=['t','0','t','0','t','1']):
            with self.assertRaisesRegex(RuntimeError,'Active or waiting work'):
                a.assert_idle_work()
    def test_schema24_without_launch_table_can_upgrade(self):
        with patch.object(a,'assert_no_github_login'), patch.object(a,'database_environment',return_value={}), patch.object(a,'database_scalar',side_effect=['t','0','t','0','f']):
            a.assert_idle_work()
    def test_database_gate_fails_closed(self):
        with patch.object(a,'database_environment',return_value={}), patch.object(a,'database_scalar',side_effect=RuntimeError('unavailable')):
            with self.assertRaises(RuntimeError): a.assert_idle_work()
    def test_pending_github_auth_blocks_release_without_reading_credentials(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            group = root/'cgroups/system.slice/cuan.service'
            group.mkdir(parents=True)
            (group/'cgroup.procs').write_text('100 101')
            for pid, args in [('100', b'/opt/cuan/cuan-server\0'), ('101', b'/private/runtime/gh\0auth\0login\0--web\0')]:
                process = root/'proc'/pid
                process.mkdir(parents=True)
                (process/'cmdline').write_bytes(args)
            with self.assertRaisesRegex(RuntimeError, 'GitHub sign-in is active'):
                a.assert_no_github_login('/system.slice/cuan.service', root/'proc', root/'cgroups')
            (root/'proc/101/cmdline').write_bytes(b'/private/runtime/gh\0auth\0token\0')
            with self.assertRaisesRegex(RuntimeError, 'GitHub sign-in is active'):
                a.assert_no_github_login('/system.slice/cuan.service', root/'proc', root/'cgroups')
            (root/'proc/101/cmdline').unlink()
            a.assert_no_github_login('/system.slice/cuan.service', root/'proc', root/'cgroups')
    def test_github_process_gate_fails_closed_when_state_is_unreadable(self):
        with tempfile.TemporaryDirectory() as directory:
            with self.assertRaisesRegex(RuntimeError, 'Could not verify'):
                a.assert_no_github_login('/system.slice/cuan.service', Path(directory), Path(directory))

    def test_empty_or_public_existing_backup_is_rejected(self):
        with tempfile.TemporaryDirectory() as directory:
            a.BASE = Path(directory)
            backup = a.BASE/'backups/x.dump'
            backup.parent.mkdir(); backup.write_bytes(b'')
            backup.chmod(0o600)
            with self.assertRaises(RuntimeError): a.backup_database('x')
            backup.write_bytes(b'not empty'); backup.chmod(0o644)
            with self.assertRaises(RuntimeError): a.backup_database('x')

if __name__ == '__main__': unittest.main()
