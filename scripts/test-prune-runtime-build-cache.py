#!/usr/bin/env python3
import fcntl
import importlib.util
import os
from pathlib import Path
import tempfile
import unittest
import time
import tarfile
import io
import subprocess
import shutil
from unittest.mock import patch

spec = importlib.util.spec_from_file_location('runtime_pruner', Path(__file__).with_name('prune-runtime-build-cache.py'))
module = importlib.util.module_from_spec(spec)
spec.loader.exec_module(module)


class RuntimeCacheTest(unittest.TestCase):
    def fixture(self, root):
        owner = root / ('a' * 64)
        cache = owner / 'go-build'
        artifact = cache / 'aa' / ('b' * 64 + '-d')
        artifact.parent.mkdir(parents=True)
        artifact.write_text('compiled artifact')
        (cache / 'README').write_text('This directory holds cached build artifacts from the Go build system.')
        gate = owner / '.build-cache.lock'
        gate.touch(mode=0o600)
        protected = [owner / 'go/pkg/mod/source.go', owner / 'credentials', root / 'workspace/checkpoint']
        for path in protected:
            path.parent.mkdir(parents=True, exist_ok=True)
            path.write_text('preserve')
        return owner, cache, artifact, gate, protected

    def test_pressure_preview_active_lease_and_actual_cleaner_boundary(self):
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp)
            owner, cache, artifact, gate, protected = self.fixture(root)
            cleaned = []
            def cleaner(path, identity):
                self.assertEqual(path, cache)
                self.assertEqual(identity.st_uid, owner.stat().st_uid)
                cleaned.append(path)
                artifact.unlink()
            args = dict(free=lambda _: 2 * module.GIB, in_use=lambda _: False, cleaner=cleaner)
            self.assertEqual(module.prune(root, **args)['eligible'], 1)
            self.assertTrue(artifact.exists())
            with gate.open('r+') as lease:
                fcntl.flock(lease, fcntl.LOCK_SH)
                self.assertEqual(module.prune(root, apply=True, **args)['busy'], 1)
            self.assertTrue(artifact.exists())
            self.assertEqual(module.prune(root, apply=True, **args)['removed'], 1)
            self.assertEqual(cleaned, [cache])
            for path in protected:
                self.assertEqual(path.read_text(), 'preserve')
            self.assertTrue(gate.exists())

    def test_healthy_disk_and_unknown_reader_do_not_clear_warm_cache(self):
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp)
            _, _, artifact, _, _ = self.fixture(root)
            def forbidden(*_):
                self.fail('cleaner called without idle pressure evidence')
            result = module.prune(root, apply=True, free=lambda _: 3 * module.GIB, cleaner=forbidden)
            self.assertFalse(result['pressure'])
            result = module.prune(root, apply=True, free=lambda _: 1, in_use=lambda _: True, cleaner=forbidden)
            self.assertEqual(result['busy'], 1)
            self.assertTrue(artifact.exists())

    def test_missing_and_linked_legacy_gate_and_linked_cache_preserved(self):
        for kind in ['missing gate', 'linked gate', 'linked cache']:
            with self.subTest(kind=kind), tempfile.TemporaryDirectory() as temp:
                root = Path(temp)
                owner, cache, artifact, gate, _ = self.fixture(root)
                if kind == 'missing gate':
                    gate.unlink()
                elif kind == 'linked gate':
                    gate.unlink()
                    gate.symlink_to(owner / 'credentials')
                else:
                    cache.rename(owner / 'saved-cache')
                    cache.symlink_to(owner / 'saved-cache')
                result = module.prune(root, apply=True, free=lambda _: 1, in_use=lambda _: False)
                self.assertEqual(result['removed'], 0)
                self.assertEqual((owner / 'credentials').read_text(), 'preserve')
                self.assertTrue(artifact.exists())

    def test_legacy_process_environment_is_protected(self):
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp)
            process = root / '123456789'
            process.mkdir()
            (process / 'environ').write_bytes(b'GOCACHE=/runtime/owner/go-build\0')
            (process / 'cmdline').write_bytes(b'go\0test\0')
            self.assertTrue(module.referenced(Path('/runtime/owner'), root))

    def test_native_go_command_is_isolated_and_drops_privileges(self):
        identity = type('Identity', (), dict(st_uid=1234, st_gid=1235))()
        with patch.object(module.os, 'geteuid', return_value=0), patch.object(module.subprocess, 'run') as run:
            module.clean(Path('/runtime/owner/go-build'), identity)
        args, kwargs = run.call_args
        self.assertEqual(args[0], ['/usr/bin/go', 'clean', '-cache'])
        self.assertEqual(kwargs['user'], 1234)
        self.assertEqual(kwargs['extra_groups'], [])
        self.assertEqual(kwargs['cwd'], '/')
        self.assertEqual(kwargs['env']['GOTOOLCHAIN'], 'local')
        self.assertNotIn('HOME', kwargs['env'])



