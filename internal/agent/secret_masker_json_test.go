package agent

import (
	"encoding/json"
	"testing"
)

// Stored task instructions embed project guidance as JSON inside a fenced block.
// The SSH host-key pattern used to consume the closing quote of the surrounding
// JSON string, so the snapshot could never be parsed again and every task for
// that project was rejected before an attempt started.
func TestMaskSecretsKeepsEmbeddedJSONValid(t *testing.T) {
	doc := map[string]any{
		"isolated_repository": true,
		"project_id":          "proj_0123456789abcdef0123456789abcdef",
		"environment":         "CUAN_PUBLICATION_SSH_HOST_KEY=ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAINspux4JFwlYuqloxDv0xMPd56KGC\nCUAN_MANAGED_RELEASE=1\n",
	}
	raw, err := json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	if !json.Valid(raw) {
		t.Fatal("fixture is not valid JSON")
	}

	masked := MaskSecrets(string(raw), nil)
	if !json.Valid([]byte(masked)) {
		t.Fatalf("masking broke the JSON document: %s", masked)
	}
	var back map[string]any
	if err := json.Unmarshal([]byte(masked), &back); err != nil {
		t.Fatalf("masked document no longer parses: %v", err)
	}
	env, _ := back["environment"].(string)
	if contains(env, "AAAAC3NzaC1lZDI1NTE5AAAAINspux4JFwlYuqloxDv0xMPd56KGC") {
		t.Fatalf("host key material was not redacted: %q", env)
	}
	if back["project_id"] != doc["project_id"] {
		t.Fatal("masking changed an unrelated field")
	}
}

// A host key printed in prose is still a credential.
func TestMaskSecretsRedactsHostKeyInProse(t *testing.T) {
	in := "Host key: ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAINspux4JFwlYuqloxDv0xMPd56KGC root@host"
	out := MaskSecrets(in, nil)
	if contains(out, "AAAAC3NzaC1lZDI1NTE5AAAAINspux4JFwlYuqloxDv0xMPd56KGC") {
		t.Fatalf("host key not redacted: %q", out)
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
