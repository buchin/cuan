#!/usr/bin/env python3
"""Install/check supervision for a Cuan native installation (systemd, non-root app)."""
import argparse
import json
import os
from pathlib import Path
import pwd
import re
import shutil
import stat
import subprocess
import sys
import tempfile
import time
import urllib.request

MARKER='# Managed by Cuan setup-native-service v1\n'


def unit_text(user, directory):
    # systemd supports percent specifiers even inside quoted arguments.
    def quote(value):
        return '"' + str(value).replace('\\','\\\\').replace('"','\\"').replace('%','%%') + '"'
    return (MARKER+'# Installation: '+str(directory)+'\n# User: '+user+'\n'+
        '[Unit]\nDescription=Cuan project manager\nWants=network-online.target\nAfter=network-online.target\n'
        'StartLimitIntervalSec=0\n\n[Service]\nType=simple\nUser='+user+'\n'
        'WorkingDirectory='+str(directory/'data').replace('%','%%')+'\n'
        'ExecStart='+quote(directory/'cuan')+' start\n'
        'Restart=always\nRestartSec=5\nKillMode=control-group\nTimeoutStopSec=30\n'
        'UMask=0077\n\n[Install]\nWantedBy=multi-user.target\n')


def retain_existing(path, text, replace):
    if path.is_symlink():
        raise RuntimeError('Unit path is a symbolic link; it was not changed')
    if not path.exists():
        return
    previous=path.read_text()
    if previous==text:
        return
    identity='\n'.join(text.splitlines()[:3])+'\n'
    if not previous.startswith(identity):
        raise RuntimeError('An unrelated service unit already exists; choose another --name')
    if not replace:
        raise RuntimeError('Managed service unit was customized; retain it or explicitly use --replace-managed-unit')


def installation(directory, username):
    account=pwd.getpwnam(username)
    if account.pw_uid==0:
        raise RuntimeError('Cuan must run as a non-root service user')
    if directory.is_symlink() or not directory.is_dir() or directory.stat().st_uid!=account.pw_uid:
        raise RuntimeError('Installation directory must be owned by the service user and not a symbolic link')
    config=directory/'config.json'
    if config.is_symlink() or not config.is_file() or config.stat().st_uid!=account.pw_uid or stat.S_IMODE(config.stat().st_mode)!=0o600:
        raise RuntimeError('Private config.json must be a regular file owned by the service user with mode 0600')
    saved=json.loads(config.read_text())
    if not isinstance(saved,dict) or not all(isinstance(saved.get(key),str) and saved[key] for key in ['HOME','PORT','DATABASE_URL','CUAN_AUTH_SECRET']):
        raise RuntimeError('Installed configuration is incomplete')
    if not saved['PORT'].isdigit() or not 1<=int(saved['PORT'])<=65535 or len(saved['CUAN_AUTH_SECRET'])<32:
        raise RuntimeError('Installed configuration has an invalid port or auth secret')
    home=Path(saved['HOME'])
    if not home.is_dir() or home.stat().st_uid!=account.pw_uid:
        raise RuntimeError('Configured HOME must exist and belong to the service user')
    launcher=directory/'cuan'
    if launcher.is_symlink() or not launcher.is_file() or not os.access(launcher,os.X_OK) or launcher.stat().st_uid!=account.pw_uid:
        raise RuntimeError('Installed launcher is missing, not executable or not owned by the service user')
    current=(directory/'current').resolve(strict=True)
    if not current.is_relative_to((directory/'releases').resolve()) or not (current/'cuan-server').is_file():
        raise RuntimeError('Current release is missing or outside the installation')
    return saved,account


def command(args):
    result=subprocess.run(['systemctl',*args],capture_output=True,text=True,timeout=40)
    if result.returncode:
        raise RuntimeError('systemctl failed; inspect the service with systemctl status and journalctl on this host')
    return result.stdout.strip()


def check(name,config,directory,wait=0):
    until=time.monotonic()+wait
    while True:
        try:
            active=command(['is-active',name])=='active'
            pid=command(['show',name,'-p','MainPID','--value'])
            owns_process=pid.isdigit() and int(pid)>0 and Path('/proc',pid,'exe').resolve()==(directory/'current/cuan-server').resolve()
            with urllib.request.urlopen('http://127.0.0.1:'+config['PORT']+'/readyz',timeout=3) as response:
                ready=json.load(response)
            if active and owns_process and ready.get('status')=='ready' and ready.get('database')=='connected':
                return {'serviceActive':True,'applicationReady':True,'databaseReady':True,'aiExecutionVerified':False}
        except (OSError,ValueError,RuntimeError):
            pass
        if time.monotonic()>=until:
            raise RuntimeError('Service did not become ready. Inspect its journal; saved configuration and data were not reset')
        time.sleep(.25)


