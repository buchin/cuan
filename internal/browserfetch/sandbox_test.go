package browserfetch

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestBrowserFilesystemSandboxHidesHostFilesAndProfiles(t *testing.T) {
	sandbox, err := exec.LookPath("bwrap")
	if err != nil {
		t.Skip("bubblewrap unavailable; required before browser enablement")
	}
	root := t.TempDir()
	secret := filepath.Join(root, "provider-auth")
	if err := os.WriteFile(secret, []byte("HOST_PRIVATE_SENTINEL"), 0600); err != nil {
		t.Fatal(err)
	}
	binary := filepath.Join(root, "engine")
	// Script is fixture-controlled. The real engine receives no script/eval input.
	script := "#!/bin/sh\n[ ! -e '" + secret + "' ] || exit 71\n[ -z \"$CUAN_AUTH_TOKEN\" ] || exit 72\n[ \"$HOME\" = /home/browser ] || exit 73\n[ \"$OBSCURA_ALLOW_PRIVATE_NETWORK\" = 0 ] || exit 74\n[ ! -e /proc/self/environ ] || exit 75\nif touch /usr/bin/cuan-browser-write-probe 2>/dev/null; then exit 76; fi\nprintf fixture > /home/browser/result\nprintf BROWSER_BOUNDARY_OK\n"
	if err := os.WriteFile(binary, []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("CUAN_AUTH_TOKEN", "HOST_PRIVATE_SENTINEL")
	r := Runtime{Binary: binary, LockFile: filepath.Join(root, "browser.lock"), Sandbox: sandbox}
	result, err := r.Fetch(context.Background(), "https://example.invalid/")
	if err != nil || result.Content != "BROWSER_BOUNDARY_OK" {
		command, args, _ := r.sandboxCommand(t.TempDir(), []string{"fetch", "https://example.invalid/"})
		debug := exec.Command(command, args...)
		debug.Env = []string{"PATH=/usr/bin:/bin"}
		detail, debugErr := debug.CombinedOutput()
		t.Fatal("real browser namespace failed", result, err, debugErr, string(detail))
	}
	raw, err := os.ReadFile(secret)
	if err != nil || string(raw) != "HOST_PRIVATE_SENTINEL" {
		t.Fatal("host file changed", err)
	}
	argsRoot := t.TempDir()
	_, args, err := r.sandboxCommand(argsRoot, []string{"fetch", "https://example.invalid/"})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(strings.Join(args, " "), "--ro-bind /home ") || strings.Contains(strings.Join(args, " "), "--ro-bind /usr/local ") {
		t.Fatal("broad host mount", args)
	}
}

func TestBrowserNeverFallsBackWithoutFilesystemSandbox(t *testing.T) {
	r := fixtureRuntime(t, "printf SHOULD_NOT_RUN\n")
	r.Sandbox = ""
	if result, err := r.Fetch(context.Background(), "https://example.invalid/"); err == nil || result != nil {
		t.Fatal("unisolated fallback", result, err)
	}
}
