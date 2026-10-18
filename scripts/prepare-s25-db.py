#!/usr/bin/env python3
"""Create an isolated S25 staging database from the running Cuan database.

Run on the VPS as the service user. Existing staging databases are reused.
Writes connection details privately; never prints credentials.
"""
import json
import os
from pathlib import Path
import secrets
import subprocess
import urllib.parse


def postgres(*args, input=None):
    return subprocess.check_output(['sudo', '-n', '-u', 'postgres', *args], input=input)


def main():
    directory = Path.home() / '.local/share/cuan-s25'
    directory.mkdir(parents=True, exist_ok=True, mode=0o700)
    receipt = directory / 'database.json'
    if receipt.exists():
        print('Staging database already prepared; private connection receipt retained.')
        return
    pid = subprocess.check_output(['systemctl', 'show', 'cuan', '-p', 'MainPID', '--value'], text=True).strip()
    entries = Path(f'/proc/{pid}/environ').read_bytes().split(b'\0')
    environment = dict(entry.decode().split('=', 1) for entry in entries if b'=' in entry)
    source = urllib.parse.urlparse(environment['DATABASE_URL'])
    password = secrets.token_hex(32)
    # Fixed identifiers and a hexadecimal password; no interpolated external SQL.
    database = 'cuan_s25'
    postgres('psql', '-X', '-v', 'ON_ERROR_STOP=1', '-d', 'postgres', input=(
        f"CREATE ROLE cuan_s25 LOGIN PASSWORD '{password}';\n").encode())
    postgres('createdb', '--owner=cuan_s25', database)
    dump = directory / 'database.dump'
    env = os.environ.copy()
    env.update(PGHOST=source.hostname or 'localhost', PGPORT=str(source.port or 5432),
               PGUSER=urllib.parse.unquote(source.username or ''),
               PGPASSWORD=urllib.parse.unquote(source.password or ''),
               PGDATABASE=source.path.lstrip('/'))
    try:
        with dump.open('wb') as output:
            dump.chmod(0o600)
            subprocess.run(['pg_dump', '--format=custom', '--no-owner', '--no-privileges'],
                           env=env, stdout=output, check=True)
        # Stream the private dump on stdin; postgres does not need access to the user's home.
        with dump.open('rb') as source_dump:
            subprocess.run(['sudo', '-n', '-u', 'postgres', 'pg_restore', '--exit-on-error',
                            '--no-owner', '--no-privileges', '--role=cuan_s25', '-d', database],
                           stdin=source_dump, check=True, stdout=subprocess.DEVNULL)
    finally:
        dump.unlink(missing_ok=True)
    descriptor = os.open(receipt, os.O_WRONLY | os.O_CREAT | os.O_EXCL, 0o600)
    with os.fdopen(descriptor, 'w') as output:
        json.dump({'database': database, 'user': 'cuan_s25', 'password': password}, output)
    print('Isolated staging database prepared. Production data unchanged; credentials stored privately.')


if __name__ == '__main__':
    main()
