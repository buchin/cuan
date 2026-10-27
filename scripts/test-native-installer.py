#!/usr/bin/env python3
"""Targeted release/configuration contracts; disposable files, no AI or database."""
import fcntl
import json
import os
from pathlib import Path
import stat
import subprocess
import sys
import tempfile
import unittest

INSTALLER=Path(__file__).with_name('install-native.py')

class NativeInstallerContracts(unittest.TestCase):
    def setUp(self):
        self.temporary=tempfile.TemporaryDirectory(prefix='cuan-native-contract-')
        self.addCleanup(self.temporary.cleanup)
        self.base=Path(self.temporary.name)
        self.root=self.base/'private app'
        self.binaries=self.base/'bin';self.binaries.mkdir()
        self.web=self.base/'static';self.web.mkdir()
        (self.web/'index.html').write_text('<html>Cuan fixture</html>')
        for name in ['cuan-server','cuan-migrate']:
            path=self.binaries/name
            path.write_text('#!/usr/bin/env python3\nimport os,json\nprint(json.dumps({"fixture":True}))\n')
            path.chmod(0o700)
        self.env={**os.environ,'NATIVE_FIXTURE_DATABASE':'postgres://fixture:private-password@localhost/never_connect'}
    def run_install(self,*extra,env=None):
        return subprocess.run([sys.executable,str(INSTALLER),'--directory',str(self.root),'--database-env','NATIVE_FIXTURE_DATABASE',
                               '--binary-dir',str(self.binaries),'--static-dir',str(self.web),*extra],
                              env=env or self.env,capture_output=True,text=True,timeout=10)
    def install(self):
        result=self.run_install()
        self.assertEqual(result.returncode,0,result.stderr)
        self.assertNotIn('private-password',result.stdout+result.stderr)
    def test_private_install_and_upgrade_preserve_configuration_and_data(self):
        self.install()
        config=(self.root/'config.json').read_bytes()
        self.assertEqual(stat.S_IMODE((self.root/'config.json').stat().st_mode),0o600)
        self.assertEqual(stat.S_IMODE(self.root.stat().st_mode),0o700)
        self.assertFalse(json.loads(config)['CUAN_AUTO_MIGRATE']=='true')
        original=(self.root/'current').resolve()
        (self.root/'data'/'kept').write_text('persistent')
        result=self.run_install(env={**self.env,'NATIVE_FIXTURE_DATABASE':'postgres://changed/ignored'})
        self.assertEqual(result.returncode,0,result.stderr)
        self.assertEqual((self.root/'config.json').read_bytes(),config)
        self.assertEqual((self.root/'data'/'kept').read_text(),'persistent')
        self.assertNotEqual((self.root/'current').resolve(),original)
        self.assertTrue(original.is_dir())
    def test_failed_migration_retains_current_and_hides_secret(self):
        self.install();before=(self.root/'current').resolve();config=(self.root/'config.json').read_bytes()
        binary=self.binaries/'cuan-migrate'
        binary.write_text('#!/usr/bin/env python3\nimport os,sys\nprint(os.environ["DATABASE_URL"]);sys.exit(1)\n')
        result=self.run_install('--migrate')
        self.assertNotEqual(result.returncode,0)
        self.assertEqual((self.root/'current').resolve(),before)
        self.assertEqual((self.root/'config.json').read_bytes(),config)
        self.assertNotIn('private-password',result.stdout+result.stderr)
        self.assertIn('private-password',(self.root/'migration.log').read_text())
        self.assertEqual(stat.S_IMODE((self.root/'migration.log').stat().st_mode),0o600)
        self.assertFalse(list((self.root/'releases').glob('.building-*')))
    def test_malformed_settings_preserved(self):
        self.install();path=self.root/'config.json';path.write_text('{malformed')
        result=self.run_install();self.assertNotEqual(result.returncode,0)
        self.assertEqual(path.read_text(),'{malformed')
    def test_lock_prevents_duplicate_installs(self):
        self.install();before=(self.root/'current').resolve()
        with (self.root/'.install.lock').open('a') as lock:
            fcntl.flock(lock,fcntl.LOCK_EX|fcntl.LOCK_NB)
            result=self.run_install()
        self.assertNotEqual(result.returncode,0);self.assertIn('Another installer',result.stderr)
        self.assertEqual((self.root/'current').resolve(),before)
    def test_dry_run_does_not_create_files(self):
        result=self.run_install('--dry-run')
        self.assertEqual(result.returncode,0,result.stderr);self.assertFalse(self.root.exists())
    def test_launcher_uses_saved_paths_without_shell_evaluation(self):
        self.install()
        binary=self.binaries/'cuan-server'
        binary.write_text('#!/usr/bin/env python3\nimport os,json\nprint(json.dumps({"cwd":os.getcwd(),"static":os.environ["CUAN_STATIC_DIR"],"automaticMigration":os.environ["CUAN_AUTO_MIGRATE"]}))\n')
        self.install()
        result=subprocess.run([str(self.root/'cuan'),'start'],capture_output=True,text=True,timeout=5)
        self.assertEqual(result.returncode,0,result.stderr);data=json.loads(result.stdout)
        self.assertEqual(data['cwd'],str(self.root/'data'))
        self.assertEqual(data['static'],str(self.root/'current/static'))
        self.assertEqual(data['automaticMigration'],'false')

if __name__=='__main__':
    unittest.main()
