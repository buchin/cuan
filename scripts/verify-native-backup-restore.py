#!/usr/bin/env python3
"""Restore one native dump into a disposable private PostgreSQL instance."""
import argparse
import json
import os
from pathlib import Path
import pwd
import re
import shutil
import stat
import subprocess
import tempfile


def verify(backup, user='ubuntu', binaries=Path('/usr/lib/postgresql/16/bin')):
    info = backup.lstat()
    if not stat.S_ISREG(info.st_mode) or info.st_nlink != 1 or info.st_mode & 0o077:
        raise ValueError('Use a private regular backup')
    account = pwd.getpwnam(user)
    if account.pw_uid == 0 or os.geteuid() not in (0, account.pw_uid):
        raise ValueError('Restore probe requires an unprivileged service user')
    privileges = dict(user=account.pw_uid, group=account.pw_gid, extra_groups=[]) if os.geteuid() == 0 else {}
    root = Path(tempfile.mkdtemp(prefix='cuan-restore-probe-'))
    started = False
    try:
        private_dump = root / 'backup.dump'
        shutil.copyfile(backup, private_dump)
        private_dump.chmod(0o600)
        if os.geteuid() == 0:
            os.chown(root, account.pw_uid, account.pw_gid)
            os.chown(private_dump, account.pw_uid, account.pw_gid)
        # An isolated environment and unique private socket make accidental
        # production connections impossible. No app, listener or email job starts.
        env = dict(PATH=os.defpath, HOME=str(root), PGHOST=str(root),
                   PGPORT='55439', PGUSER='cuan_restore_probe', PGDATABASE='postgres')
        def run(name, *args, timeout=120):
            return subprocess.run([str(binaries / name), *args], cwd=root, env=env,
                                  stdin=subprocess.DEVNULL, capture_output=True,
                                  timeout=timeout, check=True, **privileges).stdout
        data = root / 'data'
        run('initdb', '-D', str(data), '-U', 'cuan_restore_probe', '-A', 'trust', '--no-locale', '-E', 'UTF8')
        config = data / 'postgresql.conf'
        with config.open('a') as output:
            output.write(f"\nlisten_addresses = ''\nunix_socket_directories = '{root}'\nport = 55439\nshared_buffers = '16MB'\nmax_connections = 10\n")
        run('pg_ctl', '-D', str(data), '-l', str(root / 'server.log'), '-w', 'start', timeout=30)
        started = True
        run('pg_restore', '--exit-on-error', '--no-owner', '--no-privileges', '-d', 'postgres', str(private_dump))
        tables = run('psql', '-XAt', '-v', 'ON_ERROR_STOP=1', '-c',
                     "SELECT count(*) FROM information_schema.tables WHERE table_schema='public'").decode().strip()
        if not re.fullmatch('[1-9][0-9]*', tables):
            raise ValueError('Restored database has no public tables')
        return dict(restored=True, public_tables=int(tables), production_database_touched=False)
    finally:
        if started or (root / 'data/postmaster.pid').exists():
            # Stop only the instance rooted in our newly created temporary tree.
            subprocess.run([str(binaries / 'pg_ctl'), '-D', str(root / 'data'), '-m', 'immediate', '-w', 'stop'],
                           cwd='/', env={'PATH': os.defpath}, stdin=subprocess.DEVNULL,
                           stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL, timeout=30,
                           check=True, **privileges)
        shutil.rmtree(root)


if __name__ == '__main__':
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--backup', required=True, type=Path)
    parser.add_argument('--user', default='ubuntu')
    parser.add_argument('--postgres-bin', type=Path, default=Path('/usr/lib/postgresql/16/bin'))
    args = parser.parse_args()
    try:
        result = verify(args.backup, args.user, args.postgres_bin)
        result['temporary_instance_removed'] = True
        print(json.dumps(result))
    except (OSError, ValueError, KeyError, subprocess.SubprocessError) as error:
        parser.exit(1, f'Isolated restore probe failed ({type(error).__name__}); no production connection used.\n')
