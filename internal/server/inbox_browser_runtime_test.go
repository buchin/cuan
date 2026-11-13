package server

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/buchin/cuan/internal/browserfetch"
)

func inboxBrowserFixture(t *testing.T) (browserfetch.Manifest, string) {
	t.Helper()
	root := t.TempDir()
	engine := filepath.Join(root, "obscura")
	body := []byte("#!/bin/sh\nexit 0\n")
	if err := os.WriteFile(engine, body, 0700); err != nil {
		t.Fatal(err)
	}
	sandbox := filepath.Join(root, "bwrap")
	if err := os.WriteFile(sandbox, body, 0700); err != nil {
		t.Fatal(err)
	}
	lock := filepath.Join(root, "browser.lock")
	if err := os.WriteFile(lock, nil, 0600); err != nil {
		t.Fatal(err)
	}
	hash := sha256.Sum256(body)
	manifest := browserfetch.Manifest{Runtime: browserfetch.Runtime{Binary: engine, Sandbox: sandbox, LockFile: lock}, Version: browserfetch.Version, ArchiveSHA256: strings.Repeat("a", 64), BinarySHA256: hex.EncodeToString(hash[:])}
	raw, _ := json.Marshal(manifest)
	path := filepath.Join(root, "runtime.json")
	if err := os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("CUAN_CHAT_BROWSER_MANIFEST", path)
	t.Setenv("PATH", root+":"+os.Getenv("PATH"))
	return manifest, path
}

func TestInboxBrowserMountsUseSharedLeaseAndPrivateProjection(t *testing.T) {
	source, _ := inboxBrowserFixture(t)
	home := t.TempDir()
	mcp, err := prepareInboxMCPBoundary(home)
	if err != nil {
		t.Fatal(err)
	}
	args, err := prepareInboxBrowserMounts(home, mcp)
	if err != nil {
		t.Fatal(err)
	}
	foundLock, foundManifest := false, false
	var projection string
	for i := 0; i+2 < len(args); i++ {
		if args[i] == "--bind" && args[i+1] == source.LockFile && args[i+2] == "/cuan-chat-browser/slot.lock" {
			foundLock = true
		}
		if args[i] == "--ro-bind" && args[i+2] == "/cuan-chat-browser/runtime.json" {
			foundManifest = true
			projection = args[i+1]
		}
		if (args[i] == "--bind" || args[i] == "--ro-bind") && args[i+1] == filepath.Dir(source.Binary) {
			t.Fatal("whole service runtime directory exposed")
		}
	}
	if !foundLock || !foundManifest {
		t.Fatal("missing shared lease or private manifest", args)
	}
	raw, err := os.ReadFile(projection)
	if err != nil {
		t.Fatal(err)
	}
	var selected browserfetch.Manifest
	if err := json.Unmarshal(raw, &selected); err != nil {
		t.Fatal(err)
	}
	if selected.Binary != "/cuan-chat-browser/obscura" || selected.Sandbox != "/cuan-chat-browser/bwrap" || selected.BinarySHA256 != source.BinarySHA256 || strings.Contains(string(raw), filepath.Dir(source.Binary)) {
		t.Fatal("host selection leaked or changed", string(raw))
	}
	info, err := os.Stat(projection)
	if err != nil || info.Mode().Perm() != 0600 {
		t.Fatal("projection not private", info, err)
	}
}

func TestInboxBrowserFailureLeavesOrdinaryChatAvailable(t *testing.T) {
	source, path := inboxBrowserFixture(t)
	home := t.TempDir()
	mcp, err := prepareInboxMCPBoundary(home)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(source.Binary, []byte("modified"), 0700); err != nil {
		t.Fatal(err)
	}
	mounts, err := prepareInboxBrowserMounts(home, mcp)
	if err != nil || len(mounts) != 0 {
		t.Fatal("invalid optional runtime disabled chat", mounts, err)
	}
	t.Setenv("CUAN_CHAT_BROWSER_MANIFEST", path+".missing")
	mounts, err = prepareInboxBrowserMounts(home, mcp)
	if err != nil || len(mounts) != 0 {
		t.Fatal("missing optional runtime disabled chat", mounts, err)
	}
}

