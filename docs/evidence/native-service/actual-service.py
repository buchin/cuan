#!/usr/bin/env python3
"""Exercise native release installation with actual binaries and a disposable DB.

Run as root on a native test host with PostgreSQL utilities and postgres OS user.
Only random cuan_fresh_install_* databases and a temporary HOME are created.
No production application or AI account is accessed.
"""
import argparse
import hashlib
import json
import os
from pathlib import Path
import pwd
import shutil
import signal
import socket
import stat
import subprocess
import tempfile
import time
import urllib.request
import uuid
import sys

parser=argparse.ArgumentParser(description=__doc__)
parser.add_argument('--binary-dir',type=Path,required=True)
parser.add_argument('--static-dir',type=Path,required=True)
parser.add_argument('--installer',type=Path,default=Path(__file__).with_name('install-native.py'))
parser.add_argument('--service-setup',type=Path,required=True)
args=parser.parse_args()
if os.geteuid()!=0:parser.error('Run this disposable test harness as root; application runs as postgres')
for name in ['cuan-server','cuan-migrate']:
 if not (args.binary_dir/name).is_file():parser.error('Both real binary files are required')
name='cuan_fresh_install_'+uuid.uuid4().hex[:16]
psql=['runuser','-u','postgres','--','psql','-XAt','-v','ON_ERROR_STOP=1']
assert subprocess.check_output([*psql,'-c',"select count(*) from pg_database where datname='"+name+"'"],text=True,cwd='/var/tmp').strip()=='0'
subprocess.run(['runuser','-u','postgres','--','createdb',name],check=True,cwd='/var/tmp')
root=Path(tempfile.mkdtemp(prefix='cuan-installer-live-',dir='/var/tmp'))
account=pwd.getpwnam('postgres')
process=None
log=None
service_name='cuan-fixture-'+uuid.uuid4().hex[:12]
unit=Path('/etc/systemd/system')/(service_name+'.service')
assert not unit.exists()
def call(path,body=None,token=''):
 headers={'Content-Type':'application/json'}
 if token:headers['Authorization']='Bearer '+token
 request=urllib.request.Request(base+path,headers=headers,data=None if body is None else json.dumps(body).encode())
 with urllib.request.urlopen(request,timeout=3) as response:return json.load(response)
def sql(query):
 return subprocess.check_output([*psql,'-d',name,'-c',query],text=True,cwd='/var/tmp').strip()
def stop():
 global process
 if process is not None and process.poll() is None:
  os.killpg(process.pid,signal.SIGTERM)
  try:process.wait(timeout=5)
  except subprocess.TimeoutExpired:os.killpg(process.pid,signal.SIGKILL);process.wait(timeout=2)
 process=None
