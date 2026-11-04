import importlib.util
from pathlib import Path
import unittest

spec = importlib.util.spec_from_file_location("release_repository", Path(__file__).with_name("release-repository.py"))
source = importlib.util.module_from_spec(spec)
spec.loader.exec_module(source)


class SourceIdentity(unittest.TestCase):
    def test_credentials_do_not_enter_build_identity(self):
        for remote in ("https://github.com/Owner/Fork.git", "git@github.com:Owner/Fork.git", "ssh://git@github.com/Owner/Fork.git", "https://test-user:synthetic-secret@github.com/Owner/Fork.git/"):
            self.assertEqual(source.source_identity(remote), "owner/fork")

    def test_wrong_hosts_extra_paths_and_linker_injection_are_rejected(self):
        for remote in ("https://github.com.evil.test/owner/repo", "https://other.test/owner/repo", "file:///owner/repo", "https://github.com/owner/repo/extra", "https://github.com/owner/repo?token=synthetic", "https://github.com/owner/repo -X bad=value", "git@github.com:../repo.git"):
            with self.subTest(remote=remote), self.assertRaises(ValueError):
                source.source_identity(remote)


if __name__ == "__main__":
    unittest.main()
