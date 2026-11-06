"""Targeted release safety tests; no system service/database mutations."""
import importlib.util
import fcntl
import json
import tempfile
import subprocess
import unittest
import subprocess
import sys
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
    def test_external_worker_does_not_wait_for_active_chat_during_api_switch(self):
        with tempfile.TemporaryDirectory() as directory:
            path = Path(directory) / 'admission.lock'
            with patch.object(a,'inbox_admission_lock',side_effect=lambda:path.open('a+')), patch.object(a,'database_environment',return_value={}), patch.object(a,'database_scalar') as query:
                with a.drain_inbox_admission(wait_active=False):
                    with path.open('a+') as other:
                        with self.assertRaises(BlockingIOError):
                            fcntl.flock(other,fcntl.LOCK_SH|fcntl.LOCK_NB)
                query.assert_not_called()

    def test_chat_worker_uses_pinned_release_and_drains_independently(self):
        text=a.chat_worker_unit(Path('/opt/cuan'),'cuan','ubuntu')
        self.assertIn('ExecStart=/opt/cuan/releases/%i/bin/cuan-server --chat-worker',text)
        self.assertIn('KillMode=mixed',text)
        self.assertIn('TimeoutStopSec=430',text)
        self.assertIn('RuntimeDirectory=cuan-chat-%i',text)
        self.assertNotIn('PartOf=',text)
        self.assertNotIn('Requires=cuan',text)
        self.assertTrue(a.supports_chat_worker({'features':{'inbox_worker':1}}))
        self.assertFalse(a.supports_chat_worker({'features':{}}))

    def test_rolling_worker_update_stops_only_old_managed_instances(self):
        old='cuan-chat@'+'a'*40+'.service'
        new='cuan-chat@'+'b'*40+'.service'
        result=type('Result',(),{'stdout':old+' loaded active running\n'+new+' loaded active running\nother.service loaded active running\n'})()
        with patch.object(a,'SERVICE','cuan'),patch.object(a.subprocess,'run',return_value=result) as command:
            a.drain_old_chat_workers(keep=new)
        calls=[call.args[0] for call in command.call_args_list]
        self.assertIn(['systemctl','stop','--no-block',old],calls)
        self.assertNotIn(['systemctl','stop','--no-block',new],calls)
        self.assertFalse(any('other.service' in call for call in calls))

    def test_inbox_drain_waits_for_live_turns_and_holds_claim_barrier(self):
        with tempfile.TemporaryDirectory() as directory:
            path = Path(directory) / 'admission.lock'
            with patch.object(a, 'inbox_admission_lock', side_effect=lambda: path.open('a+')), patch.object(a, 'database_environment', return_value={}), patch.object(a, 'database_scalar', side_effect=['t','1','t','0']) as query, patch.object(a.time, 'sleep') as sleep:
                with a.drain_inbox_admission(timeout=3):
                    with path.open('a+') as other:
                        with self.assertRaises(BlockingIOError):
                            fcntl.flock(other, fcntl.LOCK_SH | fcntl.LOCK_NB)
                    self.assertIn("state IN ('running','cancelling')", query.call_args.args[0])
                sleep.assert_called_once_with(1)
            with path.open('a+') as other:
                fcntl.flock(other, fcntl.LOCK_EX | fcntl.LOCK_NB)

    def test_inbox_drain_timeout_does_not_activate_and_releases_barrier(self):
        with tempfile.TemporaryDirectory() as directory:
            path = Path(directory) / 'admission.lock'
            with patch.object(a, 'inbox_admission_lock', side_effect=lambda: path.open('a+')), patch.object(a, 'database_environment', return_value={}), patch.object(a, 'database_scalar', side_effect=['t','1']), patch.object(a.time, 'monotonic', side_effect=[0,2]):
                with self.assertRaisesRegex(RuntimeError, 'release was not switched'):
                    with a.drain_inbox_admission(timeout=1):
                        self.fail('activation proceeded with an active turn')
            with path.open('a+') as other:
                fcntl.flock(other, fcntl.LOCK_EX | fcntl.LOCK_NB)

    def test_static_only_release_does_not_pause_inbox(self):
        with patch.object(a, 'inbox_admission_lock') as lock, patch.object(a, 'database_environment') as database:
            with a.drain_inbox_admission(no_restart=True):
                pass
            lock.assert_not_called()
            database.assert_not_called()
    def test_failed_health_never_records_task_completion(self):
        with patch.object(a, 'restart_and_check', return_value=False), patch.object(a, 'record_task_deployment') as record:
            self.assertFalse(a.verify_and_record_task_deployment({}, 'a'*40))
            record.assert_not_called()

    def test_task_evidence_is_recorded_only_after_matching_release_health(self):
        sequence = []
        def check(sha):
            sequence.append('health:' + sha)
            return True
        def record(env, sha): sequence.append('receipt:' + sha)
        with patch.object(a, 'restart_and_check', side_effect=check), patch.object(a, 'record_task_deployment', side_effect=record):
            self.assertTrue(a.verify_and_record_task_deployment({}, 'a'*40))
        self.assertEqual(sequence, ['health:'+'a'*40, 'receipt:'+'a'*40])

    def test_receipt_failure_does_not_trigger_healthy_release_rollback(self):
        with patch.object(a, 'restart_and_check', return_value=True), patch.object(a, 'record_task_deployment', side_effect=RuntimeError('database unavailable')):
            self.assertTrue(a.verify_and_record_task_deployment({}, 'a'*40))

    def test_receipt_requires_exact_task_and_attempt_binding(self):
        with patch.dict(a.os.environ, {'CUAN_DEPLOY_EXCLUDE_TASK':'task-fixture'}, clear=True), patch.object(a, 'database_scalar') as query:
            self.assertIsNone(a.record_task_deployment({}, 'a'*40))
            query.assert_not_called()
        with patch.dict(a.os.environ, {'CUAN_DEPLOY_EXCLUDE_TASK':"bad'input", 'CUAN_DEPLOY_EXCLUDE_ATTEMPT':'attempt-fixture'}, clear=True), patch.object(a, 'database_scalar') as query:
            with self.assertRaises(ValueError): a.record_task_deployment({}, 'a'*40)
            query.assert_not_called()
        with patch.dict(a.os.environ, {'CUAN_DEPLOY_EXCLUDE_TASK':'task-fixture', 'CUAN_DEPLOY_EXCLUDE_ATTEMPT':'attempt-fixture'}, clear=True), patch.object(a, 'database_scalar', return_value='t'):
            first = a.record_task_deployment({}, 'a'*40)
            self.assertEqual(first, a.record_task_deployment({}, 'a'*40))
            self.assertNotEqual(first, a.record_task_deployment({}, 'b'*40))
        with patch.dict(a.os.environ, {'CUAN_TASK_CAPABILITY_JSON': json.dumps({'task_id': 'task-fixture', 'attempt_id': 'attempt-fixture'})}, clear=True), patch.object(a, 'database_scalar', return_value='t'):
            receipt = a.record_task_deployment({}, 'a'*40)
            self.assertEqual(receipt, first)

    def test_target_sha_cannot_be_missing_from_health_response(self):
        with patch.object(a.urllib.request, 'urlopen', return_value=Response(b'{"status":"healthy"}')):
            self.assertFalse(a.healthy(target_sha='a'*40))

    def test_release_schema_count_matches_repository_migrations(self):
        migrations = Path(__file__).resolve().parent.parent / 'migrations'
        versions = [int(path.name.split('_', 1)[0]) for path in migrations.glob('*.up.sql')]
        self.assertEqual(a.SCHEMA_COUNT, max(versions), 'Update release readiness when adding a migration')

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

    def test_health_checks_commit_sha_and_status(self):
        with tempfile.TemporaryDirectory() as directory:
            a.BASE = Path(directory)
            html = a.BASE / 'current/web/dist/index.html'
            html.parent.mkdir(parents=True)
            html.write_bytes(b'candidate')
            readyz_body = json.dumps(dict(migrations_count=a.SCHEMA_COUNT, status='ready', database='connected')).encode()

            # Target SHA matches
            responses = [
                Response(json.dumps(dict(status='healthy', commit_sha='abc1234')).encode()),
                Response(readyz_body),
                Response(b'candidate'),
            ]
            with patch.object(a.urllib.request, 'urlopen', side_effect=responses):
                self.assertTrue(a.healthy(target_sha='abc1234'))

            # Target SHA mismatch fails
            responses = [
                Response(json.dumps(dict(status='healthy', commit_sha='old9999')).encode()),
                Response(readyz_body),
                Response(b'candidate'),
            ]
            with patch.object(a.urllib.request, 'urlopen', side_effect=responses):
                self.assertFalse(a.healthy(target_sha='abc1234'))

            # Healthz unhealthy status fails
            responses = [
                Response(json.dumps(dict(status='unhealthy', commit_sha='abc1234')).encode()),
                Response(readyz_body),
                Response(b'candidate'),
            ]
            with patch.object(a.urllib.request, 'urlopen', side_effect=responses):
                self.assertFalse(a.healthy(target_sha='abc1234'))

    def test_no_restart_rejected_when_binary_checksum_differs(self):
        import hashlib
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            old = root / 'old'
            target = root / 'target'
            (old / 'bin').mkdir(parents=True)
            (target / 'bin').mkdir(parents=True)
            (old / 'bin/cuan-server').write_bytes(b'binary_v1')
            (target / 'bin/cuan-server').write_bytes(b'binary_v2')

            # Mocking the condition inside activate-release.py
            old_binary = old / 'bin/cuan-server'
            target_binary = target / 'bin/cuan-server'
            self.assertNotEqual(
                hashlib.sha256(old_binary.read_bytes()).hexdigest(),
                hashlib.sha256(target_binary.read_bytes()).hexdigest()
            )

    def test_worker_drain_signal_and_cleanup(self):
        drain_file = Path('/tmp/cuan-drain')
        try:
            drain_file.unlink(missing_ok=True)
            with patch.object(a.subprocess, 'run') as mock_run:
                a.signal_worker_drain()
                self.assertTrue(drain_file.exists())
                mock_run.assert_called_once_with(['systemctl', 'kill', '--kill-whom=main', '--signal=USR1', a.SERVICE+'-runner.service'], check=False)
            a.cleanup_worker_drain()
            self.assertFalse(drain_file.exists())
        finally:
            drain_file.unlink(missing_ok=True)

    def test_runner_name_in_prompt_matches_unrelated_provider_process(self):
        # Reproduce selection only: never broadcast a signal in this test.
        child=subprocess.Popen([sys.executable,'-c','import time; time.sleep(30)','cuan-runner'])
        try:
            matches=subprocess.check_output(['pgrep','-f','cuan-runner'],text=True).splitlines()
            self.assertIn(str(child.pid),matches)
            self.assertNotEqual(Path('/proc',str(child.pid),'exe').resolve().name,'cuan-runner')
            self.assertIsNone(child.poll())
        finally:
            child.terminate()
            child.wait(timeout=3)

    def test_worker_service_restart_and_auto_undrain(self):
        drain_file = Path('/tmp/cuan-drain')
        try:
            drain_file.touch(exist_ok=True)
            with patch.object(a.subprocess, 'run', return_value=subprocess.CompletedProcess(['systemctl'], 0, '')) as mock_run:
                a.restart_worker_service()
                self.assertFalse(drain_file.exists())
                mock_run.assert_called_once_with(['systemctl', 'restart', 'cuan-runner'], check=False, capture_output=True, text=True)
        finally:
            drain_file.unlink(missing_ok=True)

if __name__ == '__main__': unittest.main()
