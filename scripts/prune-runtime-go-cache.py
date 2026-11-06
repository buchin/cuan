#!/usr/bin/env python3
"""Release idle managed Go build caches only under measured disk pressure."""
import argparse
import fcntl
import json
import os
from pathlib import Path
import re
import stat
import subprocess

GIB = 1 << 30


def directory(path):
    try:
        return path.is_dir() and not path.is_symlink() and path.resolve() == path.absolute()
    except (OSError, RuntimeError):
        return False


def referenced(root, proc=Path('/proc')):
    """Protect old executions which predate the cooperative build-cache lease."""
    prefix = str(root).encode()
    for process in proc.glob('[0-9]*'):
        if process.name == str(os.getpid()):
            continue
        try:
            for name in ('environ', 'cmdline'):
                for entry in (process / name).read_bytes().split(b'\0'):
                    value = entry.split(b'=', 1)[-1]
                    if value == prefix or value.startswith(prefix + b'/'):
                        return True
            for link in [process / 'cwd', process / 'exe', *(process / 'fd').iterdir()]:
                try:
                    target = os.readlink(link)
                    if target == str(root) or target.startswith(str(root) + '/'):
                        return True
                except FileNotFoundError:
                    pass
        except (FileNotFoundError, ProcessLookupError):
            continue
        except OSError:
            return True
    return False


def available(path):
    filesystem = os.statvfs(path)
    return filesystem.f_bavail * filesystem.f_frsize


def clean(cache, identity):
    credentials = {}
    if os.geteuid() == 0 and identity.st_uid != 0:
        credentials = dict(user=identity.st_uid, group=identity.st_gid, extra_groups=[])
    # Official Go owns cache internals. No repository, shell, user GOENV or
    # automatic toolchain download participates in maintenance.
    subprocess.run(['/usr/bin/go', 'clean', '-cache'], cwd='/',
                   env={'PATH': os.defpath, 'GOCACHE': str(cache),
                        'GOENV': 'off', 'GOTOOLCHAIN': 'local'},
                   capture_output=True, timeout=60, check=True, **credentials)


def prune(base, *, apply=False, free=available, in_use=referenced, cleaner=clean):
    result = dict(pressure=False, eligible=0, removed=0, busy=0)
    if not directory(base):
        return result
    if free(base) >= 3 * GIB:
        return result
    result['pressure'] = True
    for owner in sorted(base.iterdir()):
        if free(base) >= 4 * GIB:
            break
        if not re.fullmatch('[0-9a-f]{64}', owner.name) or not directory(owner):
            continue
        cache, gate = owner / 'go-build', owner / '.build-cache.lock'
        if not directory(cache) or cache.stat().st_dev != base.stat().st_dev:
            continue
        marker = cache / 'README'
        if marker.is_symlink() or not marker.is_file() or marker.stat().st_size > 4096:
            continue
        if not marker.read_text().startswith('This directory holds cached build artifacts from the Go build system.'):
            continue
        try:
            # Never create missing legacy gates from root maintenance.
            descriptor = os.open(gate, os.O_RDWR | os.O_NOFOLLOW | os.O_NONBLOCK | os.O_CLOEXEC)
        except OSError:
            result['busy'] += 1
            continue
        with os.fdopen(descriptor, 'r+b') as lease:
            identity = owner.stat()
            info = os.fstat(lease.fileno())
            if not stat.S_ISREG(info.st_mode) or info.st_uid != identity.st_uid or cache.stat().st_uid != identity.st_uid:
                continue
            try:
                fcntl.flock(lease.fileno(), fcntl.LOCK_EX | fcntl.LOCK_NB)
            except BlockingIOError:
                result['busy'] += 1
                continue
            if in_use(owner):
                result['busy'] += 1
                continue
            # Empty caches need no repeated subprocess. Credentials/modules live
            # in sibling paths and are never passed to the cleaner.
            if not any(re.fullmatch('[0-9a-f]{2}', p.name) and directory(p) for p in cache.iterdir()):
                continue
            result['eligible'] += 1
            if apply:
                cleaner(cache, identity)
                result['removed'] += 1
    return result


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--runtime', required=True, type=Path)
    parser.add_argument('--apply', action='store_true')
    args = parser.parse_args()
    try:
        print(json.dumps(prune(args.runtime.absolute(), apply=args.apply)))
    except (OSError, ValueError, subprocess.SubprocessError) as error:
        parser.exit(1, f'Go cache maintenance failed ({type(error).__name__}); saved work preserved.\n')


if __name__ == '__main__':
    main()
