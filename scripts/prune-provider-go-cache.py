#!/usr/bin/env python3
"""Remove idle legacy per-provider Go toolchains; never touch account credentials."""
import argparse
import contextlib
import fcntl
import json
import os
from pathlib import Path
import re
import shutil
import stat
import time

ACCOUNT = re.compile(r"acct_[0-9a-f]{32}")
TOOLCHAIN = re.compile(r"toolchain@v0\.0\.1-go[0-9]+\.[0-9]+\.[0-9]+(?:[a-z]+[0-9]+)?\.linux-(?:amd64|arm64)")


def private_directory(path):
    try:
        return path.is_dir() and not path.is_symlink() and path.resolve() == path.absolute()
    except (OSError, RuntimeError):
        return False


@contextlib.contextmanager
def idle_account(account):
    """Same exclusive gate as provider leasing; any live session holds it shared."""
    gate = account / '.execution.lock'
    descriptor = None
    try:
        descriptor = os.open(gate, os.O_RDWR | os.O_NOFOLLOW | os.O_CLOEXEC | os.O_NONBLOCK)
        if not stat.S_ISREG(os.fstat(descriptor).st_mode):
            raise OSError("Account gate is not a regular file")
        fcntl.flock(descriptor, fcntl.LOCK_EX | fcntl.LOCK_NB)
    except (OSError, BlockingIOError):
        if descriptor is not None:
            os.close(descriptor)
        yield False
        return
    try:
        yield True
    finally:
        os.close(descriptor)


def process_uses(path, proc=Path('/proc')):
    for process in proc.glob('[0-9]*'):
        for kind in ('exe', 'cwd'):
            try:
                if (process / kind).resolve(strict=True).is_relative_to(path):
                    return True
            except (OSError, RuntimeError):
                continue
    return False


def prune(accounts, *, apply=False, min_age_hours=24, now=None, in_use=process_uses):
    if min_age_hours < 1 or not private_directory(accounts):
        raise ValueError('Use a real accounts directory and retain at least one hour')
    now = time.time() if now is None else now
    result = {'eligible': 0, 'removed': 0, 'busy_accounts': 0}
    for account in sorted(accounts.iterdir()):
        if not ACCOUNT.fullmatch(account.name) or not private_directory(account):
            continue
        with idle_account(account) as idle:
            if not idle:
                result['busy_accounts'] += 1
                continue
            relative = account
            for part in ('home', 'go', 'pkg', 'mod', 'golang.org'):
                relative /= part
                if not private_directory(relative):
                    break
            else:
                for toolchain in relative.iterdir():
                    if not TOOLCHAIN.fullmatch(toolchain.name) or not private_directory(toolchain):
                        continue
                    if now - toolchain.stat().st_mtime < min_age_hours * 3600 or in_use(toolchain):
                        continue
                    # Only the known, redownloadable Go distribution tree qualifies.
                    if not (toolchain / 'bin/go').is_file() or not (toolchain / 'VERSION').is_file():
                        continue
                    result['eligible'] += 1
                    if apply:
                        shutil.rmtree(toolchain)
                        result['removed'] += 1
    return result


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--accounts', required=True, type=Path)
    parser.add_argument('--min-age-hours', default=24, type=float)
    parser.add_argument('--apply', action='store_true')
    args = parser.parse_args()
    try:
        print(json.dumps(prune(args.accounts.absolute(), apply=args.apply, min_age_hours=args.min_age_hours)))
    except (ValueError, OSError) as exc:
        parser.exit(1, f'Cache maintenance failed ({type(exc).__name__}); account data preserved.\n')


if __name__ == '__main__':
    main()
