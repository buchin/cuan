package diagnostics

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestChatDownloadOwnerAuthAndNoOverwrite(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.Method != "GET" || r.URL.Path != "/api/v1/chat/attachments/fixture-id" || r.Header.Get("Authorization") != "Bearer fixture-token" {
			t.Error("unexpected attachment request")
		}
		w.Write([]byte("# Actual fixture artifact\n"))
	}))
	defer server.Close()
	old := globalCuanClient
	globalCuanClient = &CuanClient{BaseURL: server.URL, AuthToken: "fixture-token", HTTPClient: server.Client()}
	t.Cleanup(func() { globalCuanClient = old })
	dest := filepath.Join(t.TempDir(), "guide.md")
	var out, errOut bytes.Buffer
	if code := RunChatCLI([]string{"download", "fixture-id", "--output", dest}, &out, &errOut); code != 0 {
		t.Fatal(code, errOut.String())
	}
	data, _ := os.ReadFile(dest)
	if string(data) != "# Actual fixture artifact\n" {
		t.Fatal("wrong artifact")
	}
	info, _ := os.Stat(dest)
	if info.Mode().Perm() != 0600 {
		t.Fatal("artifact must be private")
	}
	if code := RunChatCLI([]string{"download", "fixture-id", "--output", dest}, &out, &errOut); code == 0 {
		t.Fatal("existing artifact overwritten")
	}
	if code := RunChatCLI([]string{"download", "../another-owner", "--output", dest}, &out, &errOut); code != 2 {
		t.Fatal("unsafe ID accepted")
	}
	if calls != 2 {
		t.Fatal("unexpected requests", calls)
	}
}
func TestChatDownloadDoesNotSaveFailureBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(403); w.Write([]byte("private-token")) }))
	defer server.Close()
	old := globalCuanClient
	globalCuanClient = &CuanClient{BaseURL: server.URL, AuthToken: "fixture", HTTPClient: server.Client()}
	t.Cleanup(func() { globalCuanClient = old })
	dest := filepath.Join(t.TempDir(), "guide.md")
	var out, errors bytes.Buffer
	if RunChatCLI([]string{"download", "id", "--output", dest}, &out, &errors) == 0 {
		t.Fatal("failed download succeeded")
	}
	if _, err := os.Stat(dest); !os.IsNotExist(err) {
		t.Fatal("failure body saved")
	}
	if strings.Contains(out.String()+errors.String(), "private-token") {
		t.Fatal("failure body leaked")
	}
}
