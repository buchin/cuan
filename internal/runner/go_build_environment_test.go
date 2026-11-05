package runner

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/buchin/cuan/internal/terminal"
)

func TestManagedGoBuildCachesSurviveBrainHomeChangesAndIsolateOwners(t *testing.T) {
	root, base := t.TempDir(), t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module fixture\n"), 0600); err != nil {
		t.Fatal(err)
	}
	first, err := managedGoBuildEnvironment(root, base, "owner-one")
	if err != nil {
		t.Fatal(err)
	}
	second, err := managedGoBuildEnvironment(root, base, "owner-one")
	if err != nil {
		t.Fatal(err)
	}
	other, err := managedGoBuildEnvironment(root, base, "owner-two")
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"GOPATH", "GOMODCACHE", "GOCACHE"} {
		if first[key] != second[key] || first[key] == other[key] {
			t.Fatal("cache lifecycle tied to brain or another owner", key)
		}
	}
	// Check actual Go environment resolution under distinct isolated profile HOMEs.
	goBin, err := exec.LookPath("go")
	if err != nil {
		t.Skip("Go tool required")
	}
	for _, home := range []string{t.TempDir(), t.TempDir()} {
		values := []string{"HOME=" + home, "PATH=" + os.Getenv("PATH")}
		for k, v := range first {
			values = append(values, k+"="+v)
		}
		// Go's telemetry mode is a file setting, not a GOTELEMETRY override.
		// Disable it only in this private fixture to avoid detached telemetry
		// writers racing TempDir cleanup after the short env command exits.
		mode := filepath.Join(home, ".config", "go", "telemetry", "mode")
		if err := os.MkdirAll(filepath.Dir(mode), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(mode, []byte("off\n"), 0600); err != nil {
			t.Fatal(err)
		}
		cmd := exec.CommandContext(context.Background(), goBin, "env", "GOMODCACHE", "GOCACHE")
		cmd.Dir = t.TempDir()
		cmd.Env = terminal.SanitizeEnvironment(values, "fixture", cmd.Dir)
		output, err := cmd.Output()
		if err != nil {
			t.Fatal(err)
		}
		if string(output) != first["GOMODCACHE"]+"\n"+first["GOCACHE"]+"\n" {
			t.Fatal("profile HOME changed managed cache resolution", string(output))
		}
	}
}
func TestManagedGoBuildEnvironmentExcludesOtherProjectsAndDirectoryLinks(t *testing.T) {
	root, base := t.TempDir(), t.TempDir()
	if env, err := managedGoBuildEnvironment(root, base, "owner"); err != nil || len(env) != 0 {
		t.Fatal("non-Go project changed", env, err)
	}
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module fixture\n"), 0600); err != nil {
		t.Fatal(err)
	}
	env, err := managedGoBuildEnvironment(root, base, "owner")
	if err != nil {
		t.Fatal(err)
	}
	cache := env["GOCACHE"]
	if err = os.Remove(cache); err != nil {
		t.Fatal(err)
	}
	if err = os.Symlink(t.TempDir(), cache); err != nil {
		t.Fatal(err)
	}
	if _, err = managedGoBuildEnvironment(root, base, "owner"); err == nil {
		t.Fatal("linked cross-owner cache accepted")
	}
}
