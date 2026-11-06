"""Test native authorization independently from workspace activation helpers."""
import importlib.util
import json
from pathlib import Path
import subprocess
import sys
import tempfile
import unittest

spec = importlib.util.spec_from_file_location('native_activation', Path(__file__).with_name('activate-release.py'))
a = importlib.util.module_from_spec(spec)
spec.loader.exec_module(a)

class ReleaseAuthorization(unittest.TestCase):
    def test_legacy_helper_cannot_start_unapproved_release_and_rollback_remains_available(self):
        with tempfile.TemporaryDirectory() as directory:
            base = Path(directory)
            old, candidate, stale = 'a'*40, 'b'*40, 'c'*40
            for sha in (old, candidate, stale):
                (base / 'releases' / sha).mkdir(parents=True)
            units = base / 'units'
            a.authorize_service_releases(base, 'cuan', old, candidate, units)
            gate = base / 'maintenance/activation-policy/verify-authorized-release.py'
            def run(release=None):
                args = [sys.executable, str(gate), '--base', str(base)]
                if release is not None:
                    args += ['--release', release]
                return subprocess.run(args, capture_output=True, text=True)
            # A legacy helper switches current and rewrites release.conf/template,
            # but independent systemd drop-ins retain native authorization.
            (base / 'current').symlink_to(base / 'releases' / stale)
            (units / 'cuan.service.d/release.conf').write_text('[Service]\nExecStart=old helper\n')
            (units / 'cuan-chat@.service').write_text('[Service]\nExecStart=old template\n')
            self.assertNotEqual(run().returncode, 0)
            self.assertIn('has not passed native activation', run(stale).stderr)
            for sha in (old, candidate):
                a.switch(base / 'current', base / 'releases' / sha)
                self.assertEqual(run().returncode, 0)
                self.assertEqual(run(sha).returncode, 0)
            for name in ('cuan', 'cuan-runner', 'cuan-chat@'):
                self.assertTrue((units / (name + '.service.d/90-native-activation-policy.conf')).is_file())
            # A future native authorization retires the formerly allowed release.
            a.authorize_service_releases(base, 'cuan', candidate, stale, units)
            self.assertNotEqual(run(old).returncode, 0)
            self.assertEqual(run(stale).returncode, 0)

    def test_missing_corrupt_linked_or_outside_policy_targets_fail_closed(self):
        with tempfile.TemporaryDirectory() as directory:
            base = Path(directory)
            sha = 'a'*40
            (base / 'releases' / sha).mkdir(parents=True)
            units = base / 'units'
            a.authorize_service_releases(base, 'cuan', sha, sha, units)
            gate = base / 'maintenance/activation-policy/verify-authorized-release.py'
            policy = gate.with_name('authorized.json')
            good = policy.read_bytes()
            def run(release=sha):
                return subprocess.run([sys.executable, str(gate), '--base', str(base), '--release', release], capture_output=True).returncode
            self.assertEqual(run(), 0)
            self.assertNotEqual(run('../'+sha), 0)
            policy.write_text('{bad')
            self.assertNotEqual(run(), 0)
            policy.write_text(json.dumps({'version': 1, 'allowed': sha}))
            self.assertNotEqual(run(), 0)
            policy.unlink()
            other = base / 'other.json'
            other.write_bytes(good)
            policy.symlink_to(other)
            self.assertNotEqual(run(), 0)
            policy.unlink()
            self.assertNotEqual(run(), 0)
            policy.write_bytes(good)
            (base / 'releases' / sha).rmdir()
            outside = base / 'outside'
            outside.mkdir()
            (base / 'releases' / sha).symlink_to(outside)
            self.assertNotEqual(run(), 0)

if __name__ == '__main__':
    unittest.main()
