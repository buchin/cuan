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
import subprocess

GO_BINARY = "/usr/bin/go"

ACCOUNT = re.compile(r"acct_[0-9a-f]{32}")
VERSION = r"v0\.0\.1-go[0-9]+\.[0-9]+\.[0-9]+(?:[a-z]+[0-9]+)?\.linux-(?:amd64|arm64)"
TOOLCHAIN = re.compile('toolchain@' + VERSION)
ARCHIVE = re.compile(VERSION + r'\.zip')


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
        flags = os.O_RDWR | os.O_NOFOLLOW | os.O_CLOEXEC | os.O_NONBLOCK
        # Old idle profiles may predate account leasing. Only their owner may
        # create the same gate the runtime uses; root must not leave a root-owned
        # 0600 file that would prevent the provider from signing in or running.
        if os.geteuid() == account.stat().st_uid:
            flags |= os.O_CREAT
        descriptor = os.open(gate, flags, 0o600)
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
        try:
            descriptors = list((process / 'fd').iterdir())
        except OSError:
            descriptors = []
        for link in [process / 'exe', process / 'cwd', *descriptors]:
            try:
                if link.resolve(strict=True).is_relative_to(path):
                    return True
            except (OSError, RuntimeError):
                continue
    return False


def prune_downloads(modules, result, *, apply, min_age_hours, now, in_use):
    directory = modules
    for part in ('cache', 'download', 'golang.org', 'toolchain', '@v'):
        directory /= part
        if not private_directory(directory):
            return
    for archive in directory.iterdir():
        if not ARCHIVE.fullmatch(archive.name) or archive.is_symlink() or not archive.is_file():
            continue
        marker = archive.with_suffix('.ziphash')
        lock = archive.with_suffix('.lock')
        try:
            fd = os.open(lock, os.O_RDWR | os.O_NOFOLLOW | os.O_CLOEXEC | os.O_NONBLOCK)
        except OSError:
            continue
        with os.fdopen(fd, 'r+b') as lease:
            try:
                if not stat.S_ISREG(os.fstat(fd).st_mode):
                    continue
                fcntl.flock(fd, fcntl.LOCK_EX | fcntl.LOCK_NB)
                original = archive.lstat()
                if not stat.S_ISREG(original.st_mode) or now-original.st_mtime < min_age_hours*3600:
                    continue
                if marker.is_symlink() or not marker.is_file() or marker.stat().st_size > 128:
                    continue
                if not re.fullmatch(r'h1:[A-Za-z0-9+/]{43}=', marker.read_text().strip()) or in_use(archive):
                    continue
                result['eligible'] += 1
                result['download_eligible'] += 1
                if apply:
                    current = archive.lstat()
                    if (original.st_dev, original.st_ino) != (current.st_dev, current.st_ino):
                        continue
                    archive.unlink()  # Retain the checksum and download lock inode.
                    result['removed'] += 1
                    result['download_removed'] += 1
                    result['download_bytes_removed'] += original.st_size
            except (OSError, UnicodeError):
                continue


def remove_toolchain(path):
    # Go distributions in the module cache contain read-only directories. The
    # owning service can remove its own cache without elevated privileges.
    directories = [Path(root) for root, _, _ in os.walk(path, followlinks=False)]
    for directory in directories:
        info = directory.lstat()
        if not stat.S_ISDIR(info.st_mode) or (os.geteuid() != 0 and info.st_uid != os.geteuid()):
            raise OSError('Cache directory is not owned by the maintenance user')
    for directory in directories:
        os.chmod(directory, stat.S_IMODE(directory.lstat().st_mode) | stat.S_IWUSR, follow_symlinks=False)
    shutil.rmtree(path)


def prune(accounts, *, apply=False, min_age_hours=24, now=None, in_use=process_uses, all_modules=False):
    if min_age_hours < 1 or not private_directory(accounts):
        raise ValueError('Use a real accounts directory and retain at least one hour')
    now = time.time() if now is None else now
    result = {'eligible': 0, 'removed': 0, 'busy_accounts': 0,
              'download_eligible': 0, 'download_removed': 0, 'download_bytes_removed': 0,
              'module_caches_eligible': 0, 'module_caches_removed': 0}
    for account in sorted(accounts.iterdir()):
        if not ACCOUNT.fullmatch(account.name) or not private_directory(account):
            continue
        with idle_account(account) as idle:
            if not idle:
                result['busy_accounts'] += 1
                continue
            relative = account
            for part in ('home', 'go', 'pkg', 'mod'):
                relative /= part
                if not private_directory(relative):
                    break
            else:
                if all_modules:
                    # Explicit migration cleanup only: generated modules, not CLI
                    # credentials/history. The account lease spans the official
                    # Go cleaner; no model can start using this legacy cache.
                    if now-relative.stat().st_mtime < min_age_hours*3600 or in_use(relative):
                        continue
                    result['module_caches_eligible'] += 1
                    if apply:
                        subprocess.run([GO_BINARY, 'clean', '-modcache'], cwd='/',
                                       env={'PATH': os.defpath, 'GOENV': 'off', 'GOTOOLCHAIN': 'local',
                                            'HOME': str(account / 'home'), 'GOMODCACHE': str(relative)},
                                       check=True, capture_output=True, timeout=60)
                        result['module_caches_removed'] += 1
                    continue
                distributions = relative / 'golang.org'
                for toolchain in distributions.iterdir() if private_directory(distributions) else []:
                    if not TOOLCHAIN.fullmatch(toolchain.name) or not private_directory(toolchain):
                        continue
                    if now - toolchain.stat().st_mtime < min_age_hours * 3600 or in_use(toolchain):
                        continue
                    # Only the known, redownloadable Go distribution tree qualifies.
                    if not (toolchain / 'bin/go').is_file() or not (toolchain / 'VERSION').is_file():
                        continue
                    result['eligible'] += 1
                    if apply:
                        remove_toolchain(toolchain)
                        result['removed'] += 1
                prune_downloads(relative, result, apply=apply, min_age_hours=min_age_hours, now=now, in_use=in_use)
    return result


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--accounts', required=True, type=Path)
    parser.add_argument('--min-age-hours', default=24, type=float)
    parser.add_argument('--apply', action='store_true')
    parser.add_argument('--all-modules', action='store_true', help='Explicitly clean old idle legacy Go module caches with official Go; preserve account credentials/history')
    args = parser.parse_args()
    try:
        print(json.dumps(prune(args.accounts.absolute(), apply=args.apply, min_age_hours=args.min_age_hours, all_modules=args.all_modules)))
    except (ValueError, OSError, subprocess.SubprocessError) as exc:
        parser.exit(1, f'Cache maintenance failed ({type(exc).__name__}); account data preserved.\n')


if __name__ == '__main__':
    main()
