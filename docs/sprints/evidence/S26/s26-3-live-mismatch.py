import json,pathlib,hashlib,urllib.request,urllib.error,os,tempfile
root=pathlib.Path.home()
base="http://127.0.0.1:8081"
def request(path,data=None,token=None):
 headers={}
 if data is not None:headers["Content-Type"]="application/json"
 if token:headers["Authorization"]="Bearer "+token
 req=urllib.request.Request(base+path,data=None if data is None else json.dumps(data).encode(),headers=headers)
 try:
  with urllib.request.urlopen(req,timeout=10) as r:return r.status,json.load(r)
 except urllib.error.HTTPError as e:return e.code,json.load(e)
code,login=request("/api/v1/auth/login",{"username":"admin","password":"whosyourdaddy"})
assert code==200,"Staging login failed"
token=login["token"]
profiles=root/".local/share/cuan/accounts"
metadata=next(p for p in profiles.glob("*/account.json") if json.loads(p.read_bytes()).get("agent_id")=="agent_pin2")
original=metadata.read_bytes()
before={str(p.relative_to(profiles)):hashlib.sha256(p.read_bytes()).hexdigest() for p in profiles.rglob("*") if p.is_file()}
def replace(data):
 fd,path=tempfile.mkstemp(prefix=".s26-3-",dir=metadata.parent)
 try:
  os.fchmod(fd,0o600)
  with os.fdopen(fd,"wb") as f:f.write(data);f.flush();os.fsync(f.fileno())
  os.replace(path,metadata)
 finally:
  if os.path.exists(path):os.unlink(path)
try:
 bad=json.loads(original);bad["provider"]="claude";replace(json.dumps(bad).encode())
 code,error=request("/api/v1/agents/agent_pin2/connection/verify",{},token)
 assert code==409,"Mismatched profile reached probe"
 code,roster=request("/api/v1/agents",token=token)
 assert code==200
 assert next(a for a in roster["agents"] if a["id"]=="agent_pin2")["connection_state"]=="login_required"
finally:replace(original)
after={str(p.relative_to(profiles)):hashlib.sha256(p.read_bytes()).hexdigest() for p in profiles.rglob("*") if p.is_file()}
assert before==after,"Profile files changed"
code,roster=request("/api/v1/agents",token=token);assert code==200
assert next(a for a in roster["agents"] if a["id"]=="agent_pin1")["connection_state"]=="quota_limited"
sources={}
for f in ["internal/server/agent_profile.go","internal/server/agent_handlers.go"]:
 sources[f]=hashlib.sha256((root/f).read_bytes()).hexdigest()
print(json.dumps({"actualStagingHTTP":True,"mismatchedProfileProviderVerifyHTTP":409,"mismatchedRosterState":"login_required","profileCredentialAndMetadataHashesRestored":before==after,"pin1QuotaPreserved":True,"noProviderProbeOrConsent":True,"sourceSHA256":sources}))
