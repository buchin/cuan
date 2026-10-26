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

func TestCLISettingsPrepareTrustedWorkspaces(t *testing.T) {
	home := t.TempDir()
	ws1 := "/opt/cuan/data/workspaces/ws1"
	ws2 := "/opt/cuan/data/workspaces/ws2"

	if err := PrepareAntigravitySettings(home, ws1); err != nil {
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
	if settings["colorScheme"] != "terminal" || settings["enableTelemetry"] != false {
		t.Fatalf("unexpected defaults: %v", settings)
	}
	trusted, ok := settings["trustedWorkspaces"].([]any)
	if !ok || len(trusted) != 1 || trusted[0] != ws1 {
		t.Fatalf("expected trustedWorkspaces [%s], got %v", ws1, settings["trustedWorkspaces"])
	}

	// Adding duplicate ws1 and new ws2
	if err := PrepareAntigravitySettings(home, ws1, ws2); err != nil {
		t.Fatal(err)
	}
	raw, err = os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(raw, &settings); err != nil {
		t.Fatal(err)
	}
	trusted, ok = settings["trustedWorkspaces"].([]any)
	if !ok || len(trusted) != 2 || trusted[0] != ws1 || trusted[1] != ws2 {
		t.Fatalf("expected trustedWorkspaces [%s, %s], got %v", ws1, ws2, settings["trustedWorkspaces"])
	}
}

func TestCLISettingsPrepareReadOnlyPermissions(t *testing.T) {
	home := t.TempDir()
	if err := PrepareAntigravityReadOnlyPermissions(home); err != nil {
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
	perms, ok := settings["permissions"].(map[string]any)
	if !ok {
		t.Fatalf("expected permissions object, got %v", settings["permissions"])
	}
	allow, ok := perms["allow"].([]any)
	if !ok || len(allow) < 7 {
		t.Fatalf("expected allow list with at least 7 read-only tool rules, got %v", perms["allow"])
	}
	rules := map[string]bool{}
	for _, rule := range allow {
		if s, ok := rule.(string); ok {
			rules[s] = true
		}
	}
	for _, expected := range []string{
		"read_file(*)", "view_file(*)", "list_dir(*)", "grep_search(*)", "find_by_name(*)",
		"inspect_git_status(*)", "inspect_git_log(*)", "inspect_system_health(*)", "inspect_typecheck(*)",
	} {
		if !rules[expected] {
			t.Fatalf("missing expected permission rule %s in %v", expected, rules)
		}
	}

	// Idempotent: calling again does not duplicate rules
	if err := PrepareAntigravityReadOnlyPermissions(home); err != nil {
		t.Fatal(err)
	}
	raw2, _ := os.ReadFile(p)
	var settings2 map[string]any
	_ = json.Unmarshal(raw2, &settings2)
	perms2 := settings2["permissions"].(map[string]any)
	allow2 := perms2["allow"].([]any)
	if len(allow2) != len(allow) {
		t.Fatalf("expected idempotent permissions, got len %d vs %d", len(allow2), len(allow))
	}
}

func TestCLISettingsEnsureAntigravityMCPConfig(t *testing.T) {
	home := t.TempDir()
	if err := EnsureAntigravityMCPConfig(home); err != nil {
		t.Fatal(err)
	}
	mcpPath := filepath.Join(home, ".gemini", "config", "mcp_config.json")
	data, err := os.ReadFile(mcpPath)
	if err != nil {
		t.Fatalf("failed to read mcp_config.json: %v", err)
	}
	var cfg struct {
		MCPServers map[string]struct {
			Command string   `json:"command"`
			Args    []string `json:"args"`
		} `json:"mcpServers"`
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		t.Fatalf("invalid json in mcp_config.json: %v", err)
	}
	cuanServer, ok := cfg.MCPServers["cuan-inspect"]
	if !ok || cuanServer.Command != "cuan" || len(cuanServer.Args) == 0 || cuanServer.Args[0] != "mcp" {
		t.Fatalf("expected cuan-inspect mcp server in mcp_config.json, got: %+v", cfg.MCPServers)
	}
}
