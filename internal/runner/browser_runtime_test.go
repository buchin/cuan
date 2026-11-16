package runner

import (
	"context"
	"github.com/buchin/cuan/internal/task"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestBrowserRuntimeExposesInstalledOfficialToolOutsideWorkspace(t *testing.T) {
	root := t.TempDir()
	bin := filepath.Join(root, "bin")
	module := filepath.Join(root, "runtime", "playwright-core")
	for _, dir := range []string{bin, module} {
		if err := os.MkdirAll(dir, 0700); err != nil {
			t.Fatal(err)
		}
	}
	cli := filepath.Join(module, "cli.js")
	if err := os.WriteFile(cli, []byte("#!/usr/bin/env node\n"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(cli, filepath.Join(bin, "cuan-browser")); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin)
	t.Setenv("XDG_CACHE_HOME", filepath.Join(root, "cache"))
	env := map[string]string{}
	if browserRuntimeEnvironment(env, true) != "" || len(env) != 0 {
		t.Fatal("accepted unidentified package")
	}
	if err := os.WriteFile(filepath.Join(module, "package.json"), []byte(`{"name":"playwright-core"}`), 0600); err != nil {
		t.Fatal(err)
	}
	prompt := browserRuntimeEnvironment(env, true)
	resolvedModule, err := filepath.EvalSymlinks(module)
	if err != nil {
		resolvedModule = module
	}
	if env["CUAN_BROWSER_MODULE"] != resolvedModule || env["CUAN_BROWSER_CLI"] != filepath.Join(bin, "cuan-browser") || !strings.Contains(prompt, "instead of searching") {
		t.Fatal("missing installed tool context")
	}
	if env["PLAYWRIGHT_BROWSERS_PATH"] != filepath.Join(root, "cache", "ms-playwright") {
		t.Fatal("browser cache would depend on private AI HOME")
	}
	if browserRuntimeEnvironment(env, false) != "" {
		t.Fatal("unnecessary visual guidance for nonvisual task")
	}
}

func TestBrowserRuntimeHeartbeatReadinessExpiresAndNeverBorrowsOldEvidence(t *testing.T) {
	ctx := context.Background()
	store := task.NewMemoryStore()
	service := NewService(NewMemoryStore(store), store, nil)
	evidence := &task.VerificationRuntime{Fingerprint: strings.Repeat("a", 64), Version: "1.64.1", CheckedAt: time.Now()}
	if _, err := service.Heartbeat(ctx, HeartbeatRequest{RunnerID: "runner", BrowserRuntime: evidence}, time.Minute); err != nil {
		t.Fatal(err)
	}
	if service.BrowserRuntime(ctx, "runner") == nil {
		t.Fatal("fresh authenticated render unavailable")
	}
	evidence.CheckedAt = time.Now().Add(-time.Hour)
	if _, err := service.Heartbeat(ctx, HeartbeatRequest{RunnerID: "runner", BrowserRuntime: evidence}, time.Minute); err != nil {
		t.Fatal(err)
	}
	if service.BrowserRuntime(ctx, "runner") != nil {
		t.Fatal("stale render borrowed previous readiness")
	}
	if _, err := service.Heartbeat(ctx, HeartbeatRequest{RunnerID: "runner"}, time.Minute); err != nil {
		t.Fatal(err)
	}
	if service.BrowserRuntime(ctx, "runner") != nil {
		t.Fatal("missing evidence treated as ready")
	}
}

func TestBrowserRuntimeEvidenceCacheReturnsIndependentCopies(t *testing.T) {
	original := strings.Repeat("a", 64)
	r := &Runner{browserEvidence: &task.VerificationRuntime{Fingerprint: original, Version: "1.64.1", CheckedAt: time.Now()}, browserNextProbe: time.Now().Add(time.Hour)}
	var workers sync.WaitGroup
	for i := 0; i < 16; i++ {
		workers.Add(1)
		go func() {
			defer workers.Done()
			value := r.browserRuntimeEvidence(context.Background())
			if value == nil {
				t.Error("cached readiness missing")
				return
			}
			value.Fingerprint = "changed caller copy"
		}()
	}
	workers.Wait()
	if got := r.browserRuntimeEvidence(context.Background()); got == nil || got.Fingerprint != original {
		t.Fatal("caller mutated readiness cache")
	}
}

func TestBrowserRuntimeFastPathIntegrityCheck(t *testing.T) {
	root := t.TempDir()
	bin := filepath.Join(root, "bin")
	module := filepath.Join(root, "runtime", "playwright-core")
	for _, dir := range []string{bin, module} {
		if err := os.MkdirAll(dir, 0700); err != nil {
			t.Fatal(err)
		}
	}
	cli := filepath.Join(module, "cli.js")
	if err := os.WriteFile(cli, []byte("#!/usr/bin/env node\n"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(cli, filepath.Join(bin, "cuan-browser")); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(module, "package.json"), []byte(`{"name":"playwright-core","version":"1.64.1"}`), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin)
	t.Setenv("XDG_CACHE_HOME", filepath.Join(root, "cache"))

	// Quick verify should succeed for matching version
	if !quickVerifyBrowserRuntime("1.64.1") {
		t.Fatal("expected quickVerifyBrowserRuntime to succeed")
	}
	// Quick verify should fail for non-matching version
	if quickVerifyBrowserRuntime("9.99.9") {
		t.Fatal("expected quickVerifyBrowserRuntime to fail on version mismatch")
	}

	// Runner fast-path: updates CheckedAt without running probe
	r := &Runner{
		browserEvidence: &task.VerificationRuntime{
			Fingerprint: strings.Repeat("b", 64),
			Version:     "1.64.1",
			CheckedAt:   time.Now().Add(-5 * time.Minute),
		},
		browserLastFullProbe: time.Now().Add(-10 * time.Minute),
		browserNextProbe:     time.Now().Add(time.Hour),
	}
	oldChecked := r.browserEvidence.CheckedAt
	evidence := r.browserRuntimeEvidence(context.Background())
	if evidence == nil {
		t.Fatal("expected evidence from fast path")
	}
	if !evidence.CheckedAt.After(oldChecked) {
		t.Fatalf("expected CheckedAt to be refreshed, got %v <= %v", evidence.CheckedAt, oldChecked)
	}
}
