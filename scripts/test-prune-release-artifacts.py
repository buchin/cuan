import importlib.util
import json
import os
from pathlib import Path
import tempfile
import time
import unittest
import hashlib
import shutil
import subprocess
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
 def test_staging_requires_exact_installed_payload_and_preserves_unknown_files(self):
  with tempfile.TemporaryDirectory() as tmp:
   base=Path(tmp);releases=base/'releases';releases.mkdir();staging=base/'staging';staging.mkdir();now=time.time()
   for i in range(8):
    p=releases/format(i,'040x');p.mkdir();(p/'payload').write_bytes(b'verified binary')
    (p/'release.json').write_text(json.dumps({'sha':p.name,'sha256':{'payload':hashlib.sha256(b'verified binary').hexdigest()}}))
    upload=staging/p.name;shutil.copytree(p,upload);os.utime(upload,(now-7200+i,now-7200+i))
   (base/'current').symlink_to(releases/format(0,'040x'));(base/'previous').symlink_to(releases/format(1,'040x'))
   (staging/format(2,'040x')/'pending-work').write_text('preserve')
   (releases/format(3,'040x')/'payload').write_text('changed installed copy')
   (staging/format(4,'040x')/'payload').unlink();(staging/format(4,'040x')/'payload').symlink_to(releases/format(5,'040x')/'payload')
   for i in range(8):os.utime(staging/format(i,'040x'),(now-7200+i,now-7200+i))
   selected=prune.staging_candidates(base,staging,keep=2,now=now)
   self.assertEqual(selected,[staging/format(5,'040x')])
   (staging/format(5,'040x')/'payload').unlink()
   os.utime(staging/format(5,'040x'),(now-7200+5,now-7200+5))
   self.assertEqual(prune.staging_candidates(base,staging,keep=2,now=now),[staging/format(5,'040x')])
   os.utime(staging/format(5,'040x'),(now,now))
   self.assertEqual(prune.staging_candidates(base,staging,keep=2,now=now,only_names={format(5,'040x')}),[])
class FrequentDeploymentRetention(unittest.TestCase):
 def test_same_day_releases_keep_five_rollback_and_live_process(self):
  with tempfile.TemporaryDirectory() as tmp:
   base=Path(tmp); releases=base/'releases'; releases.mkdir(); now=time.time(); paths=[]
   for i in range(12):
    path=releases/format(i,'040x'); path.mkdir()
    (path/'release.json').write_text(json.dumps({'sha':path.name}))
    os.utime(path,(now-7200+i,now-7200+i)); paths.append(path)
   (base/'current').symlink_to(paths[0]); (base/'previous').symlink_to(paths[1])
   process=subprocess.Popen(['sleep','30'],cwd=paths[2])
   try:
    self.assertEqual(set(prune.candidates(base,now=now)),set(paths[3:7]))
    os.utime(paths[3],(now-1800,now-1800))
    self.assertNotIn(paths[3],prune.candidates(base,now=now))
    self.assertTrue(paths[2].exists())
   finally:
    process.terminate(); process.wait()
unittest.main()
