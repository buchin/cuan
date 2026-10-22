#!/usr/bin/env python3
"""Targeted service setup contracts; no host units or services are changed."""
import importlib.util
import json
import os
from pathlib import Path
import pwd
import tempfile
import unittest
spec=importlib.util.spec_from_file_location('native_service',Path(__file__).with_name('setup-native-service.py'))
service=importlib.util.module_from_spec(spec);spec.loader.exec_module(service)
class NativeServiceContracts(unittest.TestCase):
 def setUp(self):
  self.temp=tempfile.TemporaryDirectory(prefix='cuan-service-contract-');self.addCleanup(self.temp.cleanup)
  self.root=Path(self.temp.name)
 def test_unit_escapes_paths_and_supervises_descendants(self):
  text=service.unit_text('cuan',Path('/home/cuan/app 100% "test"'))
  self.assertIn('100%% \\"test\\"',text)
  self.assertIn('Restart=always',text);self.assertIn('RestartSec=5',text)
  self.assertIn('KillMode=control-group',text)
  self.assertIn('User=cuan',text)
  self.assertIn('WantedBy=multi-user.target',text)
  self.assertNotIn('DATABASE_URL',text)
 def test_unrelated_unit_never_overwritten(self):
  path=self.root/'cuan.service';path.write_text('[Service]\nExecStart=/bin/true\n')
  original=path.read_text()
  with self.assertRaisesRegex(RuntimeError,'unrelated'):
   service.retain_existing(path,service.unit_text('cuan',self.root),True)
  self.assertEqual(path.read_text(),original)
 def test_customized_managed_unit_requires_explicit_replacement(self):
  path=self.root/'cuan.service';text=service.unit_text('cuan',self.root)
  path.write_text(text+'# Administrator customization\n')
  with self.assertRaisesRegex(RuntimeError,'customized'):
   service.retain_existing(path,text,False)
  service.retain_existing(path,text,True)
  self.assertTrue(path.read_text().endswith('customization\n'))
 def test_identical_unit_is_idempotent(self):
  path=self.root/'cuan.service';text=service.unit_text('cuan',self.root)
  path.write_text(text);before=path.stat().st_mtime_ns
  service.retain_existing(path,text,False)
  self.assertEqual(path.stat().st_mtime_ns,before)
 def test_unit_symlink_rejected(self):
  target=self.root/'target';target.write_text('keep')
  path=self.root/'cuan.service';path.symlink_to(target)
  with self.assertRaisesRegex(RuntimeError,'symbolic link'):
   service.retain_existing(path,service.unit_text('cuan',self.root),True)
  self.assertEqual(target.read_text(),'keep')
 def test_private_installation_validation(self):
  account=pwd.getpwuid(os.getuid())
  release=self.root/'releases/r';release.mkdir(parents=True)
  (release/'cuan-server').write_text('fixture')
  (self.root/'current').symlink_to(release)
  launcher=self.root/'cuan';launcher.write_text('fixture');launcher.chmod(0o700)
  config=self.root/'config.json'
  config.write_text(json.dumps({'HOME':str(self.root),'PORT':'8123','DATABASE_URL':'postgres://fixture/unused','CUAN_AUTH_SECRET':'x'*32}))
  config.chmod(0o600)
  saved,_=service.installation(self.root,account.pw_name)
  self.assertEqual(saved['PORT'],'8123')
  config.chmod(0o644)
  with self.assertRaisesRegex(RuntimeError,'0600'):
   service.installation(self.root,account.pw_name)
if __name__=='__main__':unittest.main()
