package agent

import (
	"context"
	"encoding/json"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestS26AccountPersistenceOwnerAndPaths(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	a, err := CreateProviderAccount("owner-a", "Google kerja")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := GetProviderAccount("owner-b", a.ID); err == nil {
		t.Fatal("another owner read account")
	}
	for _, id := range []string{"../outside", "acct_../../outside", "", "acct_bad"} {
		if _, err := ProviderAccountHome("owner-a", id); err == nil {
			t.Fatalf("unsafe id %q accepted", id)
		}
	}
	home, err := ProviderAccountHome("owner-a", a.ID)
	if err != nil {
		t.Fatal(err)
	}
	if home == ConfigHome() {
		t.Fatal("profile uses service home")
	}
	if err := os.MkdirAll(AntigravityConfigDirPath(home), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(AntigravityCredentialsFilePath(home), []byte(`{"access_token":"fixture-only-not-a-real-token"}`), 0600); err != nil {
		t.Fatal(err)
	}
	got, err := GetProviderAccount("owner-a", a.ID)
	if err != nil || !got.CredentialsPresent || got.Label != "Google kerja" {
		t.Fatalf("metadata roundtrip failed: %v", err)
	}
	list, err := ListProviderAccounts("owner-a")
	if err != nil || len(list) != 1 {
		t.Fatalf("list: %d %v", len(list), err)
	}
	raw, _ := json.Marshal(got)
	if strings.Contains(string(raw), "owner-a") || strings.Contains(string(raw), home) || strings.Contains(string(raw), "fixture-only") {
		t.Fatal("private value leaked in DTO")
	}
	dir, _ := accountDirectory(a.ID)
	info, _ := os.Stat(filepath.Join(dir, "account.json"))
	if info.Mode().Perm() != 0600 {
		t.Fatal("metadata permissions")
	}
	other, _ := CreateProviderAccount("owner-a", "Google pribadi")
	otherHome, _ := ProviderAccountHome("owner-a", other.ID)
	if IsAntigravityLoggedInInDir(otherHome) {
		t.Fatal("empty profile inherited another account")
	}
	if _, err := StartProviderAccountAuth(context.Background(), "owner-a", a.ID); err == nil {
		t.Fatal("fresh login would overwrite existing credentials")
	}
}

func TestS26ProfileEnvironmentDoesNotInheritSecrets(t *testing.T) {
	t.Setenv("GOOGLE_API_KEY", "secret-google")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "secret-aws")
	t.Setenv("ANTIGRAVITY_TOKEN_PATH", "/other/account")
	t.Setenv("HOME", "/original/service")
	env := strings.Join(AntigravityProfileEnvironment("/isolated/profile"), "\n")
	for _, bad := range []string{"secret-google", "secret-aws", "/other/account", "HOME=/original/service"} {
		if strings.Contains(env, bad) {
			t.Fatalf("inherited %q", bad)
		}
	}
	if !strings.Contains(env, "HOME=/isolated/profile\n") || !strings.Contains(env, "XDG_CONFIG_HOME=/isolated/profile/.config") {
		t.Fatal("profile environment missing")
	}
	if os.Getenv("HOME") != "/original/service" {
		t.Fatal("global HOME changed")
	}
}

func TestS26ScopedOAuthRejectsOtherOwnerAccountAndLegacy(t *testing.T) {
	id := "s26-scoped-session"
	writer := &s25EnterWriter{done: make(chan struct{})}
	session := &AntigravityAuthSession{ID: id, OwnerID: "owner-a", AccountID: "acct-a", Stdin: writer, Done: make(chan struct{}), CreatedAt: time.Now()}
	authSessionsMu.Lock()
	authSessions[id] = session
	authSessionsMu.Unlock()
	defer CancelAntigravityAuthSession(id)
	if err := submitAntigravityAuthCode(id, "code", "owner-b", "acct-a"); err == nil {
		t.Fatal("other owner accepted")
	}
	if err := submitAntigravityAuthCode(id, "code", "owner-a", "acct-b"); err == nil {
		t.Fatal("other account accepted")
	}
	if err := SubmitAntigravityAuthCode(id, "code"); err == nil {
		t.Fatal("legacy endpoint accepted scoped session")
	}
	if CancelLegacyAntigravityAuthSession(id) {
		t.Fatal("legacy endpoint cancelled account session")
	}
	if err := CancelProviderAccountAuth("owner-b", "acct-a", id); err == nil {
		t.Fatal("other owner cancelled session")
	}
	if len(writer.input) != 0 {
		t.Fatal("untrusted code reached terminal")
	}
	if err := CancelProviderAccountAuth("owner-a", "acct-a", id); err != nil {
		t.Fatal(err)
	}
}

func TestS26RealIsolatedOAuthURL(t *testing.T) {
	if os.Getenv("CUAN_VERIFY_REAL_ACCOUNT_OAUTH") != "1" {
		t.Skip("explicit actual CLI check")
	}
	home := t.TempDir()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	s, err := startAntigravityAuthSession(ctx, home, "real-check-owner", "real-check-account")
	if err != nil {
		t.Fatal(err)
	}
	defer CancelAntigravityAuthSession(s.ID)
	secondHome := t.TempDir()
	second, err := startAntigravityAuthSession(ctx, secondHome, "real-check-owner", "real-check-second-account")
	if err != nil {
		t.Fatal(err)
	}
	defer CancelAntigravityAuthSession(second.ID)
	if !validGoogleAuthURL(s.AuthURL) || !validGoogleAuthURL(second.AuthURL) || s.ID == second.ID || s.ProfileHome == second.ProfileHome {
		t.Fatal("isolated real profiles reused OAuth session")
	}
	parsed, _ := url.Parse(s.AuthURL)
	if !strings.Contains(parsed.Query().Get("prompt"), "select_account") {
		t.Fatal("new-account OAuth does not ask for account choice")
	}
	if IsAntigravityLoggedInInDir(home) || IsAntigravityLoggedInInDir(secondHome) {
		t.Fatal("fresh profile inherited credentials")
	}
	t.Log("Two actual isolated CLI profiles produced official Google OAuth URLs with distinct sessions; URL/session withheld. No consent/code submitted.")
}

func TestS26ScopedExpiryAndProfileCredentialTimestamp(t *testing.T) {
	home := t.TempDir()
	dir := AntigravityConfigDirPath(home)
	os.MkdirAll(dir, 0700)
	path := AntigravityCredentialsFilePath(home)
	os.WriteFile(path, []byte("fixture-credential-never-real"), 0600)
	before := time.Now().Add(-time.Hour)
	os.Chtimes(path, before, before)
	session := &AntigravityAuthSession{ID: "s26-expired", OwnerID: "owner", AccountID: "account", ProfileHome: home, CreatedAt: time.Now(), Done: make(chan struct{})}
	if sessionCredentialsSaved(session) {
		t.Fatal("old credential treated as new exchange")
	}
	now := time.Now()
	os.Chtimes(path, now, now)
	if !sessionCredentialsSaved(session) {
		t.Fatal("fresh profile credential not detected")
	}
	session.CreatedAt = time.Now().Add(-6 * time.Minute)
	authSessionsMu.Lock()
	authSessions[session.ID] = session
	authSessionsMu.Unlock()
	if submitAntigravityAuthCode(session.ID, "code", "owner", "account") == nil {
		t.Fatal("expired account OAuth accepted")
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatal("expiry deleted credential")
	}
}

func TestS26DedicatedProfileBinding(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	a, err := CreateProviderAccount("owner-a", "agent-one")
	if err != nil {
		t.Fatal(err)
	}
	home, _ := ProviderAccountHome("owner-a", a.ID)
	if err := BindProviderAccount("owner-a", a.ID, "agent-one", "conn_profile_first"); err == nil {
		t.Fatal("profile bound without login")
	}
	os.MkdirAll(AntigravityConfigDirPath(home), 0700)
	os.WriteFile(AntigravityCredentialsFilePath(home), []byte("fixture-only-credential-value"), 0600)
	if err := BindProviderAccount("owner-a", a.ID, "agent-one", "conn_profile_first"); err != nil {
		t.Fatal(err)
	}
	if err := BindProviderAccount("owner-a", a.ID, "agent-two", "conn_profile_second"); err == nil {
		t.Fatal("profile shared between agents")
	}
	if _, err := ProviderAccountForConnection("owner-b", "agent-one", "conn_profile_first"); err == nil {
		t.Fatal("other owner resolved binding")
	}
	if _, err := ProviderAccountForConnection("owner-a", "agent-two", "conn_profile_first"); err == nil {
		t.Fatal("other agent borrowed connection")
	}
	saved, err := ProviderAccountForConnection("owner-a", "agent-one", "conn_profile_first")
	if err != nil || saved.ID != a.ID {
		t.Fatal("persisted binding lost")
	}
	UnbindProviderAccount("owner-a", a.ID, "wrong-agent", "conn_profile_first")
	if _, err := ProviderAccountForConnection("owner-a", "agent-one", "conn_profile_first"); err != nil {
		t.Fatal("wrong rollback unbound another agent")
	}
	UnbindProviderAccount("owner-a", a.ID, "agent-one", "conn_profile_first")
	if _, err := ProviderAccountForConnection("owner-a", "agent-one", "conn_profile_first"); err == nil {
		t.Fatal("failed-registration binding survived rollback")
	}
	if !IsAntigravityLoggedInInDir(home) {
		t.Fatal("rollback removed login credential")
	}
}
