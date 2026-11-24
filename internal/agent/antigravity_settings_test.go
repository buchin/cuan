package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
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

func TestChatProfileReadDenyPreservesOwnerRulesAndNativeState(t *testing.T) {
	home := t.TempDir()
	dir := AntigravityConfigDirPath(home)
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "settings.json")
	initial := `{"permissions":{"allow":["read_file(*)","mcp(custom/tool)"],"ask":["command(*)"],"deny":["write_file(private/)"],"future_policy":{"enabled":true}},"selectedModel":"owner-model","interactionSharing":false}`
	if err := os.WriteFile(path, []byte(initial), 0600); err != nil {
		t.Fatal(err)
	}
	auth := filepath.Join(dir, "auth.json")
	if err := os.WriteFile(auth, []byte("fixture-native-state"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := PrepareAntigravityReadOnlyPermissions(home); err != nil {
		t.Fatal(err)
	}
	first, _ := os.ReadFile(path)
	var settings struct {
		Permissions struct {
			Allow, Ask, Deny []string
			Future           json.RawMessage `json:"future_policy"`
		}
		Model   string `json:"selectedModel"`
		Sharing bool   `json:"interactionSharing"`
	}
	if err := json.Unmarshal(first, &settings); err != nil {
		t.Fatal(err)
	}
	if settings.Model != "owner-model" || settings.Sharing || len(settings.Permissions.Future) == 0 || !reflect.DeepEqual(settings.Permissions.Ask, []string{"command(*)"}) {
		t.Fatal("owner preferences changed")
	}
	rules := map[string]bool{}
	for _, rule := range settings.Permissions.Deny {
		rules[rule] = true
	}
	for _, dir := range []string{".gemini", ".cuan", ".ssh", ".claude", ".codex"} {
		if !rules["read_file("+filepath.Join(home, dir)+")"] {
			t.Fatal("profile tools remain readable", dir)
		}
	}
	if !rules["write_file(private/)"] {
		t.Fatal("owner deny rule lost")
	}
	if err := PrepareAntigravityReadOnlyPermissions(home); err != nil {
		t.Fatal(err)
	}
	second, _ := os.ReadFile(path)
	if string(first) != string(second) {
		t.Fatal("permission update is not idempotent")
	}
	state, _ := os.ReadFile(auth)
	if string(state) != "fixture-native-state" {
		t.Fatal("native authentication state changed")
	}
}

func TestChatProfileReadDenyRejectsMalformedRulesWithoutOverwriting(t *testing.T) {
	for _, raw := range []string{`{"permissions":[]}`, `{"permissions":{"deny":"read_file(*)"}}`, `{"permissions":{"allow":[123]}}`, `{"permissions":{"ask":{}}}`} {
		home := t.TempDir()
		dir := AntigravityConfigDirPath(home)
		if err := os.MkdirAll(dir, 0700); err != nil {
			t.Fatal(err)
		}
		path := filepath.Join(dir, "settings.json")
		if err := os.WriteFile(path, []byte(raw), 0600); err != nil {
			t.Fatal(err)
		}
		if err := PrepareAntigravityReadOnlyPermissions(home); err == nil {
			t.Fatal("malformed rule accepted", raw)
		}
		unchanged, _ := os.ReadFile(path)
		if string(unchanged) != raw {
			t.Fatal("malformed owner rules overwritten")
		}
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
		"search_web(*)", "web_fetch(*)", "call_mcp_tool(*)",
		"mcp(*)", "mcp(cuan-inspect/*)", "mcp(cuan-inspect:*)", "mcp(cuan-platform-mcp/*)",
		"mcp(cuan-inspect/inspect_git_log)", "mcp(cuan-inspect/inspect_git_status)",
		"inspect_database_query(*)", "inspect_database_schema(*)", "inspect_app_logs(*)",
		"list_projects(*)", "list_tasks(*)", "list_inbox_conversations(*)", "list_contacts(*)", "list_scheduled_jobs(*)", "get_system_settings(*)",
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

func TestCLISettingsEnsureWorkspaceMCPConfig(t *testing.T) {
	ws := t.TempDir()
	if err := EnsureWorkspaceMCPConfig(ws); err != nil {
		t.Fatal(err)
	}
	for _, sub := range []string{
		filepath.Join(ws, ".gemini", "config", "mcp_config.json"),
		filepath.Join(ws, ".agents", "mcp_config.json"),
	} {
		data, err := os.ReadFile(sub)
		if err != nil {
			t.Fatalf("failed to read workspace mcp_config.json at %s: %v", sub, err)
		}
		var cfg struct {
			MCPServers map[string]struct {
				Command string   `json:"command"`
				Args    []string `json:"args"`
			} `json:"mcpServers"`
		}
		if err := json.Unmarshal(data, &cfg); err != nil {
			t.Fatalf("invalid json: %v", err)
		}
		cuanServer, ok := cfg.MCPServers["cuan-inspect"]
		if !ok || cuanServer.Command != "cuan" || len(cuanServer.Args) == 0 || cuanServer.Args[0] != "mcp" {
			t.Fatalf("expected cuan-inspect in %s, got: %+v", sub, cfg.MCPServers)
		}
	}
}

func TestCLISettingsTaskExecutionClearsToolDenials(t *testing.T) {
	home := t.TempDir()
	ws := t.TempDir()

	// 1. Simulate legacy deny rules written to settings.json
	dir := AntigravityConfigDirPath(home)
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(dir, "settings.json")
	sensitiveRule := "read_file(" + filepath.Join(home, ".gemini") + ")"
	initial := fmt.Sprintf(`{"permissions":{"deny":["%s","command(*)","write_file(*)","edit_file(*)"]}}`, sensitiveRule)
	if err := os.WriteFile(p, []byte(initial), 0600); err != nil {
		t.Fatal(err)
	}

	raw, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	var settings map[string]any
	if err := json.Unmarshal(raw, &settings); err != nil {
		t.Fatal(err)
	}
	perms, _ := settings["permissions"].(map[string]any)
	denied, _ := perms["deny"].([]any)
	hasCommandDeny := false
	for _, r := range denied {
		if r == "command(*)" || r == "write_file(*)" {
			hasCommandDeny = true
			break
		}
	}
	if !hasCommandDeny {
		t.Fatalf("expected legacy permissions to deny command(*) or write_file(*), got: %v", denied)
	}

	// 2. Prepare settings for task runner with trusted workspace
	if err := PrepareAntigravitySettings(home, ws); err != nil {
		t.Fatal(err)
	}

	raw, err = os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &settings); err != nil {
		t.Fatal(err)
	}
	perms, _ = settings["permissions"].(map[string]any)
	denied, _ = perms["deny"].([]any)
	for _, r := range denied {
		ruleStr, _ := r.(string)
		if ruleStr == "command(*)" || ruleStr == "write_file(*)" || ruleStr == "edit_file(*)" || strings.HasPrefix(ruleStr, "write_file(") || strings.HasPrefix(ruleStr, "command(") {
			t.Fatalf("task execution settings must not retain tool denial rule: %s in %v", ruleStr, denied)
		}
	}

	// Verify that profile sensitive path denials (.gemini, .cuan, .ssh) were still preserved
	hasPathDeny := false
	for _, r := range denied {
		ruleStr, _ := r.(string)
		if strings.Contains(ruleStr, ".gemini") || strings.Contains(ruleStr, ".ssh") {
			hasPathDeny = true
			break
		}
	}
	if !hasPathDeny {
		t.Fatalf("expected profile sensitive path denials to be preserved, got: %v", denied)
	}
}
