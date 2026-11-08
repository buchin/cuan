package runner

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestPHPDependenciesAreLockedPrivateAndDoNotBorrowCredentials(t *testing.T) {
	for _, scenario := range []string{"valid", "missing lock", "shared vendor", "linked lock", "tracked vendor", "custom repository", "custom cache", "credential URL", "local archive", "failed installer"} {
		t.Run(scenario, func(t *testing.T) {
			root, bin := t.TempDir(), t.TempDir()
			write := func(name, value string) {
				t.Helper()
				if err := os.WriteFile(filepath.Join(root, name), []byte(value), 0600); err != nil {
					t.Fatal(err)
				}
			}
			if out, err := exec.Command("git", "init", root).CombinedOutput(); err != nil {
				t.Fatal(err, string(out))
			}
			write(".gitignore", "vendor/\n")
			write("composer.json", `{"name":"fixture/test"}`)
			lock := `{"content-hash":"fixture","packages":[{"type":"library","dist":{"type":"zip","url":"https://example.invalid/package.zip"}}],"packages-dev":[]}`
			if scenario == "credential URL" {
				lock = strings.ReplaceAll(lock, "https://example.invalid", "https://user:private@example.invalid")
			}
			if scenario == "local archive" {
				lock = strings.ReplaceAll(lock, "https://example.invalid/package.zip", "file:///tmp/shared.zip")
			}
			write("composer.lock", lock)
			switch scenario {
			case "missing lock":
				os.Remove(filepath.Join(root, "composer.lock"))
			case "shared vendor":
				os.Symlink(t.TempDir(), filepath.Join(root, "vendor"))
			case "linked lock":
				os.Remove(filepath.Join(root, "composer.lock"))
				os.Symlink(filepath.Join(root, "composer.json"), filepath.Join(root, "composer.lock"))
			case "tracked vendor":
				os.Remove(filepath.Join(root, ".gitignore"))
			case "custom cache":
				write("composer.json", `{"config":{"cache-files-dir":"/tmp/shared-cache"}}`)
			case "custom repository":
				write("composer.json", `{"repositories":[{"type":"path","url":"../shared"}]}`)
			}
			script := "#!/bin/sh\n[ -z \"$COMPOSER_AUTH$GH_TOKEN$GITHUB_TOKEN$CUAN_TASK_CAPABILITY_JSON$SSH_AUTH_SOCK\" ] || exit 9\nprintf '%s\\n' \"$@\" > setup-arguments\nprintf '%s' \"$HOME\" > setup-home\n"
			if scenario == "failed installer" {
				script += "echo 'ENOSPC secret=private-installer-secret' >&2\nexit 7\n"
			}
			if err := os.WriteFile(filepath.Join(bin, "composer"), []byte(script), 0700); err != nil {
				t.Fatal(err)
			}
			t.Setenv("PATH", bin+":"+os.Getenv("PATH"))
			for _, key := range []string{"COMPOSER_AUTH", "GH_TOKEN", "GITHUB_TOKEN", "CUAN_TASK_CAPABILITY_JSON", "SSH_AUTH_SOCK"} {
				t.Setenv(key, "private")
			}
			err := preparePHPDependencies(context.Background(), root, t.TempDir(), "owner")
			if (err == nil) != (scenario == "valid") {
				t.Fatal("incorrect setup admission", err)
			}
			args, readErr := os.ReadFile(filepath.Join(root, "setup-arguments"))
			if scenario == "valid" {
				if readErr != nil || string(args) != "--no-plugins\n--no-scripts\ninstall\n--prefer-dist\n--no-interaction\n--no-progress\n" {
					t.Fatal("unlocked or script-capable setup", string(args))
				}
				home, _ := os.ReadFile(filepath.Join(root, "setup-home"))
				if string(home) == os.Getenv("HOME") {
					t.Fatal("borrowed user home")
				}
				if _, err := os.Stat(string(home)); !os.IsNotExist(err) {
					t.Fatal("installer home retained")
				}
			} else if scenario != "failed installer" && readErr == nil {
				t.Fatal("unsafe preparation launched installer")
			}
			if scenario == "failed installer" && (!strings.Contains(err.Error(), "ENOSPC") || strings.Contains(err.Error(), "private-installer-secret")) {
				t.Fatal("installer diagnostic leaked or lost", err)
			}
		})
	}
}

