package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/buchin/cuan/internal/git"
)

func deviceLoginFixture(t *testing.T) (*Server, func(string, string) *httptest.ResponseRecorder, string, string) {
	t.Helper()
	s, token := setupTestChatServer(t)
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("GH_TOKEN", "AMBIENT_FURAL_TOKEN_SENTINEL")
	t.Setenv("GITHUB_TOKEN", "AMBIENT_GITHUB_TOKEN_SENTINEL")
	t.Setenv("CUAN_GITHUB_CLIENT_ID", "")
	if err := os.MkdirAll(filepath.Join(home, ".config", "gh"), 0700); err != nil {
		t.Fatal(err)
	}
	global := filepath.Join(home, ".config", "gh", "hosts.yml")
	os.WriteFile(global, []byte("AMBIENT_PROFILE_UNCHANGED"), 0600)
	root := t.TempDir()
	approve := filepath.Join(root, "approve")
	cli := filepath.Join(root, "gh")
	script := `#!/bin/sh
[ -z "$GH_TOKEN$GITHUB_TOKEN$FURAL_AGENT_TOKEN$SSH_AUTH_SOCK$DOJO_FURAL_API_TOKEN" ] || exit 30
[ "$GH_CONFIG_DIR" = "$HOME/.config/gh" ] || exit 31
case "$*" in
 "auth login --hostname github.com --git-protocol https --web --insecure-storage")
 [ ! -f "$GH_CONFIG_DIR/hosts.yml" ] || exit 32
 printf '%s' "$HOME" > '` + root + `/profile'
 echo login >> '` + root + `/calls'
 printf '! First copy your one-time code: ABCD-EFGH\n' >&2
 while [ ! -f '` + approve + `' ];do sleep 0.02;done
 mkdir -p "$GH_CONFIG_DIR"
 printf fixture-device-token > "$GH_CONFIG_DIR/hosts.yml" ;;
 "auth token --hostname github.com") cat "$GH_CONFIG_DIR/hosts.yml" ;;
 *) exit 33 ;;
esac
`
	if err := os.WriteFile(cli, []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	original := ensureGitHubDeviceBinary
	ensureGitHubDeviceBinary = func(context.Context) (string, error) { return cli, nil }
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/user" || r.Header.Get("Authorization") != "Bearer fixture-device-token" {
			w.WriteHeader(401)
			return
		}
		w.Write([]byte(`{"login":"device-owner","id":123}`))
	}))
	oldBase := githubOAuthAPIBase
	githubOAuthAPIBase = provider.URL
	t.Cleanup(func() {
		s.cancelGitHubDeviceOwner("")
		for _, j := range s.githubDeviceJobs {
			select {
			case <-j.done:
			case <-time.After(3 * time.Second):
				t.Error("device process did not end")
			}
		}
		ensureGitHubDeviceBinary = original
		githubOAuthAPIBase = oldBase
		provider.Close()
	})
	call := func(method, path string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, path, strings.NewReader(`{}`))
		r.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()
		s.Router().ServeHTTP(w, r)
		return w
	}
	return s, call, root, home
}
func decodeDevice(t *testing.T, w *httptest.ResponseRecorder) githubDeviceStatus {
	t.Helper()
	if w.Code != 200 {
		t.Fatalf("device API failed: %d", w.Code)
	}
	var data struct {
		Device githubDeviceStatus `json:"device_login"`
	}
	if json.Unmarshal(w.Body.Bytes(), &data) != nil || data.Device.ID == "" {
		t.Fatal("missing device login")
	}
	return data.Device
}
func waitDevice(t *testing.T, call func(string, string) *httptest.ResponseRecorder, id, state string) githubDeviceStatus {
	t.Helper()
	until := time.Now().Add(3 * time.Second)
	for time.Now().Before(until) {
		d := decodeDevice(t, call("GET", "/api/v1/integrations/github/device/"+id))
		if d.State == state {
			return d
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("device state not reached", state)
	return githubDeviceStatus{}
}

func TestGitHubDeviceLoginIsolatedIdempotentAndEncrypted(t *testing.T) {
	s, call, root, home := deviceLoginFixture(t)
	first := decodeDevice(t, call("POST", "/api/v1/integrations/github/oauth"))
	second := decodeDevice(t, call("POST", "/api/v1/integrations/github/oauth"))
	if first.ID != second.ID {
		t.Fatal("duplicate sign-in")
	}
	pending := waitDevice(t, call, first.ID, "awaiting_authorization")
	if pending.UserCode != "ABCD-EFGH" || pending.VerificationURL != "https://github.com/login/device" {
		t.Fatal("missing device instructions")
	}
	availability := call("GET", "/api/v1/integrations/github/oauth")
	if !strings.Contains(availability.Body.String(), first.ID) {
		t.Fatal("reload does not resume pending sign-in")
	}
	os.WriteFile(filepath.Join(root, "approve"), []byte("test owner consent"), 0600)
	waitDevice(t, call, first.ID, "connected")
	job := s.githubDeviceJobs[first.ID]
	<-job.done
	connection := call("GET", "/api/v1/integrations/github")
	if !strings.Contains(connection.Body.String(), `"connected":true`) || !strings.Contains(connection.Body.String(), "device-owner") || strings.Contains(connection.Body.String(), "fixture-device-token") {
		t.Fatal("invalid public connection")
	}
	owner, _ := s.authSvc.OwnerID(context.Background())
	path, _ := githubConnectionPath(owner)
	raw, _ := os.ReadFile(path)
	if strings.Contains(string(raw), "fixture-device-token") {
		t.Fatal("plaintext token stored")
	}
	_, saved, err := git.LoadConnection(path, s.cfg.AuthSecret)
	if err != nil || saved != "fixture-device-token" {
		t.Fatal("connection not usable")
	}
	profile, _ := os.ReadFile(filepath.Join(root, "profile"))
	if !strings.HasPrefix(string(profile), filepath.Join(home, ".local", "share", "cuan", "github-device")) {
		t.Fatal("ambient profile used")
	}
	if _, err := os.Stat(string(profile)); !os.IsNotExist(err) {
		t.Fatal("temporary credentials retained")
	}
	original, _ := os.ReadFile(filepath.Join(home, ".config", "gh", "hosts.yml"))
	if string(original) != "AMBIENT_PROFILE_UNCHANGED" {
		t.Fatal("ambient credentials modified")
	}
	calls, _ := os.ReadFile(filepath.Join(root, "calls"))
	if string(calls) != "login\n" {
		t.Fatal("multiple login processes")
	}
}
func TestGitHubDeviceCancelCannotSaveLateCredentials(t *testing.T) {
	s, call, root, _ := deviceLoginFixture(t)
	first := decodeDevice(t, call("POST", "/api/v1/integrations/github/oauth"))
	waitDevice(t, call, first.ID, "awaiting_authorization")
	cancelled := decodeDevice(t, call("POST", "/api/v1/integrations/github/device/"+first.ID+"/cancel"))
	if cancelled.State != "cancelled" || cancelled.UserCode != "" {
		t.Fatal("not cancelled")
	}
	os.WriteFile(filepath.Join(root, "approve"), []byte("late"), 0600)
	<-s.githubDeviceJobs[first.ID].done
	owner, _ := s.authSvc.OwnerID(context.Background())
	path, _ := githubConnectionPath(owner)
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("late credentials saved after cancellation")
	}
	if got := decodeDevice(t, call("GET", "/api/v1/integrations/github/device/"+first.ID)); got.State != "cancelled" {
		t.Fatal("cancel overwritten")
	}
}
func TestGitHubDeviceAuthorizationAndDisconnect(t *testing.T) {
	s, call, _, _ := deviceLoginFixture(t)
	first := decodeDevice(t, call("POST", "/api/v1/integrations/github/oauth"))
	waitDevice(t, call, first.ID, "awaiting_authorization")
	noAuth := httptest.NewRecorder()
	s.Router().ServeHTTP(noAuth, httptest.NewRequest("GET", "/api/v1/integrations/github/device/"+first.ID, nil))
	if noAuth.Code != 401 {
		t.Fatal("login code exposed without authentication")
	}
	s.githubDeviceMu.Lock()
	s.githubDeviceJobs["other-owner"] = &githubDeviceLogin{owner: "someone-else", status: githubDeviceStatus{ID: "other-owner"}}
	s.githubDeviceMu.Unlock()
	if w := call("GET", "/api/v1/integrations/github/device/other-owner"); w.Code != 404 {
		t.Fatal("another owner's login exposed")
	}
	s.githubDeviceMu.Lock()
	delete(s.githubDeviceJobs, "other-owner")
	s.githubDeviceMu.Unlock()
	if w := call("DELETE", "/api/v1/integrations/github"); w.Code != 200 {
		t.Fatal("disconnect failed")
	}
	<-s.githubDeviceJobs[first.ID].done
	if s.githubDeviceJobs[first.ID].snapshot().State != "cancelled" {
		t.Fatal("disconnect left pending sign-in")
	}
}