class NPMCacheTest(unittest.TestCase):
    def fixture(self, root):
        cache = root / 'npm-cache'
        for name in ('_cacache/content-v2', '_cacache/index-v5'):
            (cache / name).mkdir(parents=True)
        (cache / '_cacache/content-v2/artifact').write_text('download')
        return cache

    def test_managed_npm_pressure_and_shared_lease(self):
        with tempfile.TemporaryDirectory() as temporary:
            base = Path(temporary)
            owner = base / ('a' * 64)
            cache = self.fixture(owner)
            gate = owner / '.build-cache.lock'
            gate.touch(mode=0o600)
            cleaned = []
            args = dict(kind='npm', apply=True, free=lambda _: 2*module.GIB,
                        in_use=lambda _: False, cleaner=lambda path, identity, kind: cleaned.append((path,kind)))
            with gate.open('r+b') as lease:
                fcntl.flock(lease,fcntl.LOCK_SH)
                self.assertEqual(module.prune(base,**args)['busy'],1)
            self.assertEqual(module.prune(base,**args)['removed'],1)
            self.assertEqual(cleaned,[(cache,'npm')])

    def test_legacy_npm_uses_same_provider_lease_and_preserves_auth(self):
        with tempfile.TemporaryDirectory() as temporary:
            accounts=Path(temporary);account=accounts/('acct_'+'b'*32)
            home=account/'home';home.mkdir(parents=True)
            cache=self.fixture(home)
            cache.rename(home/'.npm');cache=home/'.npm'
            gate=account/'.execution.lock';gate.touch(mode=0o600)
            secret=home/'.npmrc';secret.write_text('preserved configuration')
            history=home/'.gemini/history';history.parent.mkdir();history.write_text('saved history')
            old=time.time()-7200;os.utime(cache/'_cacache',(old,old))
            cleaned=[]
            def cleaner(path,identity,kind):
                self.assertEqual((path,kind),(cache,'npm'))
                cleaned.append(path)
                shutil.rmtree(path/'_cacache')
            args=dict(apply=True,free=lambda _:2*module.GIB,in_use=lambda _:False,cleaner=cleaner)
            with gate.open('r+b') as lease:
                fcntl.flock(lease,fcntl.LOCK_SH)
                self.assertEqual(module.prune_legacy_npm(accounts,**args)['busy'],1)
            self.assertEqual(module.prune_legacy_npm(accounts,**args)['removed'],1)
            self.assertEqual(cleaned,[cache])
            self.assertEqual(secret.read_text(),'preserved configuration')
            self.assertEqual(history.read_text(),'saved history')
            self.assertTrue(gate.exists())

    def test_legacy_npm_skips_healthy_recent_linked_and_unknown_readers(self):
        for scenario in ('healthy','recent','linked','unknown reader','missing gate'):
            with self.subTest(scenario=scenario), tempfile.TemporaryDirectory() as temporary:
                accounts=Path(temporary);account=accounts/('acct_'+'c'*32)
                home=account/'home';home.mkdir(parents=True)
                cache=self.fixture(home);cache.rename(home/'.npm');cache=home/'.npm'
                gate=account/'.execution.lock';gate.touch(mode=0o600)
                old=time.time()-7200;os.utime(cache/'_cacache',(old,old))
                if scenario=='recent':os.utime(cache/'_cacache',None)
                if scenario=='linked':
                    cache.rename(home/'retained-cache');cache.symlink_to(home/'retained-cache')
                if scenario=='missing gate':gate.unlink()
                def forbidden(*_):self.fail('unsafe legacy cleanup')
                with patch.object(module.os,'geteuid',return_value=0):
                    result=module.prune_legacy_npm(accounts,apply=True,
                        free=lambda _:3*module.GIB if scenario=='healthy' else 2*module.GIB,
                        in_use=lambda _:scenario=='unknown reader',cleaner=forbidden)
                self.assertEqual(result['removed'],0)
                self.assertTrue((cache/'_cacache/content-v2/artifact').exists())
                if scenario=='missing gate':self.assertFalse(gate.exists())

    @unittest.skipUnless(Path('/usr/bin/npm').is_file(),'native npm required')
    def test_official_npm_clears_only_download_cache_without_network(self):
        with tempfile.TemporaryDirectory() as temporary:
            root=Path(temporary);home=root/'home';home.mkdir();cache=root/'npm-cache'
            package=root/'fixture.tgz'
            data=b'{"name":"cuan-cache-fixture","version":"1.0.0"}'
            with tarfile.open(package,'w:gz') as archive:
                entry=tarfile.TarInfo('package/package.json');entry.size=len(data)
                archive.addfile(entry,io.BytesIO(data))
            environment={'PATH':os.defpath,'HOME':str(home),
                         'NPM_CONFIG_USERCONFIG':str(home/'user.npmrc'),
                         'NPM_CONFIG_GLOBALCONFIG':str(home/'global.npmrc')}
            subprocess.run(['/usr/bin/npm','cache','add',str(package),'--cache',str(cache),'--ignore-scripts'],
                           cwd=root,env=environment,capture_output=True,timeout=20,check=True)
            self.assertTrue(module.npm_cache(cache))
            protected=cache/'operator-note';protected.write_text('preserve')
            module.clean(cache,cache.stat(),'npm')
            self.assertFalse((cache/'_cacache').exists())
            self.assertEqual(protected.read_text(),'preserve')
            self.assertTrue(package.exists())


