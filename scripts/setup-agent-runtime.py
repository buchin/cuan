#!/usr/bin/env python3
"""Set up and verify Cuan's native Linux AI runtime without provider inference."""
import argparse
import json
import os
from pathlib import Path
import pwd
import subprocess
import sys


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--user', required=True, help='Linux account running the Cuan service')
    parser.add_argument('--check', action='store_true', help='Verify only; do not install or change host policy')
    parser.add_argument('--browser', action='store_true', help='Include browser setup/readiness for screenshot tasks')
    args = parser.parse_args()
    if sys.platform != 'linux':
        parser.error('Native Linux is required')
    try:
        account = pwd.getpwnam(args.user)
    except KeyError:
        parser.error('Service user does not exist')
    if account.pw_uid == 0:
        parser.error('Choose a non-root Linux service user')
    home = Path(account.pw_dir)
    scripts = Path(__file__).resolve().parent

    def as_user(command):
        if os.geteuid() == account.pw_uid:
            return command
        if os.geteuid() != 0:
            raise RuntimeError('Run as the service user for --check, or as root for installation')
        return ['sudo', '-H', '-u', args.user, '--', *command]

    try:
        if not args.check:
            if os.geteuid() != 0:
                raise RuntimeError('Installation requires root; use --check for verification only')
            if Path('/.dockerenv').exists():
                raise RuntimeError('This installer supports native Linux. The stock Docker image does not yet provide a supported AI sandbox')
            # The sandbox gate runs first: do not download a CLI into an unusable host.
            subprocess.run([sys.executable, str(scripts/'setup-inbox-sandbox.py'), '--user', args.user], check=True, timeout=180)
            subprocess.run(as_user([sys.executable, str(scripts/'install-antigravity.py'), '--prefix', str(home/'.local')]), check=True, timeout=240)

        subprocess.run([sys.executable, str(scripts/'setup-inbox-sandbox.py'), '--user', args.user, '--check'], check=True, timeout=25)
        # Resolve under the service account's PATH, never root's unrelated CLI.
        env = {'HOME': str(home), 'PATH': str(home/'.local/bin')+':/usr/local/bin:/usr/bin:/bin'}
        locate = subprocess.run(as_user(['/usr/bin/env', '-i', *[k+'='+v for k,v in env.items()], '/bin/sh', '-c', 'command -v agy']), capture_output=True, text=True, timeout=5)
        if locate.returncode or not locate.stdout.strip():
            raise RuntimeError('AI CLI is missing for the service user; rerun without --check')
        cli = Path(locate.stdout.strip()).resolve(strict=True)
        bwrap = Path('/opt/cuan/runtime/bwrap')
        if not bwrap.exists():
            lookup = subprocess.run(as_user(['/usr/bin/env', '-i', *[k+'='+v for k,v in env.items()], '/bin/sh', '-c', 'command -v bwrap']), capture_output=True, text=True, timeout=5)
            if lookup.returncode or not lookup.stdout.strip():
                raise RuntimeError('Chat sandbox is missing for the service user')
            bwrap = Path(lookup.stdout.strip()).resolve(strict=True)
        mounts = []
        for name in ['/usr', '/bin', '/lib', '/lib64', '/etc/ssl']:
            if Path(name).exists():
                mounts += ['--ro-bind', name, name]
        # Disposable HOME guarantees this check never reads/refreshes an AI account.
        command = [str(bwrap), '--die-with-parent', '--new-session', '--unshare-pid', '--unshare-ipc', '--unshare-uts', '--unshare-net', '--cap-drop', 'ALL', '--clearenv', *mounts,
                   '--proc', '/proc', '--remount-ro', '/proc', '--dev', '/dev', '--tmpfs', '/tmp', '--dir', '/tmp/runtime-home',
                   '--ro-bind', str(cli), str(cli), '--setenv', 'HOME', '/tmp/runtime-home', '--setenv', 'PATH', env['PATH'],
                   '--setenv', 'TMPDIR', '/tmp', '--chdir', '/tmp/runtime-home', '--', str(cli), '--version']
        result = subprocess.run(as_user(command), capture_output=True, text=True, timeout=20)
        if result.returncode or not result.stdout.strip():
            raise RuntimeError('AI CLI could not start inside the sandbox. Check CLI compatibility and host namespace policy')
        if args.browser:
            browser_args = ['--check'] if args.check else ['--install-deps']
            subprocess.run([sys.executable, str(scripts/'setup-browser-runtime.py'), '--user', args.user, *browser_args], check=True, timeout=600)
        print(json.dumps({'runtimeReady': True, 'sandboxReady': True, 'cliStartsInSandbox': True, 'providerInvocations': 0,
                          'nextStep': 'Connect an AI account in Settings > Agents, then submit a goal in Studio'}))
    except (OSError, RuntimeError, subprocess.SubprocessError) as error:
        parser.exit(1, f'AI runtime setup failed: {error}\n')


if __name__ == '__main__':
    main()
