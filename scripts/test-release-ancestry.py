#!/usr/bin/env python3
"""Real Git ancestry proof without a host repository cache or source checkout."""
import importlib.util
import json
import os
from pathlib import Path
import subprocess
import tempfile
import unittest

spec=importlib.util.spec_from_file_location('activation',Path(__file__).with_name('activate-release.py'))
module=importlib.util.module_from_spec(spec);spec.loader.exec_module(module)

class ReleaseAncestryTest(unittest.TestCase):
    def test_real_commit_pack_descendant_divergent_missing_and_corrupt(self):
        with tempfile.TemporaryDirectory() as temporary:
            base=Path(temporary);repo=base/'source';repo.mkdir()
            def git(*args):
                return subprocess.check_output(['git','-C',str(repo),*args],stderr=subprocess.DEVNULL)
            git('init','-q');git('config','user.name','Fixture');git('config','user.email','fixture@example.test')
            (repo/'source').write_text('first');git('add','.');git('commit','-qm','first');first=git('rev-parse','HEAD').decode().strip()
            (repo/'source').write_text('second');git('commit','-qam','second');second=git('rev-parse','HEAD').decode().strip()
            old=base/first;target=base/second;old.mkdir();target.mkdir()
            (old/'release.json').write_text(json.dumps({'sha':first}));(target/'release.json').write_text(json.dumps({'sha':second}))
            objects=git('rev-list','HEAD')
            packed=subprocess.run(['git','-C',str(repo),'pack-objects','--stdout'],input=objects,capture_output=True,check=True).stdout
            pack=target/'source-commits.pack';pack.write_bytes(packed)
            # No base/data/projects/default repository exists. The real parent
            # object graph alone proves containment without trees or checkout.
            module.assert_no_accidental_downgrade(old,target,base)
            (old/'release.json').write_text(json.dumps({'sha':'a'*40}))
            with self.assertRaisesRegex(RuntimeError,'objects are unavailable'):
                module.assert_no_accidental_downgrade(old,target,base)
            module.assert_no_accidental_downgrade(old,target,base,allow_divergent=True)
            (old/'release.json').write_text(json.dumps({'sha':second}));(target/'release.json').write_text(json.dumps({'sha':first}))
            with self.assertRaisesRegex(RuntimeError,'Regression guard'):
                module.assert_no_accidental_downgrade(old,target,base)
            pack.write_bytes(b'corrupt pack')
            with self.assertRaisesRegex(RuntimeError,'could not be verified'):
                module.assert_no_accidental_downgrade(old,target,base)
            pack.unlink()
            with self.assertRaisesRegex(RuntimeError,'history is unavailable'):
                module.assert_no_accidental_downgrade(old,target,base)
            self.assertFalse(list(base.glob('.release-ancestry-*')))

    def test_bound_task_cannot_use_operator_override(self):
        environment = dict(os.environ, CUAN_TASK_CAPABILITY_JSON='fixture')
        result = subprocess.run(['python3', str(Path(__file__).with_name('activate-release.py')),
                                 '--release', 'a'*40, '--allow-divergent'],
                                env=environment, capture_output=True, text=True)
        self.assertEqual(result.returncode, 2)
        self.assertIn('Task executions cannot override release ancestry', result.stderr)

    def test_unknown_metadata_is_not_permission_to_deploy(self):
        with tempfile.TemporaryDirectory() as temporary:
            base=Path(temporary);old=base/'old';target=base/'target';old.mkdir();target.mkdir()
            with self.assertRaisesRegex(RuntimeError,'metadata is unavailable'):
                module.assert_no_accidental_downgrade(old,target,base)

if __name__=='__main__':unittest.main()
