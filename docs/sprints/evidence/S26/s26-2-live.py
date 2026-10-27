"""Read-only staging snapshot; credentials/token stay private. Run before/after boot."""
import json, pathlib, subprocess, sys
ROOT = pathlib.Path(__file__).resolve().parents[4]
BASE = "https://cuan-s25-dojo.fural.space"
SID = "CSd9bffddac00994453568f6cc"
TOKEN = (pathlib.Path.home()/".local/share/cuan-s25/checks/staging-token").read_text().strip()
def api(path):
    r = subprocess.run(["curl","--fail","--silent","--show-error","--max-time","10","--config","-","--url",BASE+path], input='header = "Authorization: Bearer '+TOKEN+'"\n',text=True,capture_output=True,check=True)
    return json.loads(r.stdout)
proof = json.loads((ROOT/"docs/sprints/evidence/S26/pin2-real-nonce-reply.json").read_text())
agents = api("/api/v1/agents")
rows = agents.get("agents", agents if isinstance(agents,list) else [])
assert {a["id"] for a in rows} == {"agent_pin1","agent_pin2"}
messages = api("/api/v1/chat/conversations/"+proof["conversation_id"]+"/messages")["messages"]
assert any(m["uuid"]==proof["reply"]["uuid"] and m.get("content",m.get("message"))==proof["reply"]["content"] for m in messages)
result = {"fixture":False,"agents":[{k:a.get(k) for k in ("id","connection_id","connection_state")} for a in rows],
          "pin2ActualNonceRetained":True,"readiness":api("/readyz")}
old = json.loads((ROOT/"docs/sprints/evidence/S25/real-inbox-reply.json").read_text())
messages = api("/api/v1/chat/conversations/"+"agent_pin1"+"/messages")["messages"]
result["pin1PriorRealReplyRetained"] = any(m["uuid"]==old["agent_reply_uuid"] for m in messages)
assert result["pin1PriorRealReplyRetained"]
private = pathlib.Path.home()/".local/share/cuan-s25/checks/s26-2-before.json"
mapping = {a["id"]:a["connection_id"] for a in rows}
if sys.argv[1]=="before":
    private.write_text(json.dumps(mapping)); private.chmod(0o600)
else:
    result["mappingUnchangedAcrossRestart"] = mapping==json.loads(private.read_text())
    assert result["mappingUnchangedAcrossRestart"]
out = ROOT/("docs/sprints/evidence/S26/s26-2-"+sys.argv[1]+".json")
out.write_text(json.dumps(result,indent=2)+"\n")
print(json.dumps(result))