class ComposerCacheTest(unittest.TestCase):
    def test_pressure_cleaner_retains_live_downloads_and_uses_same_lease(self):
        with tempfile.TemporaryDirectory() as temporary:
            base=Path(temporary);owner=base/('a'*64);cache=owner/'composer-cache'
            (cache/'files').mkdir(parents=True)
            archive=cache/'files/package.zip';archive.write_text('partial download')
            gate=owner/'.build-cache.lock';gate.touch(mode=0o600)
            source=owner/'workspace.php';source.write_text('preserve')
            def cleaner(path,identity,kind):
                self.assertEqual((path,kind),(cache,'composer'))
                archive.unlink()
            args=dict(kind='composer',apply=True,free=lambda _:2*module.GIB,
                      in_use=lambda _:False,cleaner=cleaner)
            with gate.open('r+b') as lease:
                fcntl.flock(lease,fcntl.LOCK_SH)
                self.assertEqual(module.prune(base,**args)['busy'],1)
            self.assertTrue(archive.exists())
            self.assertEqual(module.prune(base,**args)['removed'],1)
            self.assertEqual(source.read_text(),'preserve')
            self.assertTrue(gate.exists())

    def test_native_composer_cleaner_has_disposable_auth_free_home(self):
        with tempfile.TemporaryDirectory() as temporary:
            cache=Path(temporary)/'composer-cache';cache.mkdir()
            with patch.object(module.shutil,'which',return_value='/usr/local/bin/composer'), patch.object(module.subprocess,'run') as run:
                module.clean(cache,cache.stat(),'composer')
                args,kwargs=run.call_args
                self.assertEqual(args[0],['/usr/local/bin/composer','--no-plugins','--no-scripts','clear-cache'])
                self.assertEqual(kwargs['env']['COMPOSER_CACHE_DIR'],str(cache))
                self.assertNotIn('COMPOSER_AUTH',kwargs['env'])
                home=Path(kwargs['env']['HOME'])
                self.assertFalse(home.exists())

    @unittest.skipUnless(shutil.which('composer'), 'native Composer required')
    def test_real_composer_clears_only_archive_cache(self):
        with tempfile.TemporaryDirectory() as temporary:
            owner=Path(temporary);cache=owner/'composer-cache';(cache/'files').mkdir(parents=True)
            (cache/'files/archive.zip').write_text('download')
            protected=owner/'vendor/source.php';protected.parent.mkdir();protected.write_text('preserve')
            with patch.object(module.os,'defpath',str(Path(shutil.which('php')).parent)+':'+os.defpath), patch.object(module.shutil,'which',return_value=shutil.which('composer')):
                module.clean(cache,cache.stat(),'composer')
            self.assertFalse((cache/'files/archive.zip').exists())
            self.assertEqual(protected.read_text(),'preserve')

if __name__ == '__main__':
    unittest.main()
