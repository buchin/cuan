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
	for _, scenario := range []string{"valid", "missing lock", "shared vendor", "linked lock", "tracked vendor", "custom repository", "credential URL", "local archive", "failed installer"} {
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
			err := preparePHPDependencies(context.Background(), root)
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
	if err := preparePHPDependencies(ctx, root); err != nil {
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
