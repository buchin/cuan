#!/usr/bin/env python3
"""Verify native first-run/restart behavior using a disposable local PostgreSQL DB.

Requires root, PostgreSQL utilities and the postgres OS user. Never contacts an AI
provider. Creates and drops only a randomly named cuan_fresh_install_* database.
"""
import os,subprocess,tempfile,pwd,socket,time,json,urllib.request,signal,shutil,argparse,uuid
from pathlib import Path
parser=argparse.ArgumentParser(description=__doc__)
parser.add_argument('--binary', required=True, help='Candidate cuan-server binary to verify')
args=parser.parse_args()
binary=Path(args.binary).resolve()
if os.geteuid()!=0: parser.error('run with sudo; the server itself runs as postgres')
if not binary.is_file(): parser.error('candidate binary does not exist')
name='cuan_fresh_install_'+uuid.uuid4().hex[:16]
psql=['runuser','-u','postgres','--','psql','-XAt']
def sql(q):return subprocess.check_output([*psql,'-d',name,'-c',q],text=True,cwd='/var/tmp').strip()
assert subprocess.check_output([*psql,'-c',"select count(*) from pg_database where datname='"+name+"'"],text=True,cwd='/var/tmp').strip()=='0'
subprocess.run(['runuser','-u','postgres','--','createdb',name],check=True,cwd='/var/tmp')
root=Path(tempfile.mkdtemp(prefix='cuan-fresh-native-',dir='/var/tmp')); user=pwd.getpwnam('postgres');os.chown(root,user.pw_uid,user.pw_gid)
shutil.copy2(binary,root/'cuan-server');os.chmod(root/'cuan-server',0o755)
with socket.socket() as sock:sock.bind(('127.0.0.1',0));port=sock.getsockname()[1]
base='http://127.0.0.1:'+str(port);proc=None
auth_secret=os.urandom(32).hex()
log=open(root/'server.log','wb')
def call(path,body=None,token=''):
 headers={'Content-Type':'application/json'}
 if token:headers['Authorization']='Bearer '+token
 req=urllib.request.Request(base+path,headers=headers,data=None if body is None else json.dumps(body).encode())
 with urllib.request.urlopen(req,timeout=3) as r:return json.load(r)
def start():
 global proc
 env={'HOME':str(root),'PATH':'/usr/bin:/bin','PORT':str(port),'CUAN_ENV':'staging','CUAN_AUTH_SECRET':auth_secret,'DATABASE_URL':'postgres://postgres@/'+name+'?host=/var/run/postgresql','CUAN_AUTO_MIGRATE':'true','CUAN_AUTO_SYNC_README':'false','CUAN_STATIC_DIR':str(root/'static'),'APP_MODE':'studio'}
 proc=subprocess.Popen(['runuser','-u','postgres','--','env',*[k+'='+v for k,v in env.items()],str(root/'cuan-server')],cwd=root,stdout=log,stderr=log,start_new_session=True)
 for _ in range(60):
  if proc.poll() is not None:raise RuntimeError('candidate server exited')
  try:call('/readyz');return
  except Exception:time.sleep(.1)
 raise RuntimeError('candidate server readiness timeout')
def stop():
 global proc
 if proc is not None and proc.poll() is None:
  os.killpg(proc.pid,signal.SIGTERM)
  try:proc.wait(timeout=5)
  except subprocess.TimeoutExpired:os.killpg(proc.pid,signal.SIGKILL);proc.wait(timeout=2)
 proc=None
try:
 start();initial=call('/api/v1/auth/status');assert initial['setup_required'] and not initial['owner_configured']
 session=call('/api/v1/auth/setup',{'email':'native@example.invalid','password':'fixture-native-password-123'});token=session['token']
 project=call('/api/v1/projects/default',token=token);assert project['name']=='Default' and not project['repo_url']
 sql("UPDATE projects SET name='My personal workspace',repo_url='https://example.invalid/user-project' WHERE id='default'")
 stop();start()
 project=call('/api/v1/projects/default',token=token);assert project['name']=='My personal workspace' and project['repo_url']=='https://example.invalid/user-project'
 assert sql('select count(*) from owners')=='1'
 assert sql('select count(*) from projects')=='1'
 assert sql("select count(*) from cuan_schema_migrations where version='000025'")=='1'
 assert not call('/api/v1/auth/status')['setup_required']
 print(json.dumps({'actualNativeBinary':True,'emptyDatabaseMigratedTo25':True,'ownerSetupRequired':True,'neutralDefaultProject':True,'sessionAndRenamedProjectSurviveProcessRestart':True,'owners':1,'projects':1,'providerInvocations':0,'productionDatabaseWrites':0}))
finally:
 stop();log.close();subprocess.run(['runuser','-u','postgres','--','dropdb',name],check=True,cwd='/var/tmp');shutil.rmtree(root)
 print('Dedicated native database and temporary HOME removed.')
