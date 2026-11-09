package runner

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"syscall"
	"testing"

	"github.com/buchin/cuan/internal/project"
)

func TestNativeReleaseReusesOwnerBuildCachesUnderMaintenanceLease(t *testing.T) {
	root, base := t.TempDir(), t.TempDir()
	for _, name := range []string{"go.mod", "package.json"} {
		if err := os.WriteFile(filepath.Join(root, name), []byte("fixture"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	input := []string{"HOME=/private-release/home", "GOCACHE=/old/checkout/cache", "GOPATH=/old/checkout/go", "NPM_CONFIG_CACHE=/old/npm", "NPM_CONFIG_USERCONFIG=/private-release/npmrc", "PATH=/usr/bin", "OWNER_PROJECT_SETTING=preserved"}
	execution := &NativeReleaseExecution{Workspace: &project.PreparedNativeReleaseWorkspace{Path: root}, Environment: input}
	workspaces := filepath.Join(base, "workspaces")
	environment, closeCaches, err := prepareNativeReleaseBuildCaches(context.Background(), execution, workspaces, "owner")
	if err != nil {
		t.Fatal(err)
	}
	defer closeCaches()
	values := map[string]string{}
	for _, entry := range environment {
		key, value, _ := strings.Cut(entry, "=")
		values[key] = value
	}
	expected, err := managedGoBuildEnvironment(root, filepath.Join(base, "go-build-runtime"), "owner")
	if err != nil {
		t.Fatal(err)
	}
	for key, value := range expected {
		if values[key] != value {
			t.Fatal("native release did not reuse developer cache", key)
		}
	}
	if values["HOME"] != "/private-release/home" || values["NPM_CONFIG_USERCONFIG"] != "/private-release/npmrc" || values["OWNER_PROJECT_SETTING"] != "preserved" {
		t.Fatal("cache preparation changed private configuration")
	}
	if !reflect.DeepEqual(input, execution.Environment) {
		t.Fatal("saved preparation environment mutated")
	}
	for _, cache := range []string{values["GOCACHE"], values["NPM_CONFIG_CACHE"]} {
		gate, err := os.OpenFile(filepath.Join(filepath.Dir(cache), ".build-cache.lock"), os.O_RDWR, 0600)
		if err != nil {
			t.Fatal(err)
		}
		if err = syscall.Flock(int(gate.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err == nil {
			gate.Close()
			t.Fatal("maintenance could erase active native cache")
		}
		gate.Close()
	}
	closeCaches()
	for _, cache := range []string{values["GOCACHE"], values["NPM_CONFIG_CACHE"]} {
		gate, err := os.OpenFile(filepath.Join(filepath.Dir(cache), ".build-cache.lock"), os.O_RDWR, 0600)
		if err != nil {
			t.Fatal(err)
		}
		if err = syscall.Flock(int(gate.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
			gate.Close()
			t.Fatal("cache remained busy after release", err)
		}
		gate.Close()
	}
	other, closeOther, err := prepareNativeReleaseBuildCaches(context.Background(), execution, workspaces, "another-owner")
	if err != nil {
		t.Fatal(err)
	}
	defer closeOther()
	for _, entry := range other {
		if entry == "GOCACHE="+values["GOCACHE"] || entry == "NPM_CONFIG_CACHE="+values["NPM_CONFIG_CACHE"] {
			t.Fatal("owners shared generated caches")
		}
	}
	if _, cleanup, err := prepareNativeReleaseBuildCaches(context.Background(), execution, workspaces, ""); err == nil {
		cleanup()
		t.Fatal("missing cache owner accepted")
	}
}