func TestPHPDependenciesRealComposerPreservesLockAndSkipsScripts(t *testing.T) {
	if _, err := exec.LookPath("composer"); err != nil {
		t.Skip("Composer is not installed")
	}
	root := t.TempDir()
	home := t.TempDir()
	manifest := `{"name":"fixture/locked-setup","require":{},"scripts":{"post-install-cmd":"touch must-not-run"}}`
	if err := os.WriteFile(filepath.Join(root, "composer.json"), []byte(manifest), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".gitignore"), []byte("vendor/\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command("git", "init", root).CombinedOutput(); err != nil {
		t.Fatal(err, string(out))
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "composer", "update", "--no-install", "--no-plugins", "--no-scripts", "--no-interaction", "--no-progress")
	cmd.Dir = root
	cmd.Env = []string{"PATH=" + os.Getenv("PATH"), "HOME=" + home, "COMPOSER_HOME=" + home, "COMPOSER_DISABLE_NETWORK=1"}
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatal("offline fixture lock preparation failed", err, string(out))
	}
	before, err := os.ReadFile(filepath.Join(root, "composer.lock"))
	if err != nil {
		t.Fatal(err)
	}
	if err := preparePHPDependencies(ctx, root, t.TempDir(), "owner"); err != nil {
		t.Fatal(err)
	}
	after, err := os.ReadFile(filepath.Join(root, "composer.lock"))
	if err != nil || string(before) != string(after) {
		t.Fatal("setup changed lock")
	}
	if _, err := os.Stat(filepath.Join(root, "vendor", "autoload.php")); err != nil {
		t.Fatal("Composer did not install autoloader", err)
	}
	if _, err := os.Stat(filepath.Join(root, "must-not-run")); !os.IsNotExist(err) {
		t.Fatal("project install script ran")
	}
}

// Simulate a partial download, then a fresh verifier snapshot. The archive
// survives the failed install, but neither HOME nor vendor crosses snapshots.
func TestPHPPreparationRetainsDownloadsAcrossSnapshotsAndIsolatesOwners(t *testing.T) {
	base, bin := t.TempDir(), t.TempDir()
	script := `#!/bin/sh
[ -z "$COMPOSER_AUTH$GH_TOKEN$SSH_AUTH_SOCK" ] || exit 9
printf '%s' "$COMPOSER_CACHE_DIR" > setup-cache
printf '%s' "$HOME" > setup-home
[ -f "$COMPOSER_HOME/config.json" ] || exit 8
mkdir -p "$COMPOSER_CACHE_DIR/files" vendor
if [ -f "$COMPOSER_CACHE_DIR/files/archive.zip" ]; then
 echo reused > vendor/result
else
 echo downloaded > "$COMPOSER_CACHE_DIR/files/archive.zip"
 echo partial > vendor/result
 exit 7
fi
`
	if err := os.WriteFile(filepath.Join(bin, "composer"), []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+":"+os.Getenv("PATH"))
	t.Setenv("COMPOSER_AUTH", "must-not-inherit")
	fixture := func() string {
		root := t.TempDir()
		for name, value := range map[string]string{"composer.json": `{"name":"fixture/cache"}`, "composer.lock": `{"content-hash":"fixture","packages":[],"packages-dev":[]}`, ".gitignore": "vendor/\n"} {
			if err := os.WriteFile(filepath.Join(root, name), []byte(value), 0600); err != nil {
				t.Fatal(err)
			}
		}
		if out, err := exec.Command("git", "init", root).CombinedOutput(); err != nil {
			t.Fatal(err, string(out))
		}
		return root
	}
	first, second, other := fixture(), fixture(), fixture()
	if err := preparePHPDependencies(context.Background(), first, base, "one"); err == nil {
		t.Fatal("partial install passed")
	}
	if err := preparePHPDependencies(context.Background(), second, base, "one"); err != nil {
		t.Fatal("archive not reused", err)
	}
	if err := preparePHPDependencies(context.Background(), other, base, "two"); err == nil {
		t.Fatal("foreign owner's archive reused")
	}
	read := func(root, name string) string {
		v, err := os.ReadFile(filepath.Join(root, name))
		if err != nil {
			t.Fatal(err)
		}
		return string(v)
	}
	if read(first, "setup-cache") != read(second, "setup-cache") || read(first, "setup-cache") == read(other, "setup-cache") {
		t.Fatal("wrong cache ownership")
	}
	if read(first, "vendor/result") != "partial\n" || read(second, "vendor/result") != "reused\n" {
		t.Fatal("vendor was shared")
	}
	for _, root := range []string{first, second, other} {
		if _, err := os.Stat(read(root, "setup-home")); !os.IsNotExist(err) {
			t.Fatal("temporary config/auth HOME retained")
		}
	}
	// Cancellation kills the whole installer tree and reports an explicit timeout,
	// while retaining the already downloaded archive for the bounded recovery.
	if err := os.WriteFile(filepath.Join(bin, "composer"), []byte("#!/bin/sh\nsleep 30 &\nwait\n"), 0700); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	started := time.Now()
	err := preparePHPDependencies(ctx, second, base, "one")
	if err == nil || !strings.Contains(err.Error(), "setup deadline exceeded") || time.Since(started) > 3*time.Second {
		t.Fatal("timeout diagnosis/process boundary lost", err)
	}
	if _, err := os.Stat(filepath.Join(read(second, "setup-cache"), "files/archive.zip")); err != nil {
		t.Fatal("timeout discarded archives", err)
	}
}
