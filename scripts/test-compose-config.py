#!/usr/bin/env python3
"""Check real Compose rendering without a daemon or printing private config."""
import argparse
import json
import os
from pathlib import Path
import shutil
import subprocess

parser = argparse.ArgumentParser(description=__doc__)
parser.add_argument('--compose', help='Standalone Docker Compose executable; otherwise detect Compose')
args = parser.parse_args()
root = Path(__file__).resolve().parents[1]
if args.compose:
    command = [str(Path(args.compose).resolve())]
elif shutil.which('docker-compose'):
    command = ['docker-compose']
else:
    command = ['docker', 'compose']
# Never inherit deployment credentials into test rendering.
env = {'HOME': str(Path.home()), 'PATH': os.environ.get('PATH', '/usr/bin:/bin')}
fixture = {'POSTGRES_PASSWORD': 'fixture-only-password', 'CUAN_AUTH_SECRET': 'fixture-only-auth-secret-32-characters', 'CUAN_DOMAIN': 'cuan.example.invalid', 'CUAN_STAGING_DOMAIN': 'staging.example.invalid'}
results = []
volumes = {}
for kind, filename, default_port in [('production', 'docker-compose.yml', 8080), ('staging', 'docker-compose.staging.yml', 8081)]:
    base = [*command, '--env-file', '/dev/null', '-f', str(root / filename), '--profile', 'proxy', 'config', '--format', 'json']
    # Manual startup must not silently use a public shared password/auth key.
    for missing in fixture:
        if missing not in ('POSTGRES_PASSWORD', 'CUAN_AUTH_SECRET'):
            continue
        bad = subprocess.run(base, env={**env, **{k:v for k,v in fixture.items() if k != missing}}, capture_output=True, text=True, timeout=10)
        assert bad.returncode != 0 and missing in bad.stderr, f'{kind}: missing {missing} was accepted'
    for port in [None, 9081]:
        values = {**env, **fixture, **({'PORT': str(port)} if port else {})}
        run = subprocess.run(base, env=values, capture_output=True, text=True, timeout=10)
        assert run.returncode == 0, f'{kind}: Compose rendering failed'
        config = json.loads(run.stdout)
        app, database, proxy = (config['services'][key] for key in ['app', 'db', 'caddy'])
        assert '@db:5432/' in app['environment']['DATABASE_URL']
        assert str(app['ports'][0]['published']) == str(port or default_port)
        assert app['ports'][0]['target'] == 8080 and str(app['environment']['PORT']) == '8080'
        mounts = {mount['target']: mount['source'] for mount in app['volumes']}
        assert '/app/data' in mounts and '/home/cuan' in mounts
        assert all(name in config['volumes'] for name in mounts.values())
        assert not database.get('ports'), 'database must stay internal'
        assert proxy['environment']['CUAN_DOMAIN'] == fixture['CUAN_DOMAIN' if kind == 'production' else 'CUAN_STAGING_DOMAIN']
        volumes[kind] = set(mounts.values())
        results.append({'environment': kind, 'port': port or default_port, 'rendered': True, 'persistentAppAndHome': True, 'databaseAndProxyValidated': True})
assert not volumes['production'].intersection(volumes['staging'])
print(json.dumps({'checks': results, 'missingSecretsRejected': True, 'isolatedStorage': True, 'daemonRequired': False}, indent=2))
