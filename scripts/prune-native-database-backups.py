#!/usr/bin/env python3
"""Preview bounded retention of native pre-release PostgreSQL dumps."""
import argparse
import fcntl
import json
import os
from pathlib import Path
import re
import stat
import subprocess


def private_dump(path, owner):
    info = path.lstat()
    if (not stat.S_ISREG(info.st_mode) or info.st_nlink != 1 or
            info.st_uid != owner or info.st_mode & 0o077 or info.st_size < 5):
        return None
    with path.open('rb') as source:
        if source.read(5) != b'PGDMP':
            return None
    return info


def plan(base, keep=3):
    if keep < 2 or base.resolve() != base or base.is_symlink():
        raise ValueError('Use a real absolute installation and bounded retention')
    backups = base / 'backups'
    if backups.is_symlink() or not backups.is_dir():
        raise ValueError('Native backup directory unavailable')
    protected = set()
    releases = base / 'releases'
    for name in ('current', 'previous'):
        link = base / name
        target = link.resolve(strict=True)
        if not link.is_symlink() or target.parent != releases or not re.fullmatch('[a-f0-9]{40}', target.name):
            raise ValueError('Current and previous native releases must be known')
        protected.add(target.name)
    entries = []
    ignored = 0
    for path in backups.iterdir():
        if not re.fullmatch(r'[a-f0-9]{40}\.dump', path.name):
            ignored += 1
            continue
        info = private_dump(path, os.geteuid())
        if info is None:
            ignored += 1
            continue
        entries.append((path, info))
    entries.sort(key=lambda item: (item[1].st_mtime, item[0].name), reverse=True)
    protected.update(path.stem for path, _ in entries[:keep])
    eligible = [(path, info) for path, info in entries if path.stem not in protected]
    return entries, eligible, ignored


def validate_dump(path):
    # List only: no database connection, restore, credentials or dump SQL output.
    subprocess.run(['pg_restore', '--list', str(path)], stdin=subprocess.DEVNULL,
                   stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL,
                   timeout=15, check=True,
                   env={'PATH': os.defpath, 'LC_ALL': 'C'})


def prune(base, apply=False, keep=3, validate=validate_dump):
    gate = base / '.release.lock'
    descriptor = os.open(gate, os.O_RDWR | os.O_NOFOLLOW | os.O_NONBLOCK | os.O_CLOEXEC)
    with os.fdopen(descriptor, 'r+b'):
        identity = os.fstat(descriptor)
        if not stat.S_ISREG(identity.st_mode) or identity.st_nlink != 1 or identity.st_uid != os.geteuid():
            raise ValueError('Native release gate identity differs')
        try:
            fcntl.flock(descriptor, fcntl.LOCK_EX | fcntl.LOCK_NB)
        except BlockingIOError:
            return {'applied': False, 'skipped': 'release_busy'}
        entries, eligible, ignored = plan(base, keep)
        result = dict(applied=apply, eligible=len(eligible), removed=0,
                      eligible_allocated_bytes=sum(info.st_blocks * 512 for _, info in eligible),
                      retained=len(entries)-len(eligible), ignored=ignored,
                      keep_latest=keep)
        if not apply or not eligible:
            return result
        if len(entries) < 2:
            raise ValueError('Two recent valid restore points are required')
        for path, _ in entries[:2]:
            validate(path)
        for path, _ in eligible:
            validate(path)
        for path, original in eligible:
            # Preserve malformed/unreadable historical evidence rather than
            # treating corruption as permission to remove the last copy.
            current = private_dump(path, os.geteuid())
            if current is None or (current.st_dev, current.st_ino, current.st_size, current.st_mtime_ns) != (
                    original.st_dev, original.st_ino, original.st_size, original.st_mtime_ns):
                raise ValueError('Backup identity changed during retention')
            path.unlink()
            result['removed'] += 1
        directory = os.open(base / 'backups', os.O_RDONLY | os.O_DIRECTORY | os.O_NOFOLLOW)
        try:
            os.fsync(directory)
        finally:
            os.close(directory)
        return result


if __name__ == '__main__':
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--base', type=Path, default=Path('/opt/cuan'))
    parser.add_argument('--keep', type=int, default=3)
    parser.add_argument('--apply', action='store_true', help='Permanently remove eligible old native backups; default previews only')
    args = parser.parse_args()
    try:
        print(json.dumps(prune(args.base, args.apply, args.keep)))
    except (OSError, ValueError, subprocess.SubprocessError) as error:
        parser.exit(1, f'Backup retention stopped ({type(error).__name__}); no restore attempted.\n')
