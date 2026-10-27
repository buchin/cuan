#!/usr/bin/env python3
"""Install a native Cuan release without overwriting existing settings or data."""
import argparse
import fcntl
import json
import os
from pathlib import Path
import secrets
import shutil
import subprocess
import sys
import tempfile
import uuid

LAUNCHER = r'''#!/usr/bin/env python3
import argparse,json,os,pathlib,subprocess,sys,urllib.request
root=pathlib.Path(__file__).resolve().parent
parser=argparse.ArgumentParser(description='Run your installed Cuan application.')
parser.add_argument('command',choices=['start','migrate','check'],help='start in foreground, explicitly migrate, or check web/database readiness')
args=parser.parse_args()
try:
 config=json.loads((root/'config.json').read_text())
 env={**os.environ,**config}
 if args.command=='check':
  with urllib.request.urlopen('http://127.0.0.1:'+config['PORT']+'/readyz',timeout=5) as reply:
   data=json.load(reply)
  if data.get('status')!='ready': raise RuntimeError('Application is not ready')
  print(json.dumps({'applicationReady':True,'databaseReady':data.get('database')=='connected','aiExecutionVerified':False}))
 else:
  binary='cuan-server' if args.command=='start' else 'cuan-migrate'
  command=[str(root/'current'/binary)]
  os.chdir(root/'data')
  os.execve(command[0],command,env)
except Exception:
 print('Cuan could not '+args.command+'. Check the private configuration, service log and database availability; no credentials are printed.',file=sys.stderr)
 sys.exit(1)
'''


def atomic_json(path, data):
    descriptor, temporary = tempfile.mkstemp(prefix='.config-', dir=path.parent)
    try:
        with os.fdopen(descriptor, 'w') as output:
            json.dump(data, output, indent=2)
            output.write('\n')
            output.flush()
            os.fsync(output.fileno())
        os.replace(temporary, path)
    finally:
        if os.path.exists(temporary):
            os.unlink(temporary)