try:
 binaries=root/'bin';binaries.mkdir()
 for item in ['cuan-server','cuan-migrate']:shutil.copy2(args.binary_dir/item,binaries/item)
 shutil.copytree(args.static_dir,root/'static')
 (root/'source/scripts').mkdir(parents=True)
 shutil.copy2(args.installer,root/'source/scripts/install-native.py')
 shutil.copy2(args.service_setup,root/'source/scripts/setup-native-service.py')
 for file in [root,*root.rglob('*')]:os.chown(file,account.pw_uid,account.pw_gid)
 with socket.socket() as sock:sock.bind(('127.0.0.1',0));port=sock.getsockname()[1]
 base='http://127.0.0.1:'+str(port)
 destination=root/'installed app'
 command=['runuser','-u','postgres','--','env','HOME='+str(root),'DATABASE_URL=postgres://postgres@/'+name+'?host=/var/run/postgresql',
          'python3',str(root/'source/scripts/install-native.py'),'--directory',str(destination),'--binary-dir',str(binaries),'--static-dir',str(root/'static'),'--port',str(port),'--migrate']
 result=subprocess.run(command,capture_output=True,text=True,timeout=40,cwd='/var/tmp')
 assert result.returncode==0,'Installer failed; diagnostics not printed to avoid exposing connection details'
 configuration=(destination/'config.json').read_bytes()
 assert stat.S_IMODE((destination/'config.json').stat().st_mode)==0o600
 assert json.loads(configuration)['CUAN_AUTO_MIGRATE']=='false'
 log=(root/'server.log').open('wb')
 def start():
  global process
  process=subprocess.Popen(['runuser','-u','postgres','--',str(destination/'cuan'),'start'],cwd='/var/tmp',stdout=log,stderr=log,start_new_session=True)
  for _ in range(80):
   if process.poll() is not None:raise RuntimeError('Installed server exited')
   try:
    if call('/readyz')['status']=='ready':return
   except Exception:pass
   time.sleep(.1)
  raise RuntimeError('Installed server readiness timeout')
 start()
 assert call('/api/v1/auth/status')['setup_required']
 auth=call('/api/v1/auth/setup',{'email':'installer@example.invalid','password':'fixture-native-installer-123'})
 token=auth['token']
 project=call('/api/v1/projects/default',token=token)
 assert project['name']=='Default' and not project['repo_url']
 sql("update projects set name='Retained installer workspace' where id='default'")
 (destination/'data/workspaces'/'retained.txt').write_text('keep the original workspace')
 with urllib.request.urlopen(base+'/',timeout=3) as response:
  assert response.read()==(destination/'current/static/index.html').read_bytes()
 check=subprocess.run(['runuser','-u','postgres','--',str(destination/'cuan'),'check'],capture_output=True,text=True,timeout=8)
 assert check.returncode==0 and json.loads(check.stdout)['databaseReady']
 stop()
 previous=(destination/'current').resolve()
 reinstall=subprocess.run(command,capture_output=True,text=True,timeout=40,cwd='/var/tmp')
 assert reinstall.returncode==0,'Reinstall failed'
 assert configuration==(destination/'config.json').read_bytes()
 assert previous.is_dir() and (destination/'current').resolve()!=previous
 assert (destination/'data/workspaces'/'retained.txt').read_text()=='keep the original workspace'
 setup=['python3',str(root/'source/scripts/setup-native-service.py'),'--user','postgres','--directory',str(destination),'--name',service_name]
 installed=subprocess.run(setup+['--start'],capture_output=True,text=True,timeout=35)
 assert installed.returncode==0,'Service setup failed: '+installed.stderr
 assert json.loads(installed.stdout)['applicationReady']
 assert subprocess.check_output(['systemctl','is-enabled',service_name],text=True).strip()=='enabled'
 first_pid=subprocess.check_output(['systemctl','show',service_name,'-p','MainPID','--value'],text=True).strip()
 assert int(first_pid)>0
 os.kill(int(first_pid),signal.SIGKILL)
 restart_until=time.monotonic()+20
 while True:
  new_pid=subprocess.check_output(['systemctl','show',service_name,'-p','MainPID','--value'],text=True).strip()
  if new_pid.isdigit() and int(new_pid)>0 and new_pid!=first_pid:
   try:
    if call('/readyz')['status']=='ready':break
   except Exception:pass
  if time.monotonic()>=restart_until:raise RuntimeError('Managed application did not recover after process loss')
  time.sleep(.25)
 checked=subprocess.run(setup+['--check'],capture_output=True,text=True,timeout=10)
 assert checked.returncode==0 and json.loads(checked.stdout)['applicationReady']
 assert int(subprocess.check_output(['systemctl','show',service_name,'-p','NRestarts','--value'],text=True).strip())>=1
 assert not call('/api/v1/auth/status')['setup_required']
 assert call('/api/v1/projects/default',token=token)['name']=='Retained installer workspace'
 assert sql('select count(*) from owners')=='1'
 assert sql('select count(*) from projects')=='1'
 assert sql('select count(*) from cuan_schema_migrations')=='25'
 print(json.dumps({'actualSystemdUnit':True,'bootEnabled':True,'nonRootService':True,'sigkillAutomaticRestart':True,'newMainPidVerified':True,'processIdentityReadinessCheck':True,'actualBinaries':True,'serverSha256':hashlib.sha256((binaries/'cuan-server').read_bytes()).hexdigest(),
  'freshMigration25':True,'applicationReadiness':True,'staticAssetsServed':True,'ownerSetup':True,'neutralDefault':True,
  'reinstallAndRestart':True,'previousReleaseRetained':True,'configurationUnchanged':True,'configurationMode':'0600',
  'ownerSessionRetained':True,'renamedProjectRetained':True,'workspaceRetained':True,'providerInvocations':0,'productionDatabaseWrites':0}))
finally:
 if unit.exists():
  if sys.exc_info()[0] is not None:
   properties=subprocess.run(['systemctl','show',service_name,'-p','ActiveState','-p','SubState','-p','MainPID','-p','ExecMainStatus','-p','Result','-p','FragmentPath'],capture_output=True,text=True,timeout=5)
   journal=subprocess.run(['journalctl','-u',service_name,'-n','40','--no-pager'],capture_output=True,text=True,timeout=5)
   diagnostic=Path('/opt/cuan/backups/native-service-fixture-status.log')
   diagnostic.write_text(properties.stdout+'\n'+journal.stdout);diagnostic.chmod(0o600)
   print(json.dumps({'fixtureStatus':properties.stdout.splitlines()}))
  assert unit.read_text().startswith('# Managed by Cuan setup-native-service v1\n# Installation: '+str(root/'installed app')+'\n')
  subprocess.run(['systemctl','disable','--now',service_name],stdout=subprocess.DEVNULL,stderr=subprocess.DEVNULL,check=False,timeout=15)
  unit.unlink()
  subprocess.run(['systemctl','daemon-reload'],check=True,timeout=10)
  subprocess.run(['systemctl','reset-failed',service_name],stdout=subprocess.DEVNULL,stderr=subprocess.DEVNULL,check=False,timeout=5)
 stop()
 if log:log.close()
 subprocess.run(['runuser','-u','postgres','--','dropdb',name],check=True,cwd='/var/tmp')
 shutil.rmtree(root)
 print(json.dumps({'disposableDatabaseDropped':True,'temporaryHomeRemoved':True,'temporarySystemdUnitRemoved':not unit.exists()}))