func TestInboxBrowserManifestCannotBeOverriddenByProviderEnvironment(t *testing.T) {
	inboxBrowserFixture(t)
	home := t.TempDir()
	_, args, err := sandboxInboxCommand("/bin/sh", nil, []string{"PATH=/usr/bin:/bin", "CUAN_CHAT_BROWSER_MANIFEST=/ambient/provider/runtime.json"}, home, "", nil)
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for i := 0; i+2 < len(args); i++ {
		if args[i] == "--setenv" && args[i+1] == "CUAN_CHAT_BROWSER_MANIFEST" {
			count++
			if args[i+2] != "/cuan-chat-browser/runtime.json" {
				t.Fatal("provider replaced application runtime", args[i+2])
			}
		}
	}
	if count != 1 {
		t.Fatal("runtime selection lost or duplicated", count)
	}
}

func TestInboxBrowserProjectionUpdatesWithoutOverwritingOldBinding(t *testing.T) {
	source, manifestPath := inboxBrowserFixture(t)
	home := t.TempDir()
	mcp, err := prepareInboxMCPBoundary(home)
	if err != nil {
		t.Fatal(err)
	}
	projection := func(args []string) string {
		for i := 0; i+2 < len(args); i++ {
			if args[i] == "--ro-bind" && args[i+2] == "/cuan-chat-browser/runtime.json" {
				return args[i+1]
			}
		}
		return ""
	}
	before, err := prepareInboxBrowserMounts(home, mcp)
	if err != nil {
		t.Fatal(err)
	}
	oldPath := projection(before)
	oldData, err := os.ReadFile(oldPath)
	if err != nil {
		t.Fatal(err)
	}
	body := []byte("#!/bin/sh\nprintf fresh\n")
	source.Binary = source.Binary + ".new"
	if err := os.WriteFile(source.Binary, body, 0700); err != nil {
		t.Fatal(err)
	}
	hash := sha256.Sum256(body)
	source.BinarySHA256 = hex.EncodeToString(hash[:])
	raw, _ := json.Marshal(source)
	if err := os.WriteFile(manifestPath, raw, 0600); err != nil {
		t.Fatal(err)
	}
	after, err := prepareInboxBrowserMounts(home, mcp)
	if err != nil {
		t.Fatal("updated runtime blocked chat", err)
	}
	if projection(after) == oldPath || projection(after) == "" {
		t.Fatal("runtime snapshot reused old immutable binding")
	}
	saved, err := os.ReadFile(oldPath)
	if err != nil || string(saved) != string(oldData) {
		t.Fatal("old binding overwritten", err)
	}
}

func TestInboxBrowserAllowsChildUserNamespaceMapsWithKernelControlsReadOnly(t *testing.T) {
	inboxBrowserFixture(t)
	args, err := inboxSandboxArgs("/bin/sh", t.TempDir(), "", nil)
	if err != nil {
		t.Fatal(err)
	}
	privateProc, readOnlyControls := false, false
	for i := 0; i+1 < len(args); i++ {
		if args[i] == "--proc" && args[i+1] == "/proc" {
			privateProc = true
		}
		if args[i] == "--remount-ro" && args[i+1] == "/proc" {
			t.Fatal("child UID maps remain blocked")
		}
		if i+2 < len(args) && args[i] == "--ro-bind" && args[i+1] == "/proc/sys" && args[i+2] == "/proc/sys" {
			readOnlyControls = true
		}
	}
	if !privateProc || !readOnlyControls {
		t.Fatal("PID namespace or kernel controls lost", args)
	}
	t.Setenv("CUAN_CHAT_BROWSER_MANIFEST", filepath.Join(t.TempDir(), "missing"))
	args, err = inboxSandboxArgs("/bin/sh", t.TempDir(), "", nil)
	if err != nil {
		t.Fatal(err)
	}
	readOnlyProc := false
	for i := 0; i+1 < len(args); i++ {
		if args[i] == "--remount-ro" && args[i+1] == "/proc" {
			readOnlyProc = true
		}
	}
	if !readOnlyProc {
		t.Fatal("ordinary chat proc policy changed")
	}
}
