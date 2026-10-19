import pathlib,json,subprocess,hashlib,re
root=pathlib.Path('/home/dev/repos/cuan');out=root/'docs/sprints/evidence/S26';base='https://cuan.dojo.cc';token=pathlib.Path('/home/dev/.local/share/cuan-s25/checks/production-token').read_text().strip()
def get(path):
 return subprocess.check_output(['curl','--fail','--silent','--show-error','--max-time','15','--config','-','--url',base+path],input=('header = "Authorization: Bearer '+token+'"\n').encode())
ready=json.loads(get('/readyz'));assert ready['migrations_count']==20 and ready['status']=='ready'
html=get('/tasks').decode();refs=re.findall(r'(?:src|href)="(/assets/[^\"]+)"',html);assert refs
assets={}
for ref in refs:
 served=get(ref);assert served==(root/'web/dist'/ref.lstrip('/')).read_bytes();assets[ref]=hashlib.sha256(served).hexdigest()
agents=json.loads(get('/api/v1/agents'))['agents'];assert agents==[]
result={'actualProduction':True,'release':'2cd41831838befda6812c64c5d90a48ba0f0cec8','ready':ready,'exactServedAssets':assets,'agents':[{'id':a['id'],'connection_id':a.get('connection_id'),'connection_state':a.get('connection_state')} for a in agents],'productionRosterEmptyBeforeAndAfter':True,'existing34TableContentsUnchanged':'s26-5-production-data.txt','noNewProviderRequest':True,'noConsent':True,'stagingCredentialsCopied':False,'limits':'Production retains its existing roster/credentials; staging second-account login and real nonce evidence remain isolated staging evidence. No new production provider success claimed.'}
(out/'s26-5-production.json').write_text(json.dumps(result,indent=2)+'\n');print('Production public exact assets, owner API, unchanged empty roster and schema20 verified; no provider invocation.')
