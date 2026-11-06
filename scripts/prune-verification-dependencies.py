#!/usr/bin/env python3
"""Remove only idle, regenerable npm dependencies from owned verification snapshots."""
import argparse
import fcntl
import json
import os
from pathlib import Path
import re
import shutil
import stat
import subprocess
import sys
import time
from urllib.parse import urlsplit


def owner_credentials(root):
    identity = root.stat()
    if os.geteuid() == 0 and identity.st_uid != 0:
        return {'user': identity.st_uid, 'group': identity.st_gid, 'extra_groups': []}
    return {}


def git(root, *arguments):
    environment = {key: os.environ[key] for key in ('PATH', 'LANG', 'LD_LIBRARY_PATH') if key in os.environ}
    environment.update(GIT_CONFIG_GLOBAL='/dev/null', GIT_CONFIG_NOSYSTEM='1', GIT_OPTIONAL_LOCKS='0')
    # Git may run repository-defined clean filters while checking status.
    # Root maintenance must never execute those with elevated privileges.
    return subprocess.run(['git', '-c', 'safe.directory=' + str(root),
                           '-c', 'core.hooksPath=/dev/null', '-c', 'core.fsmonitor=false',
                           '-C', str(root), *arguments], env=environment,
                          capture_output=True, timeout=10, check=True, **owner_credentials(root)).stdout


def remove_dependencies(root, modules, original):
    credentials = owner_credentials(root)
    if not credentials:
        shutil.rmtree(modules)
        return
    # Keep process inspection privileged, but delete as the service user. Even
    # a hostile path replacement cannot turn maintenance into root file deletion.
    code = '''import os,shutil,stat,sys
path=sys.argv[1]
info=os.lstat(path)
if not stat.S_ISDIR(info.st_mode) or (info.st_dev,info.st_ino)!=(int(sys.argv[2]),int(sys.argv[3])):
    raise SystemExit(1)
shutil.rmtree(path)
'''
    subprocess.run([sys.executable, '-I', '-c', code, str(modules), str(original.st_dev), str(original.st_ino)],
                   cwd='/', env={'PATH': os.defpath}, capture_output=True, timeout=60,
                   check=True, **credentials)


def referenced(root, proc=Path('/proc')):
    """Protect legacy/non-runner readers as well as the cooperative OS lease."""
    prefix = str(root).encode()
    for process in proc.glob('[0-9]*'):
        try:
            if any(argument.split(b'=', 1)[-1] == prefix or argument.split(b'=', 1)[-1].startswith(prefix + b'/')
                   for argument in (process / 'cmdline').read_bytes().split(b'\0')):
                return True
            for link in [process / 'cwd', process / 'exe', *(process / 'fd').iterdir()]:
                try:
                    target = os.readlink(link)
                    if process.name == str(os.getpid()) and target == str(root / '.git/cuan-verification.lock'):
                        continue  # This pruner's own cooperative lease.
                    if target == str(root) or target.startswith(str(root) + '/'):
                        return True
                except FileNotFoundError:
                    pass  # Exiting process or a descriptor closed during observation.
        except (FileNotFoundError, ProcessLookupError):
            continue
        except (PermissionError, OSError):
            return True  # Unknown reader is not proof that deletion is safe.
    return False


def mounted_dependency(modules):
    try:
        for line in Path('/proc/self/mountinfo').read_text().splitlines():
            target = re.sub(r'\\([0-7]{3})', lambda match: chr(int(match[1], 8)), line.split()[4])
            if target == str(modules) or target.startswith(str(modules) + '/'):
                return True
        return False
    except (OSError, IndexError):
        return True


