#!/usr/bin/env python3
"""Isolated installer contract checks; never starts Docker or modifies the host."""
import os
from pathlib import Path
import re
import shutil
import subprocess
import tempfile
import unittest

ROOT = Path(__file__).resolve().parents[1]

class InstallerTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name)
        (self.root / 'scripts').mkdir()
        (self.root / 'fake').mkdir()
        shutil.copy(ROOT / 'scripts/install.sh', self.root / 'scripts/install.sh')
        shutil.copy(ROOT / '.env.example', self.root / '.env.example')
        self.docker = self.root / 'fake/docker'
        self.docker.write_text('''#!/bin/bash
case "$*" in
  info) exit "${DAEMON_EXIT:-0}";;
  'compose version') exit 0;;
  *'up -d --build') exit "${UP_EXIT:-0}";;
  *'exec -T app'*) exit "${READY_EXIT:-0}";;
esac
exit 0
''')
        self.docker.chmod(0o755)
        sleep = self.root / 'fake/sleep'
        sleep.write_text('#!/bin/sh\nexit 0\n')
        sleep.chmod(0o755)

        self.open_log = self.root / 'open.log'
        open_cmd = self.root / 'fake/open'
        open_cmd.write_text(f'#!/bin/sh\necho "$@" >> "{self.open_log}"\nexit 0\n')
        open_cmd.chmod(0o755)

    def run_install(self, *args, **env):
        values = {**os.environ, 'PATH': str(self.root / 'fake') + ':' + os.environ['PATH'], **env}
        return subprocess.run(['bash', str(self.root / 'scripts/install.sh'), '--skip-preflight', *args], env=values, capture_output=True, text=True)

    def test_dry_run_does_not_write_config(self):
        result = self.run_install('--dry-run')
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertFalse((self.root / '.env').exists())

    def test_fresh_private_config_and_existing_secrets_preserved(self):
        result = self.run_install()
        self.assertEqual(result.returncode, 0, result.stderr)
        config = self.root / '.env'
        original = config.read_bytes()
        self.assertEqual(config.stat().st_mode & 0o777, 0o600)
        self.assertNotIn(b'=replace_with_', original)
        self.assertEqual(self.run_install().returncode, 0)
        self.assertEqual(config.read_bytes(), original)

    def test_unreachable_daemon_does_not_create_config(self):
        self.assertNotEqual(self.run_install(DAEMON_EXIT='1').returncode, 0)
        self.assertFalse((self.root / '.env').exists())

    def test_launch_and_readiness_failures_never_claim_success(self):
        for env in ({'UP_EXIT': '1'}, {'READY_EXIT': '1'}):
            result = self.run_install(**env)
            self.assertNotEqual(result.returncode, 0)
            self.assertNotIn('installation completed successfully', result.stdout)
            self.assertTrue((self.root / '.env').exists())

    def test_existing_placeholders_rejected(self):
        shutil.copy(self.root / '.env.example', self.root / '.env')
        result = self.run_install()
        self.assertNotEqual(result.returncode, 0)
        self.assertIn('placeholder', result.stderr)

    def test_staging_domain_and_port(self):
        result = self.run_install('--staging', '--domain', 'staging.example.com', '--port', '9081')
        self.assertEqual(result.returncode, 0, result.stderr)
        config = (self.root / '.env.staging').read_text()
        self.assertIn('CUAN_STAGING_DOMAIN=staging.example.com', config)
        self.assertIn('PORT=9081', config)

    def test_invalid_arguments(self):
        for args in [('--domain',), ('--email',), ('--port',), ('--port', 'abc'), ('--port', '65536')]:
            self.assertNotEqual(self.run_install(*args).returncode, 0)

    def test_darwin_remote_uses_home_cuan(self):
        user_home = self.root / 'userhome'
        user_home.mkdir()
        env = {
            'HOME': str(user_home),
            'CUAN_OS_TYPE': 'Darwin',
            'CUAN_FORCE_REMOTE': 'true',
            'PATH': str(self.root / 'fake') + ':' + os.environ['PATH']
        }
        # Run with --dry-run so it doesn't need to clone or run containers
        result = subprocess.run(
            ['bash', str(self.root / 'scripts/install.sh'), '--dry-run', '--skip-preflight'],
            env=env,
            capture_output=True,
            text=True
        )
        self.assertEqual(result.returncode, 0, result.stderr)
        expected_dir = str(user_home / '.cuan')
        self.assertIn(expected_dir, result.stdout)
        self.assertNotIn('/opt/cuan', result.stdout)

    def test_darwin_unreachable_daemon_friendly_error(self):
        result = self.run_install(CUAN_OS_TYPE='Darwin', DAEMON_EXIT='1')
        self.assertNotEqual(result.returncode, 0)
        self.assertIn('Docker Desktop belum aktif / berjalan', result.stderr)
        self.assertIn('Engine running', result.stderr)

    def test_darwin_missing_docker_friendly_error(self):
        no_docker_dir = self.root / 'no_docker_bin'
        no_docker_dir.mkdir()
        for tool in ('bash', 'sed', 'awk', 'uname', 'mkdir', 'od', 'chmod', 'tr', 'cp', 'cat'):
            tool_path = shutil.which(tool)
            if tool_path:
                (no_docker_dir / tool).symlink_to(tool_path)
        env = {
            **os.environ,
            'PATH': str(no_docker_dir),
            'CUAN_OS_TYPE': 'Darwin'
        }
        result = subprocess.run(
            ['bash', str(self.root / 'scripts/install.sh'), '--skip-preflight'],
            env=env,
            capture_output=True,
            text=True
        )
        self.assertNotEqual(result.returncode, 0)
        self.assertIn('Docker Desktop belum terinstal di Mac Anda', result.stderr)
        self.assertIn('https://www.docker.com/products/docker-desktop/', result.stderr)

    def test_linux_vps_missing_docker_error(self):
        no_docker_dir = self.root / 'no_docker_bin_linux'
        no_docker_dir.mkdir()
        for tool in ('bash', 'sed', 'awk', 'uname', 'mkdir', 'od', 'chmod', 'tr', 'cp', 'cat'):
            tool_path = shutil.which(tool)
            if tool_path:
                (no_docker_dir / tool).symlink_to(tool_path)
        env = {
            **os.environ,
            'PATH': str(no_docker_dir),
            'CUAN_OS_TYPE': 'Linux'
        }
        result = subprocess.run(
            ['bash', str(self.root / 'scripts/install.sh'), '--skip-preflight'],
            env=env,
            capture_output=True,
            text=True
        )
        self.assertNotEqual(result.returncode, 0)
        self.assertIn('Docker Engine is required for this installer', result.stderr)

    def test_darwin_success_opens_browser_and_shows_banner(self):
        result = self.run_install(CUAN_OS_TYPE='Darwin')
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertIn('Cuan berhasil dipasang dan berjalan di Mac Anda!', result.stdout)
        self.assertIn('http://localhost:8080', result.stdout)
        self.assertTrue(self.open_log.exists())
        self.assertIn('http://localhost:8080', self.open_log.read_text())

    def test_linux_vps_unreachable_daemon_error(self):
        result = self.run_install(CUAN_OS_TYPE='Linux', DAEMON_EXIT='1')
        self.assertNotEqual(result.returncode, 0)
        self.assertIn('Docker Engine is not reachable', result.stderr)

    def test_install_sh_never_invokes_sudo(self):
        script_text = (ROOT / 'scripts/install.sh').read_text()
        # Ensure sudo command is never invoked
        self.assertNotIn('sudo ', script_text)

    def test_cuan_install_dir_override(self):
        for os_type in ('Darwin', 'Linux'):
            custom_dir = self.root / f'custom_cuan_{os_type}'
            custom_dir.mkdir()
            (custom_dir / 'docker-compose.yml').write_text('services: {}')
            env = {
                'CUAN_INSTALL_DIR': str(custom_dir),
                'CUAN_OS_TYPE': os_type,
                'CUAN_FORCE_REMOTE': 'true',
                'PATH': str(self.root / 'fake') + ':' + os.environ['PATH']
            }
            result = subprocess.run(
                ['bash', str(self.root / 'scripts/install.sh'), '--dry-run', '--skip-preflight'],
                env=env,
                capture_output=True,
                text=True
            )
            self.assertEqual(result.returncode, 0, result.stderr)
            self.assertIn(str(custom_dir), result.stdout)


