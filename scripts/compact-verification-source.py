#!/usr/bin/env python3
"""Release clean accepted verifier file copies, retaining independent Git objects."""
import argparse
import datetime
import fcntl
import hashlib
import importlib.util
import json
import os
from pathlib import Path
import re
import stat
import subprocess
import time

spec = importlib.util.spec_from_file_location('verification_cache', Path(__file__).with_name('prune-verification-dependencies.py'))
cache = importlib.util.module_from_spec(spec)
spec.loader.exec_module(cache)


DEFAULT_CONFIG = (
    b'{\n'
    b'  "mcpServers": {\n'
    b'    "cuan-inspect": {\n'
    b'      "args": [\n'
    b'        "mcp"\n'
    b'      ],\n'
    b'      "command": "cuan"\n'
    b'    }\n'
    b'  }\n'
    b'}\n'
)
DEFAULT_CONFIG_SHA256 = hashlib.sha256(DEFAULT_CONFIG).hexdigest()
ALLOWED_CONFIGS = {
    '.agents': '.agents/mcp_config.json',
    '.gemini': '.gemini/config/mcp_config.json',
}


def receipt(path, owner):
    info = path.lstat()
    if not stat.S_ISREG(info.st_mode) or info.st_size > 4096 or info.st_uid != owner or info.st_nlink != 1:
        raise ValueError('Unproven snapshot receipt')
    data = json.loads(path.read_bytes())
    if not isinstance(data, dict):
        raise ValueError('Unproven snapshot receipt')
    return data


def generated_defaults(root, accepted, status):
    if accepted is not None:
        configs = accepted.get('generated_configs')
        if not isinstance(configs, dict) or not set(configs).issubset(set(ALLOWED_CONFIGS.values())):
            return False
        for name, digest in configs.items():
            if digest != DEFAULT_CONFIG_SHA256:
                return False
    else:
        lines = [line.strip() for line in status.splitlines() if line.strip()]
        configs = {}
        for line in lines:
            if not line.startswith(b'!! ') or not line.endswith(b'/'):
                return False
            name = line[3:-1].decode('utf-8', errors='replace')
            if name not in ALLOWED_CONFIGS:
                return False
            configs[ALLOWED_CONFIGS[name]] = DEFAULT_CONFIG_SHA256

    parents = set()
    for name, digest in configs.items():
        if not isinstance(digest, str) or not re.fullmatch('[a-f0-9]{64}', digest):
            return False
        path = root / name
        if path.is_symlink() or not path.is_file() or path.stat().st_size > 4096 or hashlib.sha256(path.read_bytes()).hexdigest() != digest:
            return False
        parents.add(name.split('/')[0])
    expected = {b'!! ' + name.encode() + b'/' for name in parents}
    if set(status.splitlines()) != expected:
        return False
    permitted = set(configs)
    for name in configs:
        permitted.update(str(parent) for parent in Path(name).parents if str(parent) != '.')
    for name in parents:
        directory = root / name
        if directory.is_symlink():
            return False
        for entry in directory.rglob('*'):
            if entry.is_symlink() or str(entry.relative_to(root)) not in permitted:
                return False
    return True


def candidate(root, now, minimum):
    metadata = root / '.git'
    if metadata.is_symlink() or not metadata.is_dir() or os.path.lexists(metadata / 'cuan-verification-cold.json'):
        return None, 0, 'unavailable_or_cold'
    owner = root.stat().st_uid
    if metadata.stat().st_uid != owner or os.path.lexists(metadata / 'objects/info/alternates'):
        return None, 0, 'unproven'
    binding_path = metadata / 'cuan-verification.json'
    if not os.path.lexists(binding_path):
        return None, 0, 'unproven'
    binding = receipt(binding_path, owner)
    if not isinstance(binding.get('source'), str) or not os.path.isabs(binding['source']):
        return None, 0, 'unproven'
    for name in ('source_commit', 'target_commit'):
        if not re.fullmatch('[a-f0-9]{40}', binding.get(name, '')):
            return None, 0, 'unproven'

    accepted_path = metadata / 'cuan-verification-accepted.json'
    accepted = None
    if os.path.lexists(accepted_path):
        accepted = receipt(accepted_path, owner)
        if accepted.get('policy') != 'retained-git-source-v1' or not isinstance(accepted.get('task_id'), str) or not accepted['task_id']:
            return None, 0, 'unaccepted'
        for name in ('source_commit', 'target_commit'):
            if accepted.get(name) != binding[name]:
                return None, 0, 'unproven'
        if not isinstance(accepted.get('accepted_at'), str):
            return None, 0, 'unproven'
        timestamp = accepted['accepted_at'].replace('Z', '+00:00')
        # Go uses RFC3339Nano, including trimmed fractions. Python3.10 on native
        # Ubuntu hosts needs a microsecond fraction; retention tolerates truncation.
        timestamp = re.sub(r'\.(\d+)(?=[+-]\d\d:\d\d$)', lambda match: '.' + (match[1] + '000000')[:6], timestamp)
        try:
            accepted_at = datetime.datetime.fromisoformat(timestamp)
        except ValueError:
            return None, 0, 'unproven'
        if accepted_at.tzinfo is None or now - max(accepted_at.timestamp(), root.stat().st_mtime) < minimum:
            return None, 0, 'warm'
    else:
        if now - root.stat().st_mtime < minimum:
            return None, 0, 'warm'

    if cache.git(root, 'rev-parse', 'HEAD').decode().strip() != binding['target_commit']:
        return None, 0, 'changed'
    cache.git(root, 'merge-base', '--is-ancestor', binding['source_commit'], binding['target_commit'])
    if not generated_defaults(root, accepted, cache.git(root, 'status', '--porcelain=v1', '--ignored')):
        return None, 0, 'unique_files'
    if cache.mounted_dependency(root):
        return None, 0, 'mounted'
    materialized = 0
    for entry in cache.git(root, 'ls-files', '--stage', '-z').split(b'\0'):
        if not entry:
            continue
        mode, rest = entry.split(b' ', 1)
        if mode == b'160000':
            return None, 0, 'submodule'
        name = os.fsdecode(rest.split(b'\t', 1)[1])
        info = (root / name).lstat()
        if info.st_nlink != 1:
            return None, 0, 'shared_file'
        materialized += info.st_blocks * 512
    return binding, materialized, ''


