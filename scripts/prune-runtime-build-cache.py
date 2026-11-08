#!/usr/bin/env python3
"""Release idle managed build caches only under measured disk pressure."""
import argparse
import importlib.util
import fcntl
import json
import os
from pathlib import Path
import re
import stat
import shutil
import subprocess
import tempfile
import time

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


def clean(cache, identity, kind="go"):
    credentials = {}
    if os.geteuid() == 0 and identity.st_uid != 0:
        credentials = dict(user=identity.st_uid, group=identity.st_gid, extra_groups=[])
    if kind in ('npm', 'composer'):
        if kind == 'composer':
            with tempfile.TemporaryDirectory(prefix='cuan-composer-maintenance-') as temporary:
                home = Path(temporary)
                if credentials:
                    os.chown(home, identity.st_uid, identity.st_gid)
                executable = shutil.which('composer', path='/usr/local/bin:' + os.defpath)
                if executable is None:
                    raise ValueError('Composer maintenance runtime unavailable')
                subprocess.run([executable, '--no-plugins', '--no-scripts', 'clear-cache'],
                               cwd=home, env={'PATH': os.defpath, 'HOME': str(home),
                                              'COMPOSER_HOME': str(home), 'COMPOSER_CACHE_DIR': str(cache)},
                               capture_output=True, timeout=60, check=True, **credentials)
            return
        with tempfile.TemporaryDirectory(prefix='cuan-cache-maintenance-') as temporary:
            home = Path(temporary)
            if credentials:
                os.chown(home, identity.st_uid, identity.st_gid)
            for name in ('user.npmrc', 'global.npmrc'):
                (home / name).write_text('')
            subprocess.run(['/usr/bin/npm', 'cache', 'clean', '--force', '--cache', str(cache),
                            '--userconfig', str(home / 'user.npmrc'),
                            '--globalconfig', str(home / 'global.npmrc')],
                           cwd='/', env={'PATH': os.defpath, 'HOME': str(home)},
                           capture_output=True, timeout=60, check=True, **credentials)
        return
    # Official Go owns cache internals. No repository, shell, user GOENV or
    # automatic toolchain download participates in maintenance.
    subprocess.run(['/usr/bin/go', 'clean', '-cache'], cwd='/',
                   env={'PATH': os.defpath, 'GOCACHE': str(cache),
                        'GOENV': 'off', 'GOTOOLCHAIN': 'local'},
                   capture_output=True, timeout=60, check=True, **credentials)


def prune(base, *, apply=False, free=available, in_use=referenced, cleaner=clean, kind="go"):
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
        cache = owner / {'go': 'go-build', 'npm': 'npm-cache', 'composer': 'composer-cache'}[kind]
        gate = owner / '.build-cache.lock'
        if not directory(cache) or cache.stat().st_dev != base.stat().st_dev:
            continue
        if kind == 'go':
            marker = cache / 'README'
            if marker.is_symlink() or not marker.is_file() or marker.stat().st_size > 4096:
                continue
            if not marker.read_text().startswith('This directory holds cached build artifacts from the Go build system.'):
                continue
        elif kind == 'composer':
            if not directory(cache / 'files'):
                continue
        elif not npm_cache(cache):
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
            if kind == 'go' and not any(re.fullmatch('[0-9a-f]{2}', p.name) and directory(p) for p in cache.iterdir()):
                continue
            result['eligible'] += 1
            if apply:
                if kind == "go":
                    cleaner(cache, identity)
                else:
                    cleaner(cache, identity, kind)
                result['removed'] += 1
    return result


def npm_cache(cache):
    return all(directory(path) for path in (cache, cache / '_cacache',
                                           cache / '_cacache/content-v2', cache / '_cacache/index-v5'))


def prune_legacy_npm(accounts, *, apply=False, free=available, in_use=referenced, cleaner=clean):
    result = dict(pressure=False, eligible=0, removed=0, busy=0)
    if not directory(accounts) or free(accounts) >= 3 * GIB:
        return result
    result['pressure'] = True
    # Reuse the exact provider execution gate; never invent a different lock or
    # create a root-owned legacy gate that would break provider sign-in.
    spec = importlib.util.spec_from_file_location('provider_cache_lease', Path(__file__).with_name('prune-provider-go-cache.py'))
    provider = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(provider)
    for account in sorted(accounts.iterdir()):
        if free(accounts) >= 4 * GIB:
            break
        if not provider.ACCOUNT.fullmatch(account.name) or not directory(account):
            continue
        cache = account / 'home/.npm'
        if not npm_cache(cache) or cache.stat().st_dev != accounts.stat().st_dev:
            continue
        if cache.stat().st_uid != account.stat().st_uid:
            continue
        if time.time() - (cache / '_cacache').stat().st_mtime < 3600:
            continue
        with provider.idle_account(account) as idle:
            if not idle or in_use(account):
                result['busy'] += 1
                continue
            result['eligible'] += 1
            if apply:
                cleaner(cache, cache.stat(), 'npm')
                result['removed'] += 1
    return result


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--runtime', required=True, type=Path)
    parser.add_argument('--apply', action='store_true')
    parser.add_argument('--kind', choices=('go','npm','composer'), default='go')
    parser.add_argument('--legacy-accounts', type=Path, help='Also consider idle legacy npm download caches')
    args = parser.parse_args()
    try:
        result = prune(args.runtime.absolute(), apply=args.apply, kind=args.kind)
        if args.legacy_accounts:
            if args.kind != 'npm':
                raise ValueError('Legacy npm caches require npm mode')
            result['legacy'] = prune_legacy_npm(args.legacy_accounts.absolute(), apply=args.apply)
        print(json.dumps(result))
    except (OSError, ValueError, subprocess.SubprocessError) as error:
        parser.exit(1, f'Build cache maintenance failed ({type(error).__name__}); saved work preserved.\n')


if __name__ == '__main__':
    main()
