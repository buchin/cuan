#!/usr/bin/env python3
"""Activate or roll back a versioned native Cuan release (run as root on VPS)."""
import argparse
import fcntl
import json
import os
from pathlib import Path
import re
import shutil
import subprocess
import time
import urllib.request
import urllib.parse

BASE = Path('/opt/cuan')


def switch(link, target):
    temporary = link.with_name(link.name + '.next')
    temporary.unlink(missing_ok=True)
    temporary.symlink_to(target)
    os.replace(temporary, link)


def healthy():
    try:
        for endpoint in ('healthz', 'readyz'):
            with urllib.request.urlopen('http://127.0.0.1:8080/' + endpoint, timeout=3) as response:
                body = json.load(response)
                if endpoint == 'readyz' and body.get('migrations_count') != 19:
                    return False
        with urllib.request.urlopen('http://127.0.0.1:8080/tasks', timeout=3) as response:
            if response.read() != (BASE / 'current/web/dist/index.html').read_bytes():
                return False
        return True
    except Exception:
        return False


def restart_and_check():
    subprocess.run(['systemctl', 'restart', 'cuan'], check=True)
    for _ in range(20):
        if healthy():
            return True
        time.sleep(1)
    return False


def backup_database(release):
    backups=BASE/'backups'
    backups.mkdir(exist_ok=True,mode=0o700)
    target=backups/(release+'.dump')
    if target.exists(): return
    pid=subprocess.check_output(['systemctl','show','cuan','-p','MainPID','--value'],text=True).strip()
    entries=Path('/proc/'+pid+'/environ').read_bytes().split(b'\0')
    config=dict(entry.decode().split('=',1) for entry in entries if b'=' in entry)
    url=urllib.parse.urlparse(config['DATABASE_URL'])
    env=os.environ.copy()
    env.update(PGHOST=url.hostname or 'localhost',PGPORT=str(url.port or 5432),PGUSER=urllib.parse.unquote(url.username or ''),PGPASSWORD=urllib.parse.unquote(url.password or ''),PGDATABASE=url.path.lstrip('/'))
    staged=target.with_suffix('.pending')
    descriptor=os.open(staged,os.O_WRONLY|os.O_CREAT|os.O_TRUNC,0o600)
    with os.fdopen(descriptor,'wb') as output:
        subprocess.run(['pg_dump','--format=custom','--no-owner','--no-privileges'],env=env,stdout=output,check=True)
    os.replace(staged,target)
    print('Private pre-release database backup created; no migrations will run.')


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    group = parser.add_mutually_exclusive_group(required=True)
    group.add_argument('--release')
    group.add_argument('--rollback', action='store_true')
    args = parser.parse_args()
    if os.geteuid() != 0:
        parser.error('Run with sudo on the deployment host')
    with (BASE / '.release.lock').open('w') as lock:
        fcntl.flock(lock, fcntl.LOCK_EX | fcntl.LOCK_NB)
        if args.release:
            if not re.fullmatch(r'[0-9a-f]{40}',args.release): parser.error('Invalid release SHA')
            backup_database(args.release)
        current = BASE / 'current'
        previous = BASE / 'previous'
        if not current.exists():
            baseline = BASE / 'releases/legacy-before-s25'
            baseline.mkdir(parents=True, exist_ok=True)
            for name in ('bin', 'web', 'migrations', 'scripts'):
                if (BASE / name).exists() and not (baseline / name).exists():
                    shutil.copytree(BASE / name, baseline / name)
            if (BASE / 'README.md').exists():
                shutil.copy2(BASE / 'README.md', baseline / 'README.md')
            switch(current, baseline)
        old = current.resolve()
        if args.rollback:
            if not previous.exists():
                parser.error('No previous release is retained')
            target = previous.resolve()
        else:
            if not re.fullmatch(r'[0-9a-f]{40}', args.release):
                parser.error('Release must be a complete Git SHA')
            target = BASE / 'releases' / args.release
            manifest = json.loads((target / 'release.json').read_text())
            if manifest.get('sha') != args.release:
                parser.error('Release manifest mismatch')
            import hashlib
            for name, expected in manifest['sha256'].items():
                path = (target / name).resolve()
                if not path.is_relative_to(target.resolve()) or hashlib.sha256(path.read_bytes()).hexdigest() != expected:
                    parser.error('Release payload checksum mismatch')
        if not (target / 'bin/cuan-server').is_file() or not (target / 'web/dist/index.html').is_file():
            parser.error('Incomplete release')
        for release in (old, target):
            for name in ('data', 'sponsors'):
                if not (release / name).exists():
                    (release / name).symlink_to(BASE / name)
        unit_dir = Path('/etc/systemd/system/cuan.service.d')
        unit_dir.mkdir(exist_ok=True)
        # systemd EnvironmentFile overrides Environment, even in drop-ins.
        # Load these non-secret release settings after the retained private .env.
        (BASE / 'release.env').write_text(
            'PATH=/home/ubuntu/.local/bin:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin\n'
            'CUAN_STATIC_DIR=/opt/cuan/current/web/dist\nCUAN_AUTO_MIGRATE=false\n')
        (unit_dir / 'release.conf').write_text(
            '[Service]\nEnvironmentFile=/opt/cuan/release.env\nWorkingDirectory=/opt/cuan/current\nExecStart=\n'
            'ExecStart=/opt/cuan/current/bin/cuan-server\n'
            'Environment=PATH=/home/ubuntu/.local/bin:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin\n'
            'Environment=CUAN_STATIC_DIR=/opt/cuan/current/web/dist\n'
            'Environment=CUAN_AUTO_MIGRATE=false\n')
        subprocess.run(['systemctl', 'daemon-reload'], check=True)
        switch(current, target)
        if not restart_and_check():
            switch(current, old)
            recovered = restart_and_check()
            raise SystemExit('Release health failed; rollback ' + ('healthy' if recovered else 'requires intervention'))
        switch(previous, old)
        print(json.dumps({'release': target.name, 'previous': old.name, 'health': 'healthy', 'migrations': 19}))


if __name__ == '__main__':
    main()
