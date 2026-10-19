"""Targeted release safety tests; no system service/database mutations."""
import importlib.util
import json
import tempfile
import unittest
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
    def test_schema20_and_exact_html_required_for_old_and_new_code(self):
        with tempfile.TemporaryDirectory() as directory:
            a.BASE = Path(directory)
            html = a.BASE / 'current/web/dist/index.html'
            html.parent.mkdir(parents=True)
            html.write_bytes(b'candidate')
            for schema, status, database, served, expected in [
                (20, 'ready', 'connected', b'candidate', True),
                (19, 'ready', 'connected', b'candidate', False),
                (20, 'not_ready', 'connected', b'candidate', False),
                (20, 'ready', 'disconnected', b'candidate', False),
                (20, 'ready', 'connected', b'stale', False),
            ]:
                with self.subTest(schema=schema, status=status, served=served):
                    responses = [Response(b'{}'), Response(json.dumps(dict(migrations_count=schema, status=status, database=database)).encode()), Response(served)]
                    with patch.object(a.urllib.request, 'urlopen', side_effect=responses):
                        self.assertEqual(a.healthy(), expected)
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
    def test_empty_or_public_existing_backup_is_rejected(self):
        with tempfile.TemporaryDirectory() as directory:
            a.BASE = Path(directory)
            backup = a.BASE/'backups/x.dump'
            backup.parent.mkdir(); backup.write_bytes(b'')
            backup.chmod(0o600)
            with self.assertRaises(RuntimeError): a.backup_database('x')
            backup.write_bytes(b'not empty'); backup.chmod(0o644)
            with self.assertRaises(RuntimeError): a.backup_database('x')

if __name__ == '__main__': unittest.main()
