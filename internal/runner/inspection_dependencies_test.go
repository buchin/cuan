package runner

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestInspectionDependenciesUsePrivateLockedSetup(t *testing.T) {
	for _, scenario := range []string{"valid", "shared dependencies", "linked lock", "tracked output", "failed installer", "git dependency", "file dependency", "embedded credentials"} {
		t.Run(scenario, func(t *testing.T) {
			root := t.TempDir()
			bin := t.TempDir()
			write := func(p, s string) {
				t.Helper()
				if err := os.WriteFile(p, []byte(s), 0700); err != nil {
					t.Fatal(err)
				}
			}
			if out, err := exec.Command("git", "init", root).CombinedOutput(); err != nil {
				t.Fatal(err, string(out))
			}
			write(filepath.Join(root, "package.json"), `{"name":"fixture","version":"1.0.0"}`)
			write(filepath.Join(root, "package-lock.json"), `{"lockfileVersion":3,"packages":{}}`)
			for name, endpoint := range map[string]string{"git dependency": "git+ssh://example.invalid/repository", "file dependency": "file:../../shared", "embedded credentials": "https://user:private@example.invalid/package.tgz"} {
				if scenario == name {
					write(filepath.Join(root, "package-lock.json"), `{"lockfileVersion":3,"packages":{"node_modules/example":{"resolved":"`+endpoint+`"}}}`)
				}
			}
			if scenario != "tracked output" {
				write(filepath.Join(root, ".gitignore"), "node_modules/\n")
			}
			if scenario == "shared dependencies" {
				if err := os.Symlink(t.TempDir(), filepath.Join(root, "node_modules")); err != nil {
					t.Fatal(err)
				}
			}
			if scenario == "linked lock" {
				_ = os.Remove(filepath.Join(root, "package-lock.json"))
				if err := os.Symlink(filepath.Join(root, "package.json"), filepath.Join(root, "package-lock.json")); err != nil {
					t.Fatal(err)
				}
			}
			script := "#!/bin/sh\n[ -z \"$CUAN_AUTH_TOKEN$NPM_TOKEN$CUAN_TASK_CAPABILITY_JSON\" ] || exit 9\n[ \"$HOME\" != \"$ORIGINAL_HOME\" ] || exit 10\nprintf '%s\\n' \"$@\" > setup-arguments\nprintf '%s' \"$HOME\" > setup-home\n"
			if scenario == "failed installer" {
				script += "printf 'npm ERR code ENOSPC\\nsecret=private-installer-secret\\n' >&2\nexit 7\n"
			}
			write(filepath.Join(bin, "npm"), script)
			t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
			t.Setenv("CUAN_AUTH_TOKEN", "private")
			t.Setenv("NPM_TOKEN", "private")
			t.Setenv("CUAN_TASK_CAPABILITY_JSON", "private")
			t.Setenv("ORIGINAL_HOME", os.Getenv("HOME"))
			err := prepareInspectionDependencies(context.Background(), root)
			if (err == nil) != (scenario == "valid") {
				t.Fatalf("unexpected setup acceptance: %v", err)
			}
			if scenario == "failed installer" && (!strings.Contains(err.Error(), "ENOSPC") || strings.Contains(err.Error(), "private-installer-secret")) {
				t.Fatal("missing useful or leaked installer diagnostic", err)
			}
			args, readErr := os.ReadFile(filepath.Join(root, "setup-arguments"))
			if scenario == "valid" {
				if readErr != nil || !strings.Contains(string(args), "ci\n--ignore-scripts\n--no-audit\n--no-fund\n") {
					t.Fatal("installer did not use locked script-free setup")
				}
				home, _ := os.ReadFile(filepath.Join(root, "setup-home"))
				if _, err := os.Stat(string(home)); !os.IsNotExist(err) {
					t.Fatal("private installer home was retained")
				}
			} else if scenario != "failed installer" && readErr == nil {
				t.Fatal("unsafe layout launched installer")
			}
		})
	}
}
