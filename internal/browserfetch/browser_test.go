package browserfetch

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

func fixtureRuntime(t *testing.T, script string) Runtime {
	t.Helper()
	root := t.TempDir()
	binary := filepath.Join(root, "cuan-obscura")
	if err := os.WriteFile(binary, []byte("#!/bin/sh\n"+script), 0700); err != nil {
		t.Fatal(err)
	}
	sandbox := filepath.Join(root, "sandbox-stub")
	// Lifecycle fixtures bypass namespaces; real namespace tests are separate.
	stub := `#!/bin/sh
engine=''
while [ "$#" -gt 0 ]; do
 case "$1" in
 --ro-bind) [ "$3" != /cuan-obscura ] || engine="$2"; shift 3 ;;
 --) shift; shift; exec "$engine" "$@" ;;
 *) shift ;;
 esac
done
exit 99
`
	if err := os.WriteFile(sandbox, []byte(stub), 0700); err != nil {
		t.Fatal(err)
	}
	return Runtime{Binary: binary, LockFile: filepath.Join(root, "browser.lock"), Sandbox: sandbox}
}

func TestBrowserInspectionIsolatesEnvironmentAndCleansStorage(t *testing.T) {
	t.Setenv("CUAN_AUTH_TOKEN", "must-not-inherit")
	t.Setenv("HTTP_PROXY", "must-not-inherit")
	t.Setenv("OBSCURA_ALLOW_PRIVATE_NETWORK", "1")
	r := fixtureRuntime(t, `[ -z "$CUAN_AUTH_TOKEN" ] || exit 81
[ -z "$HTTP_PROXY" ] || exit 82
[ "$OBSCURA_ALLOW_PRIVATE_NETWORK" = 0 ] || exit 83
[ "$1" = fetch ] && [ "$2" = 'https://example.invalid/?q=literal;echo' ] || exit 84
printf '%s' "$HOME"
mkdir -p "$HOME/cache"
printf temporary > "$HOME/cache/item"
`)
	result, err := r.Fetch(context.Background(), "https://example.invalid/?q=literal;echo")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result.Content, "cuan-browser-inspection-") {
		t.Fatal("HOME was not isolated", result)
	}
	if _, err := os.Stat(result.Content); !os.IsNotExist(err) {
		t.Fatal("temporary data retained", err)
	}
}

func TestBrowserInspectionBoundsOutputAndSharesSlot(t *testing.T) {
	r := fixtureRuntime(t, "head -c 100000 /dev/zero | tr '\\000' x\n")
	result, err := r.Fetch(context.Background(), "https://example.invalid/")
	if err != nil || !result.Truncated || len(result.Content) != BodyLimit {
		t.Fatal("output not bounded", err)
	}
	file, err := os.OpenFile(r.LockFile, os.O_RDWR, 0600)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	if err := syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		t.Fatal(err)
	}
	if _, err := r.Fetch(context.Background(), "https://example.invalid/"); !errors.Is(err, ErrBusy) {
		t.Fatal("shared slot ignored", err)
	}
	syscall.Flock(int(file.Fd()), syscall.LOCK_UN)
	if _, err := r.Fetch(context.Background(), "https://example.invalid/"); err != nil {
		t.Fatal("released slot unavailable", err)
	}
}

func TestBrowserInspectionDeadlineKillsChildAndReleasesSlot(t *testing.T) {
	root := t.TempDir()
	pidFile := filepath.Join(root, "child.pid")
	homeFile := filepath.Join(root, "home")
	// Fixed fixture-controlled paths; no agent string is interpolated into a shell.
	r := fixtureRuntime(t, "printf '%s' \"$HOME\" > "+strconv.Quote(homeFile)+"\nsleep 30 &\nprintf '%s' \"$!\" > "+strconv.Quote(pidFile)+"\nwait\n")
	started := time.Now()
	_, err := r.fetch(context.Background(), "https://example.invalid/", 300*time.Millisecond)
	if !errors.Is(err, context.DeadlineExceeded) || time.Since(started) > 3*time.Second {
		t.Fatal("deadline not enforced", err)
	}
	raw, err := os.ReadFile(pidFile)
	if err != nil {
		t.Fatal(err)
	}
	pid, _ := strconv.Atoi(string(raw))
	// A killed child can remain a zombie until its parent/init reaps it. It must
	// not remain in a running/sleeping state after cancellation.
	if stat, err := os.ReadFile(filepath.Join("/proc", strconv.Itoa(pid), "stat")); err == nil {
		parts := strings.Split(string(stat), ") ")
		if len(parts) != 2 || !strings.HasPrefix(parts[1], "Z ") {
			t.Fatal("child still executing", string(stat))
		}
	}
	home, err := os.ReadFile(homeFile)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(string(home)); !os.IsNotExist(err) {
		t.Fatal("cancelled session retained files", err)
	}
	file, err := os.OpenFile(r.LockFile, os.O_RDWR, 0600)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	if err := syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		t.Fatal("cancelled session retained slot", err)
	}
	syscall.Flock(int(file.Fd()), syscall.LOCK_UN)
}

func TestBrowserInspectionRejectsCredentialsSchemesAndLockSymlinks(t *testing.T) {
	r := fixtureRuntime(t, "exit 0\n")
	for _, u := range []string{"file:///etc/passwd", "data:text/html,example", "https://user:secret@example.com", "not-a-url"} {
		if _, err := r.Fetch(context.Background(), u); err == nil {
			t.Fatal("invalid URL accepted", u)
		}
	}
	target := filepath.Join(t.TempDir(), "unrelated")
	if err := os.WriteFile(target, []byte("preserve"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, r.LockFile); err != nil {
		t.Fatal(err)
	}
	if _, err := r.Fetch(context.Background(), "https://example.invalid/"); err == nil {
		t.Fatal("symlink lock accepted")
	}
	raw, _ := os.ReadFile(target)
	if string(raw) != "preserve" {
		t.Fatal("unrelated file changed")
	}
}
