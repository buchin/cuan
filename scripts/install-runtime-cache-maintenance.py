#!/usr/bin/env python3
"""Install pinned storage maintenance, preserving active work and retained source."""
import argparse
import hashlib
import os
from pathlib import Path
import shutil
import subprocess
import tempfile
import pwd
import re

TOOLS = ('prune-native-release-caches.py', 'prune-runtime-build-cache.py', 'prune-provider-go-cache.py', 'prune-verification-dependencies.py', 'compact-verification-source.py')


def install(base, service_home, unit_dir, source):
    for path in (base, service_home, unit_dir):
        if not path.is_absolute() or any(c not in '/abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789_-. ' for c in str(path)) or ' ' in str(path):
            raise ValueError('Maintenance paths must be absolute simple paths')
    payload = {}
    for name in TOOLS:
        path = source / name
        if path.is_symlink() or not path.is_file():
            raise ValueError('Maintenance source must be a regular file')
        payload[name] = path.read_bytes()
    digest = hashlib.sha256(b''.join(name.encode() + b'\0' + payload[name] for name in TOOLS)).hexdigest()
    parent = base / 'maintenance/runtime-cache-tools'
    parent.mkdir(parents=True, exist_ok=True, mode=0o700)
    target = parent / digest
    if target.exists():
        if target.is_symlink() or any((target / name).is_symlink() or (target / name).read_bytes() != data for name, data in payload.items()):
            raise ValueError('Pinned maintenance tools do not match')
    else:
        temporary = Path(tempfile.mkdtemp(prefix='.install-', dir=parent))
        try:
            for name, data in payload.items():
                path = temporary / name
                path.write_bytes(data)
                path.chmod(0o500)
            os.rename(temporary, target)
        finally:
            if temporary.exists():
                shutil.rmtree(temporary)
    unit_dir.mkdir(parents=True, exist_ok=True)
    command = f'/usr/bin/python3 {target}/prune-runtime-build-cache.py'
    service = f'''[Unit]
Description=Cuan idle caches and accepted verifier source maintenance

[Service]
Type=oneshot
UMask=0077
Nice=10
IOSchedulingClass=idle
TimeoutStartSec=180
ExecStart={command} --runtime {base}/data/go-build-runtime --apply
ExecStart={command} --runtime {base}/data/php-build-runtime --kind composer --apply
ExecStart={command} --runtime {base}/data/npm-build-runtime --kind npm --legacy-accounts {service_home}/.local/share/cuan/accounts --apply
ExecStart=/usr/bin/python3 {target}/prune-verification-dependencies.py --snapshots {base}/data/projects/release-workspaces --under-storage-pressure --apply
ExecStart=/usr/bin/python3 {target}/prune-native-release-caches.py --workspaces {base}/data/projects/release-workspaces --apply
ExecStart=/usr/bin/python3 {target}/prune-verification-dependencies.py --snapshots {base}/data/projects/verification-snapshots --under-storage-pressure --apply
ExecStart=/usr/bin/python3 {target}/prune-verification-dependencies.py --snapshots {base}/data/workspaces/verification --under-storage-pressure --apply
ExecStart=/usr/bin/python3 {target}/compact-verification-source.py --snapshots {base}/data/projects/verification-snapshots --apply
ExecStart=/usr/bin/python3 {target}/compact-verification-source.py --snapshots {base}/data/workspaces/verification --apply
'''
    timer = '''[Unit]
Description=Recheck idle Cuan caches after task execution

[Timer]
OnBootSec=2min
OnUnitInactiveSec=2min
AccuracySec=15s
Unit=cuan-runtime-cache-maintenance.service

[Install]
WantedBy=timers.target
'''
    for name, content in [('cuan-runtime-cache-maintenance.service', service), ('cuan-runtime-cache-maintenance.timer', timer)]:
        descriptor, temporary = tempfile.mkstemp(prefix='.' + name, dir=unit_dir)
        try:
            with os.fdopen(descriptor, 'w') as handle:
                handle.write(content)
            os.chmod(temporary, 0o644)
            os.replace(temporary, unit_dir / name)
        finally:
            if os.path.exists(temporary):
                os.unlink(temporary)
    return target


def install_trigger_access(service_home, sudoers_dir=Path('/etc/sudoers.d')):
    # The app can request one fixed worker, never a command supplied by an agent.
    username = pwd.getpwuid(service_home.stat().st_uid).pw_name
    if not re.fullmatch(r'[a-z_][a-z0-9_-]*[$]?', username):
        raise ValueError('Unsupported native service user')
    if service_home.stat().st_uid == 0:
        return
    sudoers_dir.mkdir(parents=True, exist_ok=True)
    descriptor, temporary = tempfile.mkstemp(prefix='.cuan-storage-', dir=sudoers_dir)
    try:
        with os.fdopen(descriptor, 'w') as handle:
            handle.write(f'{username} ALL=(root) NOPASSWD: /usr/bin/systemctl start cuan-runtime-cache-maintenance.service\n')
        os.chmod(temporary, 0o440)
        subprocess.run(['/usr/sbin/visudo', '-cf', temporary], check=True, capture_output=True)
        os.replace(temporary, sudoers_dir / 'cuan-storage-maintenance')
    finally:
        if os.path.exists(temporary):
            os.unlink(temporary)


if __name__ == '__main__':
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--base', type=Path, default=Path('/opt/cuan'))
    parser.add_argument('--service-home', type=Path)
    parser.add_argument('--print-tools', action='store_true', help='List required payload filenames without installing anything')
    args = parser.parse_args()
    if args.print_tools:
        print('\n'.join(TOOLS))
        parser.exit()
    if args.service_home is None:
        parser.error('--service-home is required for installation')
    if os.geteuid() != 0:
        parser.error('Run through native deployment as root')
    install(args.base, args.service_home, Path('/etc/systemd/system'), Path(__file__).resolve().parent)
    install_trigger_access(args.service_home)
    subprocess.run(['systemctl', 'daemon-reload'], check=True)
    subprocess.run(['systemctl', 'enable', '--now', 'cuan-runtime-cache-maintenance.timer'], check=True)
