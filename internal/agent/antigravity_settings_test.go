package agent

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestCLISettingsPrepareThemeWithoutAcceptingProviderTerms(t *testing.T) {
	home := t.TempDir()
	if err := prepareAntigravitySettings(home); err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(AntigravityConfigDirPath(home), "settings.json")
	raw, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	var settings map[string]any
	if err = json.Unmarshal(raw, &settings); err != nil {
		t.Fatal(err)
	}
	if len(settings) != 2 || settings["colorScheme"] != "terminal" || settings["enableTelemetry"] != false {
		t.Fatal("unexpected defaults or consent properties", settings)
	}
	info, _ := os.Stat(p)
	if info.Mode().Perm() != 0600 {
		t.Fatal("settings not private")
	}
	original := `{"colorScheme":"dark","enableTelemetry":true,"editorMode":"vim","futureSetting":{"retain":true}}`
	if err = os.WriteFile(p, []byte(original), 0600); err != nil {
		t.Fatal(err)
	}
	if err = prepareAntigravitySettings(home); err != nil {
		t.Fatal(err)
	}
	after, _ := os.ReadFile(p)
	if string(after) != original {
		t.Fatal("explicit provider preferences rewritten")
	}
	if err = os.WriteFile(p, []byte("broken existing settings"), 0600); err != nil {
		t.Fatal(err)
	}
	if err = prepareAntigravitySettings(home); err == nil {
		t.Fatal("malformed settings overwritten")
	}
	after, _ = os.ReadFile(p)
	if string(after) != "broken existing settings" {
		t.Fatal("malformed file lost")
	}
}

func TestCLISettingsRejectsSettingsSymlink(t *testing.T) {
	home := t.TempDir()
	dir := AntigravityConfigDirPath(home)
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(t.TempDir(), "keep.json")
	if err := os.WriteFile(outside, []byte(`{"keep":true}`), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(dir, "settings.json")); err != nil {
		t.Fatal(err)
	}
	if err := prepareAntigravitySettings(home); err == nil {
		t.Fatal("linked settings accepted")
	}
	got, _ := os.ReadFile(outside)
	if string(got) != `{"keep":true}` {
		t.Fatal("unrelated settings changed")
	}
}

func TestProfileLoginPreparesSettingsBeforeLaunchingCLI(t *testing.T) {
	home := t.TempDir()
	bin := t.TempDir()
	script := `#!/bin/sh
 test -f "$ANTIGRAVITY_CONFIG_DIR/settings.json" || exit 31
 grep -q '"colorScheme": "terminal"' "$ANTIGRAVITY_CONFIG_DIR/settings.json" || exit 32
 grep -q '"enableTelemetry": false' "$ANTIGRAVITY_CONFIG_DIR/settings.json" || exit 33
 printf 'https://accounts.google.com/o/oauth2/v2/auth?client_id=fixture&redirect_uri=http://localhost/fixture&response_type=code\n'
 read ignored
 `
	if err := os.WriteFile(filepath.Join(bin, "agy"), []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+":"+os.Getenv("PATH"))
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	session, err := startAntigravityAuthSession(ctx, home, "fixture-owner", "fixture-account")
	if err != nil {
		t.Fatal(err)
	}
	defer CancelAntigravityAuthSession(session.ID)
	if !validGoogleAuthURL(session.AuthURL) {
		t.Fatal("login failed to reach Google URL with prepared settings")
	}
	if IsAntigravityLoggedInInDir(home) {
		t.Fatal("settings initialization fabricated credentials")
	}
}
