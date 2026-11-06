package runner

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/buchin/cuan/internal/terminal"
)

func TestManagedNPMCacheSurvivesBrainChangesAndPreservesRegistry(t *testing.T) {
	root, base := t.TempDir(), t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "web"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "web/package.json"), []byte(`{"private":true}`), 0600); err != nil {
		t.Fatal(err)
	}
	first, err := managedNPMBuildEnvironment(root, base, "owner")
	if err != nil {
		t.Fatal(err)
	}
	again, err := managedNPMBuildEnvironment(root, base, "owner")
	if err != nil || again["NPM_CONFIG_CACHE"] != first["NPM_CONFIG_CACHE"] {
		t.Fatal(again, err)
	}
	other, err := managedNPMBuildEnvironment(root, base, "another-owner")
	if err != nil || other["NPM_CONFIG_CACHE"] == first["NPM_CONFIG_CACHE"] {
		t.Fatal(other, err)
	}
	binary, err := exec.LookPath("npm")
	if err != nil {
		t.Skip("npm required")
	}
	for _, registry := range []string{"https://registry-one.invalid/", "https://registry-two.invalid/"} {
		home := t.TempDir()
		if err := os.WriteFile(filepath.Join(home, ".npmrc"), []byte("registry="+registry+"\ncache="+filepath.Join(home, "old-cache")+"\n"), 0600); err != nil {
			t.Fatal(err)
		}
		env := terminal.SanitizeEnvironment([]string{"PATH=" + os.Getenv("PATH"), "HOME=" + home, "NPM_CONFIG_CACHE=" + first["NPM_CONFIG_CACHE"]}, "fixture", root)
		for key, want := range map[string]string{"cache": first["NPM_CONFIG_CACHE"], "registry": registry} {
			cmd := exec.CommandContext(context.Background(), binary, "config", "get", key)
			cmd.Dir = root
			cmd.Env = env
			output, err := cmd.Output()
			if err != nil || strings.TrimSpace(string(output)) != want {
				t.Fatalf("%s resolution: %q %v", key, output, err)
			}
		}
	}
}
func TestManagedNPMCacheSkipsOtherProjectsAndRejectsLinks(t *testing.T) {
	root, base := t.TempDir(), t.TempDir()
	env, err := managedNPMBuildEnvironment(root, base, "owner")
	if err != nil || len(env) != 0 {
		t.Fatal(env, err)
	}
	os.WriteFile(filepath.Join(root, "package.json"), []byte(`{}`), 0600)
	env, err = managedNPMBuildEnvironment(root, base, "owner")
	if err != nil {
		t.Fatal(err)
	}
	cache := env["NPM_CONFIG_CACHE"]
	if err := os.Remove(cache); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(t.TempDir(), cache); err != nil {
		t.Fatal(err)
	}
	if _, err = managedNPMBuildEnvironment(root, base, "owner"); err == nil {
		t.Fatal("accepted linked cache")
	}
}
