package git

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestConnectionPrivateRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "connections", "owner.json")
	if err := SaveConnection(path, "fixture-secret", "fixture-token", Account{Login: "owner"}); err != nil {
		t.Fatal(err)
	}
	raw, _ := os.ReadFile(path)
	if strings.Contains(string(raw), "fixture-token") {
		t.Fatal("plaintext token persisted")
	}
	info, _ := os.Stat(path)
	if info.Mode().Perm() != 0600 {
		t.Fatal("credential file not private")
	}
	account, token, err := LoadConnection(path, "fixture-secret")
	if err != nil || account.Login != "owner" || token != "fixture-token" {
		t.Fatal("connection round trip failed")
	}
	if _, _, err := LoadConnection(path, "wrong-secret"); err == nil {
		t.Fatal("wrong key accepted")
	}
	if err := SaveConnection(path, "fixture-secret", "replacement", Account{Login: "another"}); err != nil {
		t.Fatal(err)
	}
	account, token, err = LoadConnection(path, "fixture-secret")
	if err != nil || account.Login != "another" || token != "replacement" {
		t.Fatal("replacement not atomic")
	}
}
