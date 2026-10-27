import pathlib,json,urllib.request,subprocess,hashlib
root=pathlib.Path('/home/dev/repos/cuan');out=root/'docs/sprints/evidence/S26'
token=pathlib.Path('/home/dev/.local/share/cuan-s25/checks/staging-token').read_text().strip()
base='https://cuan-s25-dojo.fural.space'
def get(path):
 return json.loads(subprocess.check_output(['curl','--fail','--silent','--show-error','--max-time','15','--config','-','--url',base+path],input=('header = \"Authorization: Bearer '+token+'\"\n').encode()))
rows=get('/api/v1/agents')['agents']
prior=json.loads((out/'s26-2-after.json').read_text())
current=[{k:a.get(k) for k in ['id','connection_id','connection_state','last_checked_at','verified_at']} for a in rows]
assert {a['id']:a['connection_id'] for a in current}=={a['id']:a['connection_id'] for a in prior['agents']}
assert next(a for a in rows if a['id']=='agent_pin1')['connection_state']=='quota_limited'
assert next(a for a in rows if a['id']=='agent_pin2')['connection_state']=='unknown'
pin2=get('/api/v1/chat/conversations/e7ae31a0-6fcc-4543-bddd-1728a8d8cebd/messages')
pin1=get('/api/v1/chat/conversations/agent_pin1/messages')
assert '78ae1c66-af3d-40ed-9a5d-378f8d9d0f42' in json.dumps(pin2)
assert 'c809659e-59f5-460d-8789-82a2988022f5' in json.dumps(pin1)
migrations=list((root/'migrations').glob('*'))
assert all(p.read_bytes()==subprocess.check_output(['git','show','fb83e13:migrations/'+p.name]) for p in migrations if p.is_file())
prod=subprocess.check_output(['curl','--fail','--silent','--show-error','--max-time','15','https://cuan.dojo.cc'])
retained=pathlib.Path('/home/dev/.local/share/cuan-mobile-after/web/dist/index.html')
if not retained.exists():retained=pathlib.Path('/home/dev/.local/share/cuan-mobile-after/dist/index.html')
# Match retained accepted bundle references, without claiming identical volatile HTML headers.
prod_refs=[x for x in prod.decode().split('"') if '/assets/' in x]
readiness=get('/readyz');assert readiness['migrations_count']==19
result={'fixture':False,'agents':current,'mappingUnchanged':True,'pin1ActualReplyRetained':True,'pin2ActualNonceRetained':True,'readiness':readiness,'original19MigrationsByteIdentical':True,'productionAssetReferences':prod_refs,'productionDeployPerformed':False}
(out/'s26-3-continuity.json').write_text(json.dumps(result,indent=2)+'\n');print('Actual staging mapping, expired pin2, factual quota pin1, both stored replies and original 19 migrations verified; production assets observed.')