def main():
    parser=argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--user',required=True,help='Existing non-root Linux account that owns the installation')
    parser.add_argument('--directory',required=True,type=Path,help='Persistent directory created by install-native.py')
    parser.add_argument('--name',default='cuan',help='Service name: cuan or cuan-<name> (default cuan)')
    parser.add_argument('--dry-run',action='store_true',help='Show the service plan without writing files or running systemctl')
    parser.add_argument('--check',action='store_true',help='Check installed unit and application readiness without writes or restart')
    parser.add_argument('--start',action='store_true',help='Enable boot startup and start the service; does not restart an already active service')
    parser.add_argument('--restart',action='store_true',help='Explicitly restart after an application upgrade, then verify readiness')
    parser.add_argument('--replace-managed-unit',action='store_true',help='Replace a customized unit only when it belongs to this installation and user')
    args=parser.parse_args()
    if sys.platform!='linux':
        parser.error('Native Linux with systemd is required')
    if not re.fullmatch(r'cuan(?:-[a-z0-9][a-z0-9-]{0,47})?',args.name):
        parser.error('--name must be cuan or cuan-<lowercase-name>')
    if not re.fullmatch(r'[a-z_][a-z0-9_-]*\$?',args.user):
        parser.error('Choose a valid Linux service account')
    if sum([args.dry_run,args.check,args.start,args.restart])>1:
        parser.error('Choose only one of --dry-run, --check, --start or --restart')
    directory=args.directory.expanduser().absolute()
    if any(character in str(directory) for character in ['\n','\r','\0']):
        parser.error('Installation path contains unsupported characters')
    config,account=installation(directory,args.user)
    name=args.name+'.service'
    path=Path('/etc/systemd/system')/name
    text=unit_text(args.user,directory)
    retain_existing(path,text,args.replace_managed_unit and not args.check)
    if args.dry_run:
        print(json.dumps({'unit':str(path),'user':args.user,'directory':str(directory),'restartPolicy':'always','stopDescendants':True,'writes':False}))
        return
    if not shutil.which('systemctl') or not Path('/run/systemd/system').is_dir():
        raise RuntimeError('systemd is not running on this host')
    if args.check:
        if not path.exists() or path.read_text()!=text:
            raise RuntimeError('Matching managed service unit is not installed')
        print(json.dumps(check(name,config,directory)))
        return
    if os.geteuid()!=0:
        raise RuntimeError('Service installation requires root; the application itself runs as --user')
    if args.start or args.restart:
        # Avoid replacing an actively served instance from a different command.
        current=command(['show',name,'-p','FragmentPath','--value'])
        if current and Path(current)!=path:
            raise RuntimeError('A service from another location already uses this name')
    path.parent.mkdir(parents=True,exist_ok=True)
    fd,temporary=tempfile.mkstemp(prefix='.cuan-service-',dir=path.parent)
    try:
        with os.fdopen(fd,'w') as output:
            output.write(text);output.flush();os.fsync(output.fileno())
        os.chmod(temporary,0o644)
        os.replace(temporary,path)
    finally:
        Path(temporary).unlink(missing_ok=True)
    command(['daemon-reload'])
    if args.start:
        command(['enable','--now',name])
    elif args.restart:
        command(['enable',name])
        command(['restart',name])
    result={'unitInstalled':True,'unit':name,'applicationRunsAs':args.user,'providerInvocations':0}
    if args.start or args.restart:
        result.update(check(name,config,directory,wait=20))
    print(json.dumps(result))


if __name__=='__main__':
    try:
        main()
    except (OSError,ValueError,KeyError,RuntimeError,subprocess.SubprocessError) as error:
        if isinstance(error,RuntimeError):
            print('Cuan service setup failed: '+str(error),file=sys.stderr)
        else:
            print('Cuan service setup failed. Check the installation and service journal; private configuration was not printed.',file=sys.stderr)
        sys.exit(1)
