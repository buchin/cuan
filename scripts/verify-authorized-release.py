#!/usr/bin/env python3
"""Systemd pre-start gate for native-authorized Cuan release identities."""
import argparse
import json
from pathlib import Path
import re
import sys


def verify(base, release=''):
    if not base.is_absolute() or base.resolve() != base:
        raise ValueError('Invalid native release base')
    if release:
        if not re.fullmatch('[0-9a-f]{40}', release):
            raise ValueError('Invalid release identity')
        target = base / 'releases' / release
    else:
        current = base / 'current'
        if not current.is_symlink():
            raise ValueError('Native current release link is unavailable')
        target = current.resolve(strict=True)
        release = target.name
    if not re.fullmatch('[0-9a-f]{40}', release) or target != base / 'releases' / release or target.is_symlink() or not target.is_dir():
        raise ValueError('Release target is outside the native release directory')
    policy = base / 'maintenance/activation-policy/authorized.json'
    if policy.is_symlink() or not policy.is_file():
        raise ValueError('Native activation authorization is unavailable')
    if policy.stat().st_size > 4096:
        raise ValueError('Native activation authorization is invalid')
    saved = json.loads(policy.read_text())
    if not isinstance(saved, dict):
        raise ValueError('Native activation authorization is invalid')
    allowed = saved.get('allowed', [])
    if saved.get('version') != 1 or not isinstance(allowed, list) or not all(isinstance(sha, str) and re.fullmatch('[0-9a-f]{40}', sha) for sha in allowed) or release not in allowed:
        raise ValueError('Release has not passed native activation authorization; reconcile the saved branch with the current release')
    return release


if __name__ == '__main__':
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--base', type=Path, default=Path('/opt/cuan'))
    parser.add_argument('--release', default='')
    args = parser.parse_args()
    try:
        verify(args.base, args.release)
    except (OSError, ValueError, TypeError) as error:
        print(str(error), file=sys.stderr)
        sys.exit(1)
