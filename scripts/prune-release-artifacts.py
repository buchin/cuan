#!/usr/bin/env python3
"""Prune old immutable release payloads; never touch application data/backups."""
import argparse
import fcntl
import json
import re
import shutil
import time
from pathlib import Path


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
    args = parser.parse_args()
    if args.keep < 2 or args.min_age_hours < 24:
        parser.error('Retain at least two releases and a minimum age of 24 hours')
    args.base = args.base.resolve()
    # Serialize with atomic activation/rollback so their targets cannot become
    # eligible between the reference checks and deletion.
    with (args.base / '.release.lock').open('a') as lock:
        fcntl.flock(lock, fcntl.LOCK_EX | fcntl.LOCK_NB)
        paths = candidates(args.base, args.keep, args.min_age_hours)
        if args.apply:
            for path in paths:
                if path in candidates(args.base, args.keep, args.min_age_hours):
                    shutil.rmtree(path)
        print(json.dumps({'applied': args.apply, 'eligible': [p.name for p in paths],
                          'keep': args.keep, 'applicationDataTouched': False, 'backupsTouched': False}))



if __name__ == '__main__':
    main()
