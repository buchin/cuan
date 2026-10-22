package agent

import (
	"bytes"
	"context"
	"crypto/sha256"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestCLISetupDriverRequiresConsentAndDisablesInteractionSharing(t *testing.T) {
	var input bytes.Buffer
	d := &cliSetupDriver{writer: &input}
	d.feed("Choose your color scheme:\nterminal\n[Next]")
	if input.String() != "\r" {
		t.Fatal("default theme not selected")
	}
	d.feed("Terms of Service & Data Use\n[x] Yes, I agree to help improve\n[Previous] [Done]")
	if input.String() != "\r" {
		t.Fatal("terms accepted without consent")
	}
	d.accept()
	if input.String() != "\r " {
		t.Fatal("sharing was not deselected first")
	}
	d.feed("\x1b[6G ]")
	d.feed("\r >  Previous Confirm")
	d.feed(" [Previous]   >  Done ")
	if input.String() != "\r \t\t\r" {
		t.Fatalf("unexpected wizard controls %q", input.String())
	}
	before := input.Len()
	d.feed("Terms of Service & Data Use\n[x] Yes, I agree\n[Done]")
	if input.Len() != before {
		t.Fatal("wizard replayed after completion")
	}
}
func TestCLISetupDriverDoesNotToggleAlreadyDisabledSharing(t *testing.T) {
	var input bytes.Buffer
	d := &cliSetupDriver{writer: &input}
	d.accept()
	for _, chunk := range []string{"Terms of Service & Data ", "Use\n[ ] Yes, I ", "agree\n[Previous] [Done]"} {
		d.feed(chunk)
	}
	if input.String() != "\t" {
		t.Fatalf("off choice toggled or unrecognized %q", input.String())
	}
	d.feed("> Previous")
	d.feed("> Done")
	if input.String() != "\t\t\r" {
		t.Fatal("done not selected")
	}
	var unknown bytes.Buffer
	other := &cliSetupDriver{writer: &unknown}
	other.accept()
	other.feed("Terms of Service & Data Use\nEnable new sharing options?\n[Done]")
	if unknown.Len() != 0 {
		t.Fatal("unrecognized consent UI received input")
	}
}

func TestCLISetupDriverHandlesThemeScreenWithoutNextTag(t *testing.T) {
	var input bytes.Buffer
	d := &cliSetupDriver{writer: &input}
	// Simulate mobile screen where [Next] is cut off / below fold
	d.feed("Choose your color scheme:\n> terminal\nlight\ndark")
	if input.String() != "\r" {
		t.Fatalf("expected \\r on color scheme screen, got %q", input.String())
	}
}

func TestEnsureAntigravityOnboarding(t *testing.T) {
	home := t.TempDir()
	// No token -> no-op
	if err := EnsureAntigravityOnboarding(home); err != nil {
		t.Fatal(err)
	}
	if AntigravityOnboardingComplete(home) {
		t.Fatal("expected onboarding not complete without login")
	}

	// Create token
	tokenPath := filepath.Join(AntigravityConfigDirPath(home), "antigravity-oauth-token")
	if err := os.MkdirAll(filepath.Dir(tokenPath), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(tokenPath, []byte("fake-token-with-at-least-16-bytes"), 0600); err != nil {
		t.Fatal(err)
	}

	if err := EnsureAntigravityOnboarding(home); err != nil {
		t.Fatal(err)
	}
	if !AntigravityOnboardingComplete(home) {
		t.Fatal("expected onboarding to be complete after EnsureAntigravityOnboarding")
	}
}
func TestAccountOAuthFinishesWizardBeforeAcceptingSavedCredentials(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	bin := t.TempDir()
	script := `#!/bin/sh
 printf 'https://accounts.google.com/o/oauth2/v2/auth?client_id=fixture&redirect_uri=http://localhost/fixture&response_type=code\n'
 read authcode
 printf '{"token":{"access_token":"fixture-not-a-real-token"}}' > "$ANTIGRAVITY_TOKEN_PATH"
 stty -icanon -echo -icrnl
 printf 'Choose your color scheme:\nterminal\n[Next]'
 key=$(dd bs=1 count=1 2>/dev/null)
 [ "$key" = "$(printf '\r')" ] || exit 31
 printf 'Terms of Service & Data Use\n[x] Yes, I agree to help improve\n[Previous] [Done]'
 key=$(dd bs=1 count=1 2>/dev/null); [ "$key" = ' ' ] || exit 32
 printf '\033[6G ]'
 key=$(dd bs=1 count=1 2>/dev/null); [ "$key" = "$(printf '\t')" ] || exit 33
 printf '> Previous Confirm'
 key=$(dd bs=1 count=1 2>/dev/null); [ "$key" = "$(printf '\t')" ] || exit 34
 printf '[Previous] > Done'
 key=$(dd bs=1 count=1 2>/dev/null); [ "$key" = "$(printf '\r')" ] || exit 35
 mkdir -p "$ANTIGRAVITY_CONFIG_DIR/cache"
 printf '{"onboardingComplete":true}' > "$ANTIGRAVITY_CONFIG_DIR/cache/onboarding.json"
 sleep 3
 `
	if err := os.WriteFile(filepath.Join(bin, "agy"), []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+":"+os.Getenv("PATH"))
	a, err := CreateProviderAccount("fixture-owner", "Cuan")
	if err != nil {
		t.Fatal(err)
	}
	session, err := StartProviderAccountAuth(context.Background(), "fixture-owner", a.ID)
	if err != nil {
		t.Fatal(err)
	}
	defer CancelAntigravityAuthSession(session.ID)
	if err = SubmitProviderAccountAuth("fixture-owner", a.ID, session.ID, "fixture-code", true); err != nil {
		home, _ := ProviderAccountHome("fixture-owner", a.ID)
		info, statErr := os.Stat(AntigravityCredentialsFilePath(home))
		if statErr == nil {
			t.Fatalf("%v; saved=%t complete=%t mtimeDelta=%v", err, IsAntigravityLoggedInInDir(home), antigravityOnboardingComplete(home), info.ModTime().Sub(session.CreatedAt))
		}
		t.Fatal(err, statErr)
	}
	home, err := ProviderAccountHome("fixture-owner", a.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !antigravityOnboardingComplete(home) || !IsAntigravityLoggedInInDir(home) {
		t.Fatal("wizard or credentials not persisted")
	}
	if strings.Contains(home, "fixture-code") {
		t.Fatal("authorization material leaked into paths")
	}
}

func TestCredentialBaselineRejectsOldFilesWithoutRelyingOnMtime(t *testing.T) {
	home := t.TempDir()
	dir := AntigravityConfigDirPath(home)
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	path := AntigravityCredentialsFilePath(home)
	old := []byte(`{"token":"old-fixture-credential"}`)
	if err := os.WriteFile(path, old, 0600); err != nil {
		t.Fatal(err)
	}
	future := time.Now().Add(time.Minute)
	if err := os.Chtimes(path, future, future); err != nil {
		t.Fatal(err)
	}
	s := &AntigravityAuthSession{ProfileHome: home, CreatedAt: time.Now(), credentialBaseline: map[string][32]byte{path: sha256.Sum256(old)}}
	if sessionCredentialsSaved(s) {
		t.Fatal("old file accepted because mtime is newer")
	}
	if err := os.WriteFile(path, []byte(`{"token":"new-fixture-credential"}`), 0600); err != nil {
		t.Fatal(err)
	}
	past := time.Now().Add(-time.Minute)
	if err := os.Chtimes(path, past, past); err != nil {
		t.Fatal(err)
	}
	if !sessionCredentialsSaved(s) {
		t.Fatal("new credential rejected due to filesystem timestamp precision")
	}
	if err := os.WriteFile(path, []byte(`{}`), 0600); err != nil {
		t.Fatal(err)
	}
	if sessionCredentialsSaved(s) {
		t.Fatal("invalid changed credentials accepted")
	}
}

func TestSavedAccountSetupRecoveryPreservesCredentials(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	bin := t.TempDir()
	script := `#!/bin/sh
stty -icanon -echo -icrnl
[ "$(stty size)" = "40 120" ] || exit 41
printf 'Choose your color scheme:\nterminal\n[Next]'
key=$(dd bs=1 count=1 2>/dev/null)
[ "$key" = "$(printf '\r')" ] || exit 42
printf 'Terms of Service & Data Use\n[ ] Yes, I agree to help improve\n[Previous] [Done]'
key=$(dd bs=1 count=1 2>/dev/null); [ "$key" = "$(printf '\t')" ] || exit 43
printf '> Previous'
key=$(dd bs=1 count=1 2>/dev/null); [ "$key" = "$(printf '\t')" ] || exit 44
printf '> Done'
key=$(dd bs=1 count=1 2>/dev/null); [ "$key" = "$(printf '\r')" ] || exit 45
mkdir -p "$ANTIGRAVITY_CONFIG_DIR/cache"
printf '{"onboardingComplete":true}' > "$ANTIGRAVITY_CONFIG_DIR/cache/onboarding.json"
sleep 1
`
	os.WriteFile(filepath.Join(bin, "agy"), []byte(script), 0700)
	t.Setenv("PATH", bin+":"+os.Getenv("PATH"))
	a, err := CreateProviderAccount("owner-setup", "Dev2")
	if err != nil {
		t.Fatal(err)
	}
	home, _ := ProviderAccountHome("owner-setup", a.ID)
	os.MkdirAll(AntigravityConfigDirPath(home), 0700)
	credential := []byte(`{"token":{"access_token":"fixture-saved-not-real"}}`)
	os.WriteFile(AntigravityCredentialsFilePath(home), credential, 0600)
	if err := FinishProviderAccountSetup(context.Background(), "owner-setup", a.ID, false); err == nil {
		t.Fatal("missing consent accepted")
	}
	if antigravityOnboardingComplete(home) {
		t.Fatal("consent rejection changed setup")
	}
	if err := FinishProviderAccountSetup(context.Background(), "other-owner", a.ID, true); err == nil {
		t.Fatal("foreign owner accepted")
	}
	if err := FinishProviderAccountSetup(context.Background(), "owner-setup", a.ID, true); err != nil {
		t.Fatal(err)
	}
	if !antigravityOnboardingComplete(home) {
		t.Fatal("provider did not finish setup")
	}
	after, _ := os.ReadFile(AntigravityCredentialsFilePath(home))
	if string(after) != string(credential) {
		t.Fatal("saved sign-in replaced")
	}
	if err := FinishProviderAccountSetup(context.Background(), "owner-setup", a.ID, true); err != nil {
		t.Fatal("repeat completion failed", err)
	}
}
