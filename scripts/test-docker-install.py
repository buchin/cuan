#!/usr/bin/env python3
"""Verify fresh Docker installs on an explicitly labelled, empty disposable daemon.

Never point this at a normal application daemon. The test builds Cuan, creates
fixture accounts and volumes, recreates containers, verifies storage isolation,
and deletes its test containers/volumes. No AI provider is called.
"""
import argparse
import json
import os
from pathlib import Path
import secrets
import socket
import subprocess
import tempfile
import time
import urllib.error
import urllib.request

parser = argparse.ArgumentParser(description=__doc__)
parser.add_argument('--docker', default='docker', help='Docker CLI executable')
parser.add_argument('--compose', required=True, help='Standalone Docker Compose executable')
args = parser.parse_args()
root = Path(__file__).resolve().parents[1]

def docker(*parts, timeout=30, capture=True):
    run = subprocess.run([args.docker, *parts], capture_output=capture, text=True, timeout=timeout)
    if run.returncode:
        raise RuntimeError('Docker operation failed: ' + ' '.join(parts[:3]))
    return run.stdout.strip() if capture else ''

info = json.loads(docker('info', '--format', '{{json .}}'))
if 'cuan.install-audit=true' not in (info.get('Labels') or []):
    parser.error('requires a dedicated daemon started with --label cuan.install-audit=true')
if docker('ps', '-aq') or docker('volume', 'ls', '-q'):
    parser.error('test daemon must have no existing containers or volumes')
image = 'cuan:install-audit'
# Explicit bounds support small self-hosted build machines.
docker('build', '--network=host', '--build-arg', 'GO_BUILD_PARALLELISM=1', '--build-arg', 'GO_BUILD_MEMORY=256MiB', '--build-arg', 'NODE_BUILD_MEMORY=384', '-t', image, str(root), timeout=900, capture=False)

def free_port():
    with socket.socket() as sock:
        sock.bind(('127.0.0.1', 0))
        return sock.getsockname()[1]

def request(port, path, body=None, token=''):
    headers = {'Content-Type': 'application/json'}
    if token:
        headers['Authorization'] = 'Bearer ' + token
    req = urllib.request.Request(f'http://127.0.0.1:{port}'+path, headers=headers, data=None if body is None else json.dumps(body).encode())
    with urllib.request.urlopen(req, timeout=3) as response:
        return json.load(response)

with tempfile.TemporaryDirectory(prefix='cuan-container-fixture-') as temporary:
    temp = Path(temporary)
    environments = {}
    checks = []
    def compose(kind, *parts, timeout=45):
        spec = environments[kind]
        filename = 'docker-compose.yml' if kind == 'production' else 'docker-compose.staging.yml'
        # Bind test ports to loopback and cap runtime resource use.
        run = subprocess.run([args.compose, '-p', 'cuan-audit-'+kind, '-f', str(root/filename), '-f', str(spec['override']), '--env-file', str(spec['env']), *parts], capture_output=True, text=True, timeout=timeout)
        if run.returncode:
            raise RuntimeError(kind + ' Compose operation failed: ' + ' '.join(parts[:2]))
        return run.stdout.strip()

    for kind in ['production', 'staging']:
        port = free_port()
        environment = temp / (kind+'.env')
        environment.write_text(f'CUAN_IMAGE={image}\nPORT={port}\nPOSTGRES_PASSWORD={secrets.token_hex(24)}\nCUAN_AUTH_SECRET={secrets.token_hex(24)}\n')
        environment.chmod(0o600)
        override = temp / (kind+'.yaml')
        # !override is needed because Compose otherwise merges published ports.
        override.write_text('services:\n  app:\n    ports: !override\n      - "127.0.0.1:'+str(port)+':8080"\n    mem_limit: 192m\n    cpus: 0.5\n  db:\n    mem_limit: 192m\n    cpus: 0.5\n')
        environments[kind] = {'port':port, 'env':environment, 'override':override}

    def wait_ready(kind):
        port = environments[kind]['port']
        for _ in range(60):
            try:
                data = request(port, '/readyz')
                if data.get('status') == 'ready' and data.get('migrations_count') == 25:
                    return
            except (OSError, ValueError, urllib.error.HTTPError):
                pass
            time.sleep(.5)
        raise RuntimeError(kind+' application/database readiness failed')

    try:
        for kind in ['production', 'staging']:
            compose(kind, 'up', '-d', '--no-build', timeout=120)
            wait_ready(kind)
            port = environments[kind]['port']
            assert request(port, '/api/v1/auth/status')['setup_required']
            container = compose(kind, 'ps', '-q', 'app')
            assert docker('exec', container, 'id', '-u') == '1000'
            assert docker('exec', container, 'sh', '-c', 'printf "%s" "$HOME"') == '/home/cuan'
            assert docker('exec', container, 'sh', '-c', 'test ! -e /app/data/install-audit && test ! -e /home/cuan/install-audit; echo $?') == '0'
            session = request(port, '/api/v1/auth/setup', {'email':kind+'@example.invalid', 'password':'fixture-only-password-123'})
            token = session['token']
            project = request(port, '/api/v1/projects/default', token=token)
            assert project['name'] == 'Default' and not project['repo_url']
            docker('exec', container, 'sh', '-c', 'printf '+kind+' > /app/data/install-audit; printf '+kind+' > /home/cuan/install-audit')
            compose(kind, 'up', '-d', '--no-build', '--force-recreate', '--no-deps', 'app')
            wait_ready(kind)
            # Recreate both app and PostgreSQL without deleting volumes.
            compose(kind, 'down', timeout=45)
            compose(kind, 'up', '-d', '--no-build', timeout=120)
            wait_ready(kind)
            container = compose(kind, 'ps', '-q', 'app')
            assert docker('exec', container, 'cat', '/app/data/install-audit') == kind
            assert docker('exec', container, 'cat', '/home/cuan/install-audit') == kind
            assert not request(port, '/api/v1/auth/status')['setup_required']
            assert request(port, '/api/v1/auth/me', token=token)['owner']['email'] == kind+'@example.invalid'
            environments[kind]['token'] = token
            checks.append({'environment':kind,'nonRootUser':True,'writablePersistentHomeAndData':True,'neutralDefaultProject':True,'ownerAndSessionSurviveContainerRecreation':True})
            compose(kind, 'down')
        # Staging startup must not have overwritten retained production data.
        compose('production', 'up', '-d', '--no-build')
        wait_ready('production')
        container = compose('production', 'ps', '-q', 'app')
        assert docker('exec', container, 'cat', '/app/data/install-audit') == 'production'
        assert request(environments['production']['port'], '/api/v1/auth/me', token=environments['production']['token'])['owner']['email'] == 'production@example.invalid'
        print(json.dumps({'checks':checks,'productionStagingStorageIsolated':True,'providerCalls':0,'testImage':image}), flush=True)
    finally:
        for kind in environments:
            compose(kind, 'down', '-v', '--remove-orphans', timeout=60)
