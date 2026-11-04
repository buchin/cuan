#!/usr/bin/env python3
"""Prune old immutable release payloads; never touch application data/backups."""
import argparse
import fcntl
import hashlib
import json
import re
import shutil
import time
from pathlib import Path


def staging_candidates(base, staging, keep=5, now=None, only_names=None):
    """Only redundant upload copies with a byte-verified installed counterpart."""
    now = time.time() if now is None else now
    protected = {(base / name).resolve().name for name in ('current', 'previous')}
    paths = sorted((p for p in staging.iterdir() if p.is_dir() and not p.is_symlink()
                    and re.fullmatch('[0-9a-f]{40}', p.name)),
                   key=lambda p: p.stat().st_mtime, reverse=True)
    protected.update(p.name for p in paths[:keep])
    result = []
    for path in paths:
        if only_names is not None and path.name not in only_names:
            continue
        if path.name in protected or now - path.stat().st_mtime < 3600:
            continue
        installed = base / 'releases' / path.name
        if installed.is_symlink() or not installed.is_dir():
            continue
        try:
            raw = (path / 'release.json').read_bytes()
            manifest = json.loads(raw)
            checks = manifest.get('sha256')
            if manifest.get('sha') != path.name or not isinstance(checks, dict) or not checks or raw != (installed / 'release.json').read_bytes():
                continue
            entries = list(path.rglob('*'))
            if any(p.is_symlink() for p in entries):
                continue
            if not {str(p.relative_to(path)) for p in entries if p.is_file()} <= set(checks) | {'release.json'}:
                continue  # Unknown files might be saved work, not payload.
            for name, digest in checks.items():
                relative = Path(name)
                if relative.is_absolute() or '..' in relative.parts:
                    raise ValueError('Unsafe manifest path')
                for root in (path, installed):
                    file = root / relative
                    if root == path and not file.exists():
                        continue  # Partial upload is safe only if installed copy is complete.
                    if not file.resolve().is_relative_to(root.resolve()) or file.is_symlink():
                        raise ValueError('Unsafe payload file')
                    if hashlib.sha256(file.read_bytes()).hexdigest() != digest:
                        raise ValueError('Payload differs')
            result.append(path)
        except (OSError, ValueError, TypeError):
            continue
    return result


def candidates(base, keep=5, min_age_hours=24, now=None):
    now = time.time() if now is None else now
    releases = base / 'releases'
    protected = set()
    for name in ('current', 'previous'):
        ref = base / name
        if not ref.is_symlink() or not ref.resolve().is_dir():
            raise RuntimeError('Current and previous release links must both be valid')
        protected.add(ref.resolve())
    for process in Path('/proc').glob('[0-9]*'):
        for name in ('exe', 'cwd'):
            try:
                target = (process / name).resolve(strict=True)
                if target.is_relative_to(releases):
                    protected.add(releases / target.relative_to(releases).parts[0])
            except (OSError, RuntimeError):
                pass
    valid = []
    for path in releases.iterdir():
        if path.is_symlink() or not path.is_dir() or not re.fullmatch('[0-9a-f]{40}', path.name):
            continue
        try:
            manifest = json.loads((path / 'release.json').read_text())
            if manifest.get('sha') != path.name:
                continue
        except (OSError, ValueError):
            continue
        valid.append(path)
    valid.sort(key=lambda p: p.stat().st_mtime, reverse=True)
    protected.update(valid[:keep])
    return [p for p in valid if p.resolve() not in protected and now - p.stat().st_mtime >= min_age_hours * 3600]


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--base', type=Path, default=Path('/opt/cuan'))
    parser.add_argument('--keep', type=int, default=5)
    parser.add_argument('--min-age-hours', type=float, default=24)
    parser.add_argument('--apply', action='store_true', help='Delete eligible payloads; default is a dry run')
    parser.add_argument('--staging', type=Path, help='Also prune verified redundant upload copies older than one hour')
    args = parser.parse_args()
    if args.keep < 2 or args.min_age_hours < 24:
        parser.error('Retain at least two releases and a minimum age of 24 hours')
    args.base = args.base.resolve()
    if args.staging:
        args.staging = args.staging.resolve()
        if args.staging.is_relative_to(args.base) or args.base.is_relative_to(args.staging):
            parser.error('Staging and installation directories must not overlap')
    # Serialize with atomic activation/rollback so their targets cannot become
    # eligible between the reference checks and deletion.
    with (args.base / '.release.lock').open('a') as lock:
        fcntl.flock(lock, fcntl.LOCK_EX | fcntl.LOCK_NB)
        paths = candidates(args.base, args.keep, args.min_age_hours)
        staged = staging_candidates(args.base, args.staging.resolve(), args.keep) if args.staging else []
        if args.apply:
            # Check uploaded duplicates while their installed counterparts are
            # still present; the same activation lock protects both passes.
            for path in staged:
                if path in staging_candidates(args.base, args.staging.resolve(), args.keep, only_names={path.name}):
                    shutil.rmtree(path)
            for path in paths:
                if path in candidates(args.base, args.keep, args.min_age_hours):
                    shutil.rmtree(path)
        print(json.dumps({'applied': args.apply, 'eligible': [p.name for p in paths],
                          'stagingEligible': [p.name for p in staged],
                          'keep': args.keep, 'applicationDataTouched': False, 'backupsTouched': False}))



if __name__ == '__main__':
    main()
