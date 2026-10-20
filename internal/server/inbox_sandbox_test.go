package server

import (
	"context"
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
	for path, body := range map[string]string{filepath.Join(workspace, "README"): "project", hostSecret: "private", filepath.Join(profile, "auth"): "fixture-token"} {
		if err := os.WriteFile(path, []byte(body), 0600); err != nil {
			t.Fatal(err)
		}
	}
	args, err := inboxSandboxArgs("/bin/sh", home, workspace, []string{attachment})
	if err != nil {
		t.Fatal(err)
	}
	script := `set -eu
test "$(cat README)" = project
test ! -e "$1"
test ! -e /proc/1/environ
test ! -e /opt/cuan/.env
test "$(cat "$3")" = attached
if (echo bad > README) 2>/dev/null; then exit 40; fi
if (touch new-file) 2>/dev/null; then exit 41; fi
echo temporary > /tmp/scratch
echo refreshed > "$2"
echo BOUNDARY_OK
`
	args = append(args, "--", "/bin/sh", "-c", script, "sandbox", hostSecret, filepath.Join(profile, "auth"), attachment)
	out, err := exec.Command(bwrap, args...).CombinedOutput()
	if err != nil {
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
	if strings.TrimSpace(string(body)) != "refreshed" {
		t.Fatal("token refresh state not retained")
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
