#!/usr/bin/env python3
"""Prune regenerable caches of idle native release workspaces; preserve sources."""
import argparse
import fcntl
import importlib.util
import json
import os
from pathlib import Path
import re
import stat
import subprocess


def load_helpers():
    spec = importlib.util.spec_from_file_location('dependencies', Path(__file__).with_name('prune-verification-dependencies.py'))
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


def private_directory(path):
    return path.is_dir() and not path.is_symlink() and path.resolve() == path.absolute()


def prune(base, apply=False, force=False):
    helpers = load_helpers()
    result = dict(eligible=[], cleaned=[], busy=0, sourceDataTouched=False)
    if not private_directory(base):
        return result
    free = lambda: os.statvfs(base).f_bavail * os.statvfs(base).f_frsize
    if not force and free() >= 4 * (1 << 30):
        return result
    for root in sorted(base.iterdir()):
        if not force and free() >= 8 * (1 << 30):
            break
        if not re.fullmatch('[0-9a-f]{64}', root.name) or not private_directory(root):
            continue
        metadata = root / '.git'
        runtime = metadata / 'cuan-native-runtime'
        if not private_directory(metadata) or not private_directory(runtime):
            continue
        try:
            receipt = metadata / 'cuan-verification.json'
            if receipt.is_symlink() or not receipt.is_file() or receipt.stat().st_size > 4096:
                continue
            binding = json.loads(receipt.read_text())
            if not isinstance(binding, dict):
                continue
            target = binding.get('target_commit', '')
            if not re.fullmatch('[0-9a-f]{40}', target) or helpers.git(root, 'rev-parse', 'HEAD').decode().strip() != target:
                continue
            fd = os.open(metadata / 'cuan-verification.lock', os.O_RDWR | os.O_NOFOLLOW | os.O_CLOEXEC)
            with os.fdopen(fd, 'r+b') as lease:
                info = os.fstat(fd)
                if not stat.S_ISREG(info.st_mode) or info.st_nlink != 1 or info.st_uid != root.stat().st_uid:
                    continue
                fcntl.flock(lease, fcntl.LOCK_EX | fcntl.LOCK_NB)
                if helpers.referenced(root):
                    result['busy'] += 1
                    continue
                paths = [runtime / relative for relative in ('cache/go', 'cache/npm', 'data/go/pkg/mod')]
                # Resolve every ancestor, not just the final cache directory.
                if any(path.exists() and not private_directory(path) for path in paths):
                    continue
                has_go = any(p.is_dir() and any(p.iterdir()) for p in (paths[0], paths[2]))
                has_npm = paths[1].is_dir() and any(paths[1].iterdir())
                if not has_go and not has_npm:
                    continue
                result['eligible'].append(root.name)
                if not apply:
                    continue
                credentials = helpers.owner_credentials(root)
                env = {'PATH': os.defpath, 'HOME': str(runtime / 'home'), 'GOENV': 'off', 'GOTOOLCHAIN': 'local',
                       'GOCACHE': str(paths[0]), 'GOMODCACHE': str(paths[2])}
                if helpers.referenced(root):
                    result['busy'] += 1
                    continue
                if has_go:
                    subprocess.run(['/usr/bin/go', 'clean', '-cache', '-modcache'], cwd='/', env=env,
                                   capture_output=True, timeout=120, check=True, **credentials)
                if has_npm:
                    spec = importlib.util.spec_from_file_location('build_cache', Path(__file__).with_name('prune-runtime-build-cache.py'))
                    build_cache = importlib.util.module_from_spec(spec)
                    spec.loader.exec_module(build_cache)
                    build_cache.clean(paths[1], root.stat(), kind='npm')
                result['cleaned'].append(root.name)
        except BlockingIOError:
            result['busy'] += 1
        except (OSError, ValueError, subprocess.SubprocessError):
            result['busy'] += 1
    return result


if __name__ == '__main__':
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--workspaces', type=Path, required=True)
    parser.add_argument('--apply', action='store_true')
    parser.add_argument('--force', action='store_true', help='Clean idle caches even without measured storage pressure')
    args = parser.parse_args()
    print(json.dumps(prune(args.workspaces, args.apply, args.force)))
