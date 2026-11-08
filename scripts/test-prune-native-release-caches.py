import fcntl
import importlib.util
import json
from pathlib import Path
import tempfile
from types import SimpleNamespace
import unittest
from unittest.mock import patch

spec = importlib.util.spec_from_file_location('native_pruner', Path(__file__).with_name('prune-native-release-caches.py'))
module = importlib.util.module_from_spec(spec)
spec.loader.exec_module(module)

class NativeCachePruning(unittest.TestCase):
    def setUp(self):
        self.temporary = tempfile.TemporaryDirectory()
        self.addCleanup(self.temporary.cleanup)
        self.base = Path(self.temporary.name)
        self.root = self.base / ('a' * 64)
        self.meta = self.root / '.git'
        self.runtime = self.meta / 'cuan-native-runtime'
        (self.runtime / 'cache/go').mkdir(parents=True)
        (self.meta / 'cuan-verification.lock').touch()
        (self.meta / 'cuan-verification.json').write_text(json.dumps({'target_commit':'b'*40}))
        self.helpers = SimpleNamespace(git=lambda *args: ('b'*40).encode(), referenced=lambda root:False, owner_credentials=lambda root:{})
        self.loader = patch.object(module, 'load_helpers', return_value=self.helpers)
        self.loader.start()
        self.addCleanup(self.loader.stop)

    def test_dry_run_never_cleans(self):
        with patch.object(module.subprocess, 'run') as run:
            result = module.prune(self.base, force=True)
        self.assertEqual(result['eligible'], [self.root.name])
        run.assert_not_called()

    def test_only_official_go_clean_with_exact_cache_paths(self):
        with patch.object(module.subprocess, 'run') as run:
            result = module.prune(self.base, apply=True, force=True)
        self.assertEqual(result['cleaned'], [self.root.name])
        args, kwargs = run.call_args
        self.assertEqual(args[0], ['/usr/bin/go','clean','-cache','-modcache'])
        self.assertEqual(kwargs['env']['GOMODCACHE'], str(self.runtime / 'data/go/pkg/mod'))
        self.assertTrue(self.meta.is_dir())

    def test_active_reader_or_lease_protects_workspace(self):
        self.helpers.referenced = lambda root:True
        with patch.object(module.subprocess, 'run') as run:
            self.assertEqual(module.prune(self.base, True, True)['busy'], 1)
            self.helpers.referenced = lambda root:False
            with (self.meta / 'cuan-verification.lock').open('r+b') as gate:
                fcntl.flock(gate, fcntl.LOCK_SH)
                self.assertEqual(module.prune(self.base, True, True)['busy'], 1)
        run.assert_not_called()

    def test_linked_ancestor_and_wrong_source_never_clean(self):
        outside = self.base / 'outside'
        outside.mkdir()
        (self.runtime / 'data').symlink_to(outside, target_is_directory=True)
        (outside / 'go/pkg/mod').mkdir(parents=True)
        with patch.object(module.subprocess, 'run') as run:
            self.assertEqual(module.prune(self.base, True, True)['eligible'], [])
            self.helpers.git = lambda *args:b'c'*40
            self.assertEqual(module.prune(self.base, True, True)['eligible'], [])
        run.assert_not_called()

if __name__ == '__main__':
    unittest.main()
