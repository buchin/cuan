package server

import (
	"context"
	"fmt"
	"github.com/buchin/cuan/internal/config"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// Existing provider protocol fixtures deliberately bypass filesystem isolation;
// the separate real-bwrap tests below verify the OS boundary without inference.
func installInboxSandboxStub(t *testing.T, bin string) {
	t.Helper()
	script := "#!/bin/sh\nwhile [ \"$#\" -gt 0 ]; do if [ \"$1\" = -- ]; then shift; exec \"$@\"; fi; shift; done\nexit 99\n"
	if err := os.WriteFile(filepath.Join(bin, "bwrap"), []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
}

func TestInboxExternalSandboxBoundary(t *testing.T) {
	bwrap, err := exec.LookPath("bwrap")
	if err != nil {
		t.Skip("bubblewrap unavailable; deployment gate must run this test")
	}
	root := t.TempDir()
	home := filepath.Join(root, "account")
	workspace := filepath.Join(root, "project")
	profile := filepath.Join(home, ".gemini", "antigravity-cli")
	for _, dir := range []string{workspace, profile} {
		if err := os.MkdirAll(dir, 0700); err != nil {
			t.Fatal(err)
		}
	}
	hostSecret := filepath.Join(root, "host-secret")
	attachment := filepath.Join(root, "upload.txt")
	if err := os.WriteFile(attachment, []byte("attached"), 0600); err != nil {
		t.Fatal(err)
	}
	for path, body := range map[string]string{filepath.Join(workspace, "README"): "project", hostSecret: "private", filepath.Join(profile, "auth"): "fixture-token", filepath.Join(profile, "mcp_config.json"): `{"mcpServers":{"ambient-owner":{"command":"private"}}}`} {
		if err := os.WriteFile(path, []byte(body), 0600); err != nil {
			t.Fatal(err)
		}
	}
	// BusyBox selects applets by argv[0], so /proc/self/exe cannot act as a
	// shell helper there. Use dash when available, including the Alpine gate.
	shell := "/bin/sh"
	if dash, err := exec.LookPath("dash"); err == nil {
		shell = dash
	}
	args, err := inboxSandboxArgs(shell, home, workspace, []string{attachment})
	if err != nil {
		t.Fatal(err)
	}
	script := `set -eu
test "$(cat README)" = project
test ! -e "$1"
test -r /proc/self/stat
test -e /proc/self/exe
/proc/self/exe -c 'echo TERMINAL_HELPER_OK'
test ! -e /proc/"$4"/environ
test ! -e /opt/cuan/.env
test "$(cat "$3")" = attached
if (echo bad > README) 2>/dev/null; then exit 40; fi
if (touch new-file) 2>/dev/null; then exit 41; fi
test "$(cat "$5")" = '{"mcpServers":{"cuan-inspect":{"command":"/cuan-public-web-mcp","args":["mcp","--public-web"]}}}'
test "$(cat "$6")" = '{"mcpServers":{"cuan-inspect":{"command":"/cuan-public-web-mcp","args":["mcp","--public-web"]}}}'
echo temporary > /tmp/scratch
echo refreshed > "$2"
echo BOUNDARY_OK
`
	args = append(args, "--", shell, "-c", script, "sandbox", hostSecret, filepath.Join(profile, "auth"), attachment, fmt.Sprint(os.Getpid()), filepath.Join(home, ".gemini", "config", "mcp_config.json"), filepath.Join(profile, "mcp_config.json"))
	out, err := exec.Command(bwrap, args...).CombinedOutput()
	if err != nil {
		if strings.Contains(string(out), "Can't mount proc") {
			t.Skip("outer test runtime blocks nested proc mount; real service-user test required")
		}
		t.Fatalf("sandbox boundary failed: %v %s", err, out)
	}
	if !strings.Contains(string(out), "BOUNDARY_OK") {
		t.Fatal("missing sandbox evidence")
	}
	body, _ := os.ReadFile(filepath.Join(workspace, "README"))
	if string(body) != "project" {
		t.Fatal("workspace modified")
	}
	body, _ = os.ReadFile(filepath.Join(profile, "auth"))
	mcpBody, _ := os.ReadFile(filepath.Join(profile, "mcp_config.json"))
	if !strings.Contains(string(mcpBody), "ambient-owner") {
		t.Fatal("host MCP config changed")
	}
	if strings.TrimSpace(string(body)) != "refreshed" {
		t.Fatal("token refresh state not retained")
	}
}

func TestInboxExternalSandboxMountsUserLocalBin(t *testing.T) {
	home := t.TempDir()
	userBin := filepath.Join(home, ".local", "bin")
	if err := os.MkdirAll(userBin, 0700); err != nil {
		t.Fatal(err)
	}
	args, err := inboxSandboxArgs("/bin/sh", home, "", nil)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for i := 0; i < len(args)-2; i++ {
		if args[i] == "--ro-bind" && args[i+1] == userBin && args[i+2] == userBin {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected --ro-bind %s %s in sandbox args, got: %v", userBin, userBin, args)
	}
}

func TestInboxExternalSandboxRejectsBroadWorkspace(t *testing.T) {
	home := t.TempDir()
	for _, workspace := range []string{"/", "/etc", home} {
		if _, err := inboxSandboxArgs("/bin/sh", home, workspace, nil); err == nil {
			t.Fatalf("accepted broad workspace %s", workspace)
		}
	}
}

func TestInboxExternalSandboxMissingFailsClosed(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	if _, _, err := sandboxInboxCommand("/bin/sh", nil, nil, t.TempDir(), "", nil); err == nil {
		t.Fatal("missing sandbox accepted")
	}
}

func TestInboxExternalSandboxCLIHelp(t *testing.T) {
	binary := os.Getenv("CUAN_SANDBOX_CLI")
	if binary == "" {
		t.Skip("explicit installed CLI help check only")
	}
	home := t.TempDir()
	path, args, err := sandboxInboxCommand(binary, []string{"--help"}, []string{"HOME=" + home, "PATH=/usr/bin:/bin"}, home, "", nil)
	if err != nil {
		t.Fatal(err)
	}
	out, err := exec.Command(path, args...).CombinedOutput()
	if err != nil && strings.Contains(string(out), "Can't mount proc") {
		t.Skip("outer test runtime blocks nested proc mount; real service-user CLI check required")
	}
	if err != nil || !strings.Contains(string(out), "--dangerously-skip-permissions") {
		t.Fatalf("sandbox CLI startup failed: %v %s", err, out)
	}
}

func TestInboxExternalSandboxBlockedBeforeProvider(t *testing.T) {
	home, bin := t.TempDir(), t.TempDir()
	for name, script := range map[string]string{
		"bwrap": "#!/bin/sh\nexit 1\n",
		"agy":   "#!/bin/sh\ntouch \"$HOME/provider-started\"\n",
	} {
		if err := os.WriteFile(filepath.Join(bin, name), []byte(script), 0700); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("PATH", bin+":/usr/bin:/bin")
	s := &Server{cfg: &config.Config{Environment: "staging"}}
	out, failure := s.executeInboxCLI(context.Background(), "fixture", "agy", "Inspect README", home, t.TempDir(), nil)
	if out != "" || !strings.Contains(failure, "sandbox is unavailable") {
		t.Fatal(out, failure)
	}
	if _, err := os.Stat(filepath.Join(home, "provider-started")); !os.IsNotExist(err) {
		t.Fatal("provider launched before sandbox check")
	}
}

func TestInboxSandboxMasksMCPConnectionsWithoutChangingProfile(t *testing.T) {
	home, workspace := t.TempDir(), t.TempDir()
	targets := []string{filepath.Join(home, ".gemini", "config", "mcp_config.json"), filepath.Join(home, ".gemini", "antigravity-cli", "mcp_config.json"), filepath.Join(workspace, ".agents", "mcp_config.json")}
	const original = `{"mcpServers":{"cuan-inspect":{"command":"cuan","args":["mcp"]},"other-owner":{"command":"private"}}}`
	for _, target := range targets {
		if err := os.MkdirAll(filepath.Dir(target), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(target, []byte(original), 0600); err != nil {
			t.Fatal(err)
		}
	}
	args, err := inboxSandboxArgs("/bin/sh", home, workspace, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, target := range targets {
		masked := false
		for i := 0; i+2 < len(args); i++ {
			if args[i] == "--ro-bind" && args[i+2] == target && args[i+1] != target {
				raw, err := os.ReadFile(args[i+1])
				if err != nil || string(raw) != inboxPublicWebMCPConfig {
					t.Fatal("invalid public web MCP boundary")
				}
				masked = true
			}
		}
		if !masked {
			t.Fatal("account/workspace MCP inherited", target)
		}
		raw, _ := os.ReadFile(target)
		if string(raw) != original {
			t.Fatal("host profile changed")
		}
	}
}

func TestInboxMCPBoundaryRejectsChangedOrSymlinkedConfig(t *testing.T) {
	for _, scenario := range []string{"changed", "symlink"} {
		t.Run(scenario, func(t *testing.T) {
			home := t.TempDir()
			file, err := prepareInboxMCPBoundary(home)
			if err != nil {
				t.Fatal(err)
			}
			if scenario == "changed" {
				err = os.WriteFile(file, []byte(`{"mcpServers":{"unexpected":{}}}`), 0600)
			} else {
				if err = os.Remove(file); err == nil {
					err = os.Symlink(filepath.Join(t.TempDir(), "missing"), file)
				}
			}
			if err != nil {
				t.Fatal(err)
			}
			if _, err := prepareInboxMCPBoundary(home); err == nil {
				t.Fatal("changed MCP boundary accepted")
			}
		})
	}
}

func TestInboxPublicWebBoundaryMigratesWithoutReplacingLegacyConfig(t *testing.T) {
	home := t.TempDir()
	legacy := filepath.Join(home, ".cuan-chat-sandbox", "mcp_config.json")
	if err := os.MkdirAll(filepath.Dir(legacy), 0700); err != nil {
		t.Fatal(err)
	}
	const old = "{\"mcpServers\":{}}\n"
	if err := os.WriteFile(legacy, []byte(old), 0600); err != nil {
		t.Fatal(err)
	}
	path, err := prepareInboxMCPBoundary(home)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(path)
	if err != nil || string(raw) != inboxPublicWebMCPConfig {
		t.Fatal("new capability boundary unavailable", err)
	}
	raw, err = os.ReadFile(legacy)
	if err != nil || string(raw) != old {
		t.Fatal("legacy boundary changed", err)
	}
}
