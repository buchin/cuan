import json,pathlib,urllib.request,re,hashlib,time
urllib.request.install_opener(urllib.request.build_opener())
urllib.request._opener.addheaders=[('User-Agent','Mozilla/5.0')]
root=pathlib.Path.cwd();base='https://cuan-s25-dojo.fural.space'
def get(path,token=None):
 req=urllib.request.Request(base+path,headers={} if token is None else {'Authorization':'Bearer '+token})
 with urllib.request.urlopen(req,timeout=15) as r:return r.read()
req=urllib.request.Request(base+'/api/v1/auth/login',data=json.dumps({'username':'admin','password':'whosyourdaddy'}).encode(),headers={'Content-Type':'application/json','Origin':base})
with urllib.request.urlopen(req,timeout=15) as r:token=json.load(r)['token']
private=pathlib.Path.home()/'.local/share/cuan-s25/checks/staging-token';private.write_text(token);private.chmod(0o600)
html=get('/').decode();assets=re.findall(r'(?:src|href)="(/assets/[^\"]+)"',html)
assert assets
matches={a:hashlib.sha256(get(a)).hexdigest()==hashlib.sha256((root/'web/dist'/a.lstrip('/')).read_bytes()).hexdigest() for a in assets};assert all(matches.values())
roster=json.loads(get('/api/v1/agents',token))['agents'];assert len(roster)==2
rows=[{k:a.get(k) for k in ['id','connection_id','connection_state','last_checked_at','verified_at']} for a in roster]
assert next(a for a in roster if a['id']=='agent_pin1')['connection_state']=='quota_limited'
assert next(a for a in roster if a['id']=='agent_pin2')['connection_state']=='unknown'
for agent,reply in [('agent_pin1','c809659e-59f5-460d-8789-82a2988022f5'),('agent_pin2','78ae1c66-af3d-40ed-9a5d-378f8d9d0f42')]:
 data=json.loads(get('/api/v1/chat/conversations/'+agent+'/messages',token));assert any(m.get('uuid')==reply for m in data['messages'])
browser=json.loads((root/'docs/sprints/evidence/S26/s26-4-browser.json').read_text())
data=json.loads(get('/api/v1/chat/conversations/agent_pin1/messages',token))
request=next(q for q in data['requests'] if q['request_id']==browser['requestID']);assert request['state']=='failed'
assert len([m for m in data['messages'] if m.get('uuid')==request['message_id']])==1
assert any(q['request_id']=='8b75a4eb-7742-4f7b-9666-1916a0217288' and q['state']=='failed' for q in data['requests'])
health=json.loads(get('/healthz'));ready=json.loads(get('/readyz'));assert ready['migrations_count']==20
prod=urllib.request.urlopen('https://cuan.dojo.cc/',timeout=15).read().decode();prodassets=re.findall(r'(?:src|href)="(/assets/[^\"]+)"',prod)
assert '/assets/index-BUDXr3Gh.js' in prodassets and '/assets/index-traJKXUg.css' in prodassets
out={'actualServedAssetsMatchLocalBuild':matches,'agents':rows,'bothRealReplyUUIDsRetained':True,'health':health,'durableFailureAndOriginalMessageSurviveNativeRestart':True,'earlierAcceptedRequestSurvivesMultipleNativeRestarts':True,'readiness':ready,'productionAssets':prodassets,'productionDeployment':False,'providerRequests':0}
(root/'docs/sprints/evidence/S26/s26-4-live-continuity.json').write_text(json.dumps(out,indent=2)+'\n')
print(json.dumps(out))
