import importlib.util
import json
import os
from pathlib import Path
import tempfile
import time
import unittest
spec=importlib.util.spec_from_file_location('prune',Path(__file__).with_name('prune-release-artifacts.py'))
prune=importlib.util.module_from_spec(spec);spec.loader.exec_module(prune)
class ReleaseRetention(unittest.TestCase):
 def test_protects_references_recent_payloads_and_unknown_directories(self):
  with tempfile.TemporaryDirectory() as tmp:
   base=Path(tmp);releases=base/'releases';releases.mkdir();now=time.time()
   paths=[]
   for i in range(8):
    p=releases/format(i,'040x');p.mkdir();(p/'release.json').write_text(json.dumps({'sha':p.name}));os.utime(p,(now-200000+i,now-200000+i));paths.append(p)
   (base/'current').symlink_to(paths[0]);(base/'previous').symlink_to(paths[1])
   unknown=releases/'manual';unknown.mkdir()
   selected=prune.candidates(base,keep=2,now=now)
   self.assertEqual(set(selected),set(paths[2:6]))
   os.utime(paths[2],(now,now))
   self.assertNotIn(paths[2],prune.candidates(base,keep=2,now=now))
   self.assertTrue(unknown.exists())
 def test_missing_rollback_link_fails_closed(self):
  with tempfile.TemporaryDirectory() as tmp:
   with self.assertRaises(RuntimeError):prune.candidates(Path(tmp))
unittest.main()