class PowerShellInstallerTests(unittest.TestCase):
    def setUp(self):
        self.ps1_path = ROOT / 'scripts/install.ps1'
        self.assertTrue(self.ps1_path.exists(), "scripts/install.ps1 must exist")
        self.content = self.ps1_path.read_text(encoding='utf-8')

    def test_ps1_contains_default_install_dir(self):
        self.assertIn('$env:USERPROFILE', self.content)
        self.assertIn('.cuan', self.content)

    def test_ps1_contains_docker_desktop_friendly_messages(self):
        self.assertIn('Docker Desktop belum terinstal di komputer Windows Anda', self.content)
        self.assertIn('https://www.docker.com/products/docker-desktop/', self.content)
        self.assertIn('Docker Desktop terdeteksi tetapi belum aktif / berjalan', self.content)
        self.assertIn('Engine running', self.content)

    def test_ps1_security_and_primitives(self):
        # Uses Cryptographic RNG for secrets
        self.assertIn('System.Security.Cryptography.RandomNumberGenerator', self.content)
        # Opens browser
        self.assertIn('Start-Process', self.content)
        self.assertIn('http://localhost:', self.content)
        # CmdletBinding and parameters
        self.assertIn('[CmdletBinding()]', self.content)
        self.assertIn('param(', self.content)
        self.assertIn('$DryRun', self.content)

    def test_ps1_syntax_safety_no_linux_isms(self):
        # Ensure no accidental bash commands like sudo, grep, sed
        self.assertNotIn('sudo ', self.content)
        self.assertNotIn('grep ', self.content)
        self.assertNotIn('sed ', self.content)

    def test_ps1_balanced_delimiters(self):
        # Strip comments and string literals to test balanced structural brackets
        text = self.content
        # Remove block comments <# ... #>
        text = re.sub(r'<#.*?#>', '', text, flags=re.DOTALL)
        # Remove line comments
        text = re.sub(r'#.*', '', text)
        # Count curly braces
        open_curly = text.count('{')
        close_curly = text.count('}')
        self.assertEqual(open_curly, close_curly, f"Unbalanced curly braces: {open_curly} open vs {close_curly} close")
        # Count parentheses
        open_paren = text.count('(')
        close_paren = text.count(')')
        self.assertEqual(open_paren, close_paren, f"Unbalanced parentheses: {open_paren} open vs {close_paren} close")
        # Count square brackets
        open_bracket = text.count('[')
        close_bracket = text.count(']')
        self.assertEqual(open_bracket, close_bracket, f"Unbalanced square brackets: {open_bracket} open vs {close_bracket} close")


class ReadmeDocumentationTests(unittest.TestCase):
    def setUp(self):
        self.readme_path = ROOT / 'README.md'
        self.assertTrue(self.readme_path.exists())
        self.content = self.readme_path.read_text(encoding='utf-8')

    def test_readme_contains_universal_1line_macos_linux(self):
        self.assertIn('curl -fsSL https://cuan.work/install.sh | bash', self.content)
        self.assertIn('macOS & Linux:', self.content)

    def test_readme_contains_windows_powershell_1line(self):
        self.assertIn('irm https://cuan.work/install.ps1 | iex', self.content)
        self.assertIn('Windows (PowerShell):', self.content)

    def test_readme_contains_linux_vps_instructions(self):
        self.assertIn('curl -fsSL https://cuan.work/install.sh | bash -s -- --domain', self.content)


if __name__ == '__main__':
    unittest.main()