def compact(snapshots, apply=False, minimum=48 * 3600, now=None, proc=Path('/proc')):
    now = time.time() if now is None else now
    report = {'eligible': [], 'compacted': [], 'retained': {}, 'busy': 0, 'failed': 0,
              'reclaimed_materialized_bytes': 0, 'git_objects_preserved': True}
    if snapshots.is_symlink() or not snapshots.is_dir() or snapshots.resolve() != snapshots:
        return report
    deadline, changes = time.monotonic() + 20, 0
    for root in sorted(snapshots.iterdir()):
        if time.monotonic() >= deadline or changes >= 3:
            break
        if root.is_symlink() or not root.is_dir() or not re.fullmatch('[a-f0-9]{64}', root.name):
            continue
        try:
            metadata = root / '.git'
            if metadata.is_symlink() or not metadata.is_dir():
                continue
            gate = metadata / 'cuan-verification.lock'
            fd = os.open(gate, os.O_RDWR | os.O_CREAT | os.O_NOFOLLOW | os.O_NONBLOCK | os.O_CLOEXEC, 0o600)
            with os.fdopen(fd, 'r+b'):
                if os.geteuid() == 0:
                    identity = root.stat()
                    os.fchown(fd, identity.st_uid, identity.st_gid)
                info = os.fstat(fd)
                if not stat.S_ISREG(info.st_mode) or info.st_nlink != 1 or info.st_uid != root.stat().st_uid:
                    continue
                fcntl.flock(fd, fcntl.LOCK_EX | fcntl.LOCK_NB)
                if cache.referenced(root, proc):
                    report['busy'] += 1
                    continue
                binding, materialized, reason = candidate(root, now, minimum)
                if binding is None:
                    report['retained'][reason] = report['retained'].get(reason, 0) + 1
                    continue
                report['eligible'].append(root.name)
                if not apply:
                    continue
                if cache.referenced(root, proc):
                    report['busy'] += 1
                    continue
                # The marker is durable before changing materialization. The
                # prior compatibility release restores from local Git under
                # this same lease before native inspection can run again.
                marker = root / '.git/cuan-verification-cold.json'
                descriptor = os.open(marker, os.O_WRONLY | os.O_CREAT | os.O_EXCL | os.O_NOFOLLOW, 0o600)
                with os.fdopen(descriptor, 'w') as handle:
                    if os.geteuid() == 0:
                        identity = root.stat()
                        os.fchown(handle.fileno(), identity.st_uid, identity.st_gid)
                    json.dump(binding, handle)
                    handle.flush()
                    os.fsync(handle.fileno())
                directory = os.open(root / '.git', os.O_RDONLY | os.O_DIRECTORY | os.O_NOFOLLOW)
                try:
                    os.fsync(directory)
                finally:
                    os.close(directory)
                changes += 1
                cache.git(root, 'sparse-checkout', 'set', '--no-cone', '!*')
                report['compacted'].append(root.name)
                report['reclaimed_materialized_bytes'] += materialized
        except BlockingIOError:
            report['busy'] += 1
        except FileNotFoundError:
            report['retained']['unaccepted_or_missing'] = report['retained'].get('unaccepted_or_missing', 0) + 1
        except (OSError, ValueError, TypeError, KeyError, subprocess.SubprocessError):
            report['failed'] += 1
    return report


if __name__ == '__main__':
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--snapshots', type=Path, required=True)
    parser.add_argument('--apply', action='store_true')
    args = parser.parse_args()
    if not args.snapshots.is_absolute():
        parser.error('Use an absolute snapshot root')
    result = compact(args.snapshots, args.apply)
    print(json.dumps(result))
    raise SystemExit(1 if result['failed'] else 0)
