#!/usr/bin/env python3
"""Rotate oversized web server logs and vacuum systemd journal logs safely."""
import argparse
import json
import os
from pathlib import Path
import shutil
import subprocess

MIB = 1 << 20


def oversized_logs(log_dir, max_bytes):
    if not log_dir.is_dir() or log_dir.is_symlink():
        return []
    found = []
    for entry in sorted(log_dir.iterdir()):
        if entry.is_symlink() or not entry.is_file() or not entry.name.endswith('.log'):
            continue
        try:
            if entry.stat().st_size >= max_bytes:
                found.append(entry)
        except OSError:
            continue
    return found


def maintain(*, apply=False, max_log_mb=50, journal_size='100M',
             log_configs=(('nginx', Path('/var/log/nginx'), Path('/etc/logrotate.d/nginx')),
                          ('caddy', Path('/var/log/caddy'), Path('/etc/logrotate.d/caddy')))):
    max_bytes = max_log_mb * MIB
    result = {'applied': apply, 'journal_vacuumed': False, 'rotated': [], 'eligible': []}
    journalctl = shutil.which('journalctl', path='/usr/bin:/bin:' + os.defpath)
    if journalctl and apply:
        subprocess.run([journalctl, f'--vacuum-size={journal_size}'],
                       capture_output=True, text=True, timeout=60, check=True)
        result['journal_vacuumed'] = True
    logrotate = shutil.which('logrotate', path='/usr/sbin:/sbin:/usr/bin:/bin')
    for name, log_dir, config in log_configs:
        large = oversized_logs(log_dir, max_bytes)
        if not large:
            continue
        result['eligible'].append(name)
        if apply and logrotate and config.is_file() and not config.is_symlink():
            subprocess.run([logrotate, '-f', str(config)],
                           capture_output=True, text=True, timeout=60, check=True)
            result['rotated'].append(name)
    return result


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--apply', action='store_true')
    parser.add_argument('--max-log-mb', type=int, default=50)
    parser.add_argument('--journal-size', default='100M')
    args = parser.parse_args()
    if args.max_log_mb < 1:
        parser.error('--max-log-mb must be at least 1')
    try:
        print(json.dumps(maintain(apply=args.apply, max_log_mb=args.max_log_mb, journal_size=args.journal_size)))
    except (OSError, subprocess.SubprocessError) as exc:
        parser.exit(1, f'Log maintenance failed ({type(exc).__name__}).\n')


if __name__ == '__main__':
    main()