def activate(root, release):
    link = root / ('.current-' + uuid.uuid4().hex)
    try:
        link.symlink_to(release.relative_to(root), target_is_directory=True)
        os.replace(link, root / 'current')
    finally:
        link.unlink(missing_ok=True)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--directory', type=Path, default=Path.home()/'.local/share/cuan-app', help='Persistent private installation directory')
    parser.add_argument('--port', type=int, help='HTTP port for a new installation (default 8080); existing configuration is preserved')
    parser.add_argument('--database-env', default='DATABASE_URL', help='Environment variable holding PostgreSQL URL; value is never printed')
    parser.add_argument('--binary-dir', type=Path, help='Use prebuilt cuan-server and cuan-migrate instead of building')
    parser.add_argument('--static-dir', type=Path, help='Built web assets; required with --binary-dir')
    parser.add_argument('--dry-run', action='store_true', help='Show plan without creating files or running builds/migrations')
    parser.add_argument('--migrate', action='store_true', help='Explicitly apply database migrations before activating the release')
    parser.add_argument('--start', action='store_true', help='Start in foreground after installation; does not implicitly run migrations')
    args = parser.parse_args()
    if sys.platform != 'linux' or os.geteuid() == 0:
        parser.error('Run as a non-root Linux service user with a persistent HOME')
    if args.port is not None and not 1 <= args.port <= 65535:
        parser.error('--port must be between 1 and 65535')
    if bool(args.binary_dir) != bool(args.static_dir):
        parser.error('--binary-dir and --static-dir must be supplied together')
    root = args.directory.expanduser().absolute()
    source = Path(__file__).resolve().parent.parent
    if root == source or source in root.parents:
        parser.error('Choose an installation directory outside the source checkout')
    if args.dry_run:
        print(json.dumps({'directory':str(root),'build':not bool(args.binary_dir),'preserveExistingConfiguration':True,'migrate':args.migrate,'start':args.start,'aiExecutionVerified':False}))
        return
    root.mkdir(parents=True, exist_ok=True, mode=0o700)
    if root.is_symlink() or root.stat().st_uid != os.getuid():
        parser.error('Installation directory must be owned by this user and not a symbolic link')
    root.chmod(0o700)
    with (root/'.install.lock').open('a') as lock:
        try:
            fcntl.flock(lock, fcntl.LOCK_EX | fcntl.LOCK_NB)
        except BlockingIOError:
            parser.error('Another installer is active; wait for it to finish')
        config_path = root/'config.json'
        if config_path.is_symlink():
            parser.error('config.json must be a private regular file, not a symbolic link')
        if config_path.exists():
            config = json.loads(config_path.read_text())
            if not isinstance(config, dict) or not all(isinstance(config.get(key),str) and config[key] for key in ['DATABASE_URL','CUAN_AUTH_SECRET','PORT','HOME']):
                parser.error('Existing config.json is invalid; preserved without replacement')
            if args.port is not None and str(args.port) != config['PORT']:
                parser.error('Existing port is preserved; edit private config.json explicitly to change it')
            if len(config['CUAN_AUTH_SECRET']) < 32 or not config['PORT'].isdigit() or not 1 <= int(config['PORT']) <= 65535:
                parser.error('Existing secret or port is invalid; config.json was preserved')
            config_path.chmod(0o600)
        else:
            database = os.environ.get(args.database_env, '')
            if not database.startswith(('postgres://','postgresql://')):
                parser.error('Set the PostgreSQL URL in the environment variable named by --database-env')
            config = {'DATABASE_URL':database,'CUAN_AUTH_SECRET':secrets.token_hex(32),'PORT':str(args.port or 8080),'HOME':str(Path.home()),
                      'CUAN_AUTO_MIGRATE':'false','CUAN_AUTO_SYNC_README':'false','CUAN_WORKSPACES_DIR':str(root/'data/workspaces'),
                      'CUAN_STATIC_DIR':str(root/'current/static'),'PATH':str(Path.home()/'.local/bin')+':'+os.environ.get('PATH','/usr/bin:/bin')}
        (root/'data/workspaces').mkdir(parents=True, exist_ok=True, mode=0o700)
        (root/'releases').mkdir(exist_ok=True, mode=0o700)
        stage = Path(tempfile.mkdtemp(prefix='.building-', dir=root/'releases'))
        release = root/'releases'/uuid.uuid4().hex
        try:
            if args.binary_dir:
                for name in ['cuan-server','cuan-migrate']:
                    binary = args.binary_dir/name
                    if not binary.is_file() or not os.access(binary, os.X_OK):
                        raise RuntimeError('Prebuilt server/migrator missing or not executable')
                    shutil.copy2(binary, stage/name)
                if not (args.static_dir/'index.html').is_file():
                    raise RuntimeError('Built static directory must contain index.html')
                shutil.copytree(args.static_dir,stage/'static')
            else:
                for tool in ['go','npm']:
                    if not shutil.which(tool):
                        raise RuntimeError('Missing '+tool+'; install the version documented in docs/guides/onboarding.md')
                build_env={**os.environ,'GOMAXPROCS':'2','GOFLAGS':'-p=2'}
                for name,package in [('cuan-server','./cmd/server'),('cuan-migrate','./cmd/migrate')]:
                    subprocess.run(['go','build','-o',str(stage/name),package],cwd=source,env=build_env,check=True,timeout=300)
                web=stage/'web-source'
                shutil.copytree(source/'web',web,ignore=shutil.ignore_patterns('node_modules','dist','*.tsbuildinfo'))
                subprocess.run(['npm','ci','--no-audit','--no-fund'],cwd=web,check=True,timeout=240)
                subprocess.run(['npm','run','build'],cwd=web,check=True,timeout=180)
                shutil.move(web/'dist',stage/'static')
                shutil.rmtree(web)
            # Migration output can include connection details; keep it private.
            if args.migrate:
                with os.fdopen(os.open(root/'migration.log',os.O_WRONLY|os.O_CREAT|os.O_TRUNC,0o600),'w') as log:
                    os.chmod(root/'migration.log',0o600)
                    subprocess.run([str(stage/'cuan-migrate')],env={**os.environ,**config},cwd=root/'data',stdout=log,stderr=log,check=True,timeout=60)
            stage.rename(release)
            if not config_path.exists():
                atomic_json(config_path,config)
            launcher=root/'cuan'
            descriptor,temp=tempfile.mkstemp(prefix='.launcher-',dir=root)
            with os.fdopen(descriptor,'w') as output:
                output.write(LAUNCHER)
            os.chmod(temp,0o700)
            os.replace(temp,launcher)
            activate(root,release)
            print(json.dumps({'installed':True,'directory':str(root),'configurationPreserved':True,'migrationsApplied':args.migrate,'aiExecutionVerified':False,
                              'nextStep':'Run '+str(launcher)+' start, then create your owner account and connect AI in Studio. For readiness: '+str(launcher)+' check'}), flush=True)
        except Exception:
            shutil.rmtree(stage,ignore_errors=True)
            raise
    if args.start:
        os.execv(str(root/'cuan'),[str(root/'cuan'),'start'])


if __name__ == '__main__':
    try:
        main()
    except RuntimeError as error:
        print('Native installation failed: '+str(error),file=sys.stderr)
        sys.exit(1)
    except (OSError,ValueError,subprocess.SubprocessError):
        print('Native installation failed. Existing current release and configuration are retained. Inspect local build output or private migration.log; no connection values are printed.',file=sys.stderr)
        sys.exit(1)
