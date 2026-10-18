#!/usr/bin/env python3
"""Run S25 OAuth staging on port 8081 with an SSH tunnel to its isolated DB.

Use through `fural-agent sandboxes serve`; credentials stay outside the repo.
"""
import json
import os
from pathlib import Path
import secrets
import signal
import socket
import subprocess
import time
import urllib.parse


def main():
    directory = Path.home() / '.local/share/cuan-s25'
    connection = json.loads((directory / 'database.json').read_text())
    auth_path = directory / 'auth-secret'
    if not auth_path.exists():
        fd = os.open(auth_path, os.O_WRONLY | os.O_CREAT | os.O_EXCL, 0o600)
        with os.fdopen(fd, 'w') as output:
            output.write(secrets.token_hex(32))
    remote = json.loads((directory / 'ssh.json').read_text())
    host = remote['user'] + '@' + remote['host']
    tunnel = subprocess.Popen(['ssh', '-N', '-o', 'ExitOnForwardFailure=yes',
                               '-o', 'IdentitiesOnly=yes', '-o', 'StrictHostKeyChecking=yes',
                               '-o', 'UserKnownHostsFile=' + str(directory / 'known_hosts'),
                               '-i', str(directory / 'tunnel-key'),
                               '-o', 'ServerAliveInterval=15', '-o', 'ServerAliveCountMax=2',
                               '-L', '127.0.0.1:15432:127.0.0.1:5432', host])
    server = None

    def stop(*_):
        if server is not None:
            server.terminate()
        tunnel.terminate()
        raise SystemExit(0)

    signal.signal(signal.SIGTERM, stop)
    signal.signal(signal.SIGINT, stop)
    try:
        for _ in range(50):
            if tunnel.poll() is not None:
                raise RuntimeError('Staging database tunnel failed')
            try:
                with socket.create_connection(('127.0.0.1', 15432), timeout=.2):
                    break
            except OSError:
                time.sleep(.2)
        else:
            raise RuntimeError('Staging database tunnel not ready')
        # Do not expose organization credentials to provider subprocesses.
        environment = {key: value for key, value in os.environ.items()
                       if key in ('HOME', 'PATH', 'LANG', 'TERM', 'SSL_CERT_FILE', 'LD_LIBRARY_PATH')}
        environment.update(PORT='8081', CUAN_ENV='staging', CUAN_AUTO_MIGRATE='false',
                           CUAN_AUTH_SECRET=auth_path.read_text(), CUAN_AUTO_SYNC_README='false',
                           CUAN_STATIC_DIR=str(Path.cwd() / 'web/dist'),
                           CUAN_WORKSPACES_DIR=str(directory / 'workspaces'),
                           DATABASE_URL='postgresql://cuan_s25:' + urllib.parse.quote(connection['password'])
                           + '@127.0.0.1:15432/cuan_s25?sslmode=disable')
        server = subprocess.Popen([str(directory / 'cuan-server')], env=environment)
        while server.poll() is None:
            if tunnel.poll() is not None:
                raise RuntimeError('Staging database tunnel disconnected')
            time.sleep(1)
        raise SystemExit(server.returncode)
    finally:
        if server is not None and server.poll() is None:
            server.terminate()
            server.wait(timeout=10)
        tunnel.terminate()
        tunnel.wait(timeout=10)


if __name__ == '__main__':
    main()
