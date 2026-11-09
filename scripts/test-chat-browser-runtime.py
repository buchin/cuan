#!/usr/bin/env python3
"""Pinned chat-browser installer contracts without downloads or host changes."""
import hashlib
import importlib.util
import io
import json
from pathlib import Path
import tarfile
import tempfile
import unittest
from unittest.mock import patch

spec = importlib.util.spec_from_file_location('chat_browser_runtime', Path(__file__).with_name('setup-chat-browser-runtime.py'))
runtime = importlib.util.module_from_spec(spec)
spec.loader.exec_module(runtime)


def archive(main=None, symlink=False):
    output = io.BytesIO()
    main = main or b'#!/bin/sh\n[ -z "$CUAN_AUTH_TOKEN" ] && [ -z "$HTTP_PROXY" ] || exit 91\nprintf "obscura 0.2.6\\n"\n'
    with tarfile.open(fileobj=output, mode='w:gz') as bundle:
        item = tarfile.TarInfo('obscura')
        if symlink:
            item.type, item.linkname = tarfile.SYMTYPE, '../../outside'
            bundle.addfile(item)
        else:
            item.size = len(main)
            bundle.addfile(item, io.BytesIO(main))
        for name in ['obscura-worker', '../../outside']:
            item = tarfile.TarInfo(name)
            item.size = 6
            bundle.addfile(item, io.BytesIO(b'unused'))
    return output.getvalue()


class RuntimeTests(unittest.TestCase):
    def setUp(self):
        self.directory = tempfile.TemporaryDirectory()
        self.addCleanup(self.directory.cleanup)
        self.home = Path(self.directory.name) / 'service-home'
        self.home.mkdir()
        self.sandbox = Path(self.directory.name) / 'bwrap'
        self.sandbox.write_text('#!/bin/sh\nexit 0\n')
        self.sandbox.chmod(0o700)
        self.payload = archive()
        self.checksum = hashlib.sha256(self.payload).hexdigest()
        self.pins = patch.dict(runtime.ARCHIVES, {'x86_64': self.checksum})
        self.pins.start()
        self.addCleanup(self.pins.stop)

    def install(self, **kwargs):
        with patch.object(runtime, 'download', side_effect=lambda url, target: target.write_bytes(self.payload)) as download:
            result = runtime.setup(self.home, 'x86_64', self.sandbox, **kwargs)
            return result, download

    def test_installs_only_verified_main_binary_and_reuses_cache(self):
        with patch.dict('os.environ', {'CUAN_AUTH_TOKEN': 'must-not-inherit', 'HTTP_PROXY': 'must-not-inherit'}):
            result, download = self.install()
        self.assertTrue(result['version_verified'])
        self.assertEqual(download.call_count, 1)
        base = self.home / '.local/share/cuan-chat-browser'
        self.assertEqual(sorted(path.name for path in (base / runtime.VERSION).iterdir()), ['obscura'])
        self.assertFalse((self.home / 'outside').exists())
        self.assertEqual(list(base.glob('install-*')), [])
        self.assertEqual(list(base.glob('*.tar.gz')), [])
        cached, download = self.install()
        self.assertEqual(cached, result)
        download.assert_not_called()
        checked, download = self.install(check=True)
        self.assertEqual(checked, result)
        download.assert_not_called()

    def test_checksum_failure_cannot_install_executable(self):
        self.payload += b'corrupted'
        with self.assertRaisesRegex(RuntimeError, 'checksum mismatch'):
            self.install()
        self.assertFalse((self.home / '.local/share/cuan-chat-browser' / runtime.VERSION / 'obscura').exists())

    def test_archive_symlink_cannot_be_extracted(self):
        self.payload = archive(symlink=True)
        runtime.ARCHIVES['x86_64'] = hashlib.sha256(self.payload).hexdigest()
        with self.assertRaisesRegex(RuntimeError, 'Invalid browser executable'):
            self.install()

    def test_modified_binary_is_preserved_and_rejected(self):
        self.install()
        binary = self.home / '.local/share/cuan-chat-browser' / runtime.VERSION / 'obscura'
        binary.write_text('unique changed content')
        with self.assertRaisesRegex(RuntimeError, 'integrity check failed'):
            self.install()
        self.assertEqual(binary.read_text(), 'unique changed content')

    def test_directory_symlink_and_incomplete_compatibility_runtime_rejected(self):
        base = self.home / '.local/share/cuan-chat-browser'
        base.parent.mkdir(parents=True)
        base.symlink_to(self.directory.name, target_is_directory=True)
        with self.assertRaisesRegex(RuntimeError, 'must not be symlinks'):
            self.install()
        base.unlink()
        with self.assertRaisesRegex(RuntimeError, 'both a Cuan-managed loader'):
            self.install(loader=self.sandbox)

    def test_nonstarting_binary_does_not_report_success(self):
        self.payload = archive(b'#!/bin/sh\nexit 1\n')
        runtime.ARCHIVES['x86_64'] = hashlib.sha256(self.payload).hexdigest()
        with self.assertRaisesRegex(RuntimeError, 'cannot start'):
            self.install()

    def test_offline_archive_keeps_checksum_gate_and_does_not_copy_worker(self):
        source = Path(self.directory.name) / 'official.tar.gz'
        source.write_bytes(self.payload)
        result, download = self.install(archive=source)
        self.assertTrue(result['installed'])
        download.assert_not_called()
        manifest = json.loads(Path(result['manifest']).read_text())
        self.assertEqual(manifest['archive_sha256'], self.checksum)
        self.assertFalse(Path(manifest['binary']).with_name('obscura-worker').exists())


if __name__ == '__main__':
    unittest.main()