def dependency_paths(root, now, min_age):
    metadata = root / '.git'
    if metadata.is_symlink() or not metadata.is_dir():
        return []
    receipt = metadata / 'cuan-verification.json'
    if receipt.is_symlink() or not receipt.is_file() or receipt.stat().st_size > 4096:
        return []
    binding = json.loads(receipt.read_text())
    if not isinstance(binding, dict):
        return []
    if not all(re.fullmatch('[0-9a-f]{40}', binding.get(name, ''))
               for name in ('source_commit', 'target_commit')):
        return []
    if git(root, 'rev-parse', 'HEAD').decode().strip() != binding['target_commit']:
        return []
    if git(root, 'status', '--porcelain=v1', '-z', '--untracked-files=all'):
        return []
    result = []
    for relative in (Path('.'), Path('web')):
        directory = root / relative
        modules = directory / 'node_modules'
        if directory.is_symlink() or modules.is_symlink() or not modules.is_dir():
            continue
        if now - modules.stat().st_mtime < min_age or mounted_dependency(modules):
            continue
        names = [str(relative / name) for name in ('package.json', 'package-lock.json')]
        for name in names:
            file = root / name
            if file.is_symlink() or not file.is_file() or file.stat().st_size > 8 << 20:
                break
        else:
            git(root, 'ls-files', '--error-unmatch', '--', *names)
            path = str(relative / 'node_modules')
            if git(root, 'ls-files', '-z', '--', path):
                continue
            git(root, 'check-ignore', '--quiet', '--', path + '/')
            lock = json.loads((directory / 'package-lock.json').read_text())
            if not isinstance(lock, dict) or type(lock.get('lockfileVersion')) is not int or not 2 <= lock['lockfileVersion'] <= 2**31-1 or not isinstance(lock.get('packages'), dict):
                continue
            for name, package in lock['packages'].items():
                if not name:
                    continue
                if not isinstance(package, dict) or not isinstance(package.get('resolved', ''), str) or any(type(package.get(key, False)) is not bool for key in ('link', 'inBundle')):
                    break
                endpoint = urlsplit(package.get('resolved', ''))
                if package.get('link') or (not package.get('inBundle') and
                    (endpoint.scheme != 'https' or not endpoint.hostname or endpoint.username is not None)):
                    break
            else:
                result.append(modules)
    return result


def prune(snapshots, apply=False, min_age=3600, now=None, proc=Path('/proc')):
    now = time.time() if now is None else now
    report = {'eligible': [], 'removed': [], 'busy': 0, 'sourceDataTouched': False}
    if snapshots.is_symlink() or not snapshots.is_dir() or snapshots.resolve() != snapshots:
        return report
    if not shutil.rmtree.avoids_symlink_attacks:
        return report
    for root in sorted(snapshots.iterdir()):
        if root.is_symlink() or not root.is_dir() or not re.fullmatch('[0-9a-f]{64}', root.name):
            continue
        gate = root / '.git/cuan-verification.lock'
        # Old runners do not participate in this lease. Missing gates are kept.
        try:
            fd = os.open(gate, os.O_RDWR | os.O_NOFOLLOW | os.O_NONBLOCK | os.O_CLOEXEC)
        except OSError:
            continue
        with os.fdopen(fd, 'r+b') as lease:
            try:
                info = os.fstat(fd)
                if not stat.S_ISREG(info.st_mode) or info.st_nlink != 1 or info.st_uid != root.stat().st_uid:
                    continue
                fcntl.flock(fd, fcntl.LOCK_EX | fcntl.LOCK_NB)
                if referenced(root, proc):
                    report['busy'] += 1
                    continue
                paths = dependency_paths(root, now, min_age)
                if paths and referenced(root, proc):
                    report['busy'] += 1
                    continue
                for modules in paths:
                    original = modules.stat()
                    report['eligible'].append(str(modules.relative_to(snapshots)))
                    if apply:
                        current = modules.lstat()
                        if not stat.S_ISDIR(current.st_mode) or (original.st_dev, original.st_ino) != (current.st_dev, current.st_ino):
                            continue
                        # Delete only the ignored dependency directory, never the
                        # snapshot, source, git metadata, reports or unknown files.
                        remove_dependencies(root, modules, original)
                        report['removed'].append(str(modules.relative_to(snapshots)))
            except BlockingIOError:
                report['busy'] += 1
            except (OSError, ValueError, TypeError, subprocess.SubprocessError):
                continue
    return report


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--snapshots', type=Path, required=True)
    parser.add_argument('--min-age-hours', type=float, default=1)
    parser.add_argument('--apply', action='store_true', help='Remove eligible dependency caches; default is preview only')
    args = parser.parse_args()
    if not args.snapshots.is_absolute() or args.min_age_hours < 1:
        parser.error('Use an absolute snapshot root and a minimum age of at least one hour')
    print(json.dumps(prune(args.snapshots, args.apply, args.min_age_hours * 3600)))


if __name__ == '__main__':
    main()
