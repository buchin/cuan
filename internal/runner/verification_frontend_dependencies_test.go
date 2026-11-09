package runner

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/buchin/cuan/internal/agent"
	"github.com/buchin/cuan/internal/config"
	"github.com/buchin/cuan/internal/executionpolicy"
	cuangit "github.com/buchin/cuan/internal/git"
	"github.com/buchin/cuan/internal/task"
	"github.com/buchin/cuan/internal/taskprocess"
)

func setupWarmFrontendSource(t *testing.T) (source, commit, manifestContent, lockContent string) {
	t.Helper()
	source = privateCompletionFixture(t)
	manifestContent = `{"name":"fixture","version":"1.0.0"}`
	lockContent = `{"name":"fixture","version":"1.0.0","lockfileVersion":3,"packages":{}}`
	if err := os.WriteFile(filepath.Join(source, "package.json"), []byte(manifestContent), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, "package-lock.json"), []byte(lockContent), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, ".gitignore"), []byte("node_modules/\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(source, "scripts"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, "scripts", "verify-ui.sh"), []byte("#!/bin/sh\nexit 0\n"), 0755); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("git", "add", "package.json", "package-lock.json", ".gitignore", "scripts/verify-ui.sh")
	cmd.Dir = source
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatal(err, string(out))
	}
	cmd = exec.Command("git", "commit", "-m", "add frontend files")
	cmd.Dir = source
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatal(err, string(out))
	}
	head, err := fingerprintGit(context.Background(), source, "rev-parse", "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	commit = strings.TrimSpace(string(head))

	// Warm node_modules with valid lock fingerprint
	modDir := filepath.Join(source, "node_modules")
	if err := os.MkdirAll(filepath.Join(modDir, "fixture-pkg"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(modDir, "fixture-pkg", "index.js"), []byte("export const v = 1;"), 0600); err != nil {
		t.Fatal(err)
	}
	digest := dependencyLockFingerprint([]byte(manifestContent), []byte(lockContent))
	if err := os.WriteFile(filepath.Join(modDir, dependencyLockFingerprintFilename), []byte(digest), 0600); err != nil {
		t.Fatal(err)
	}
	return source, commit, manifestContent, lockContent
}

// TestBackendVerifierDoesNotSeedNodeModules verifies that a pure backend verifier
// does not copy node_modules to its isolated snapshot.
func TestBackendVerifierDoesNotSeedNodeModules(t *testing.T) {
	ctx := context.Background()
	source, commit, _, _ := setupWarmFrontendSource(t)

	var completion CompleteAttemptRequest
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/report" {
			w.Write([]byte(`{"summary":"Backend verified","complete_note":"Targeted checks passed","activities":[{"attempt_id":"verifier","activity_type":"agent_report","new_status":"review"}]}`))
			return
		}
		if strings.HasSuffix(r.URL.Path, "/complete") {
			_ = json.NewDecoder(r.Body).Decode(&completion)
			json.NewEncoder(w).Encode(task.TaskAttempt{ID: "verifier", TaskID: filepath.Base(source), RunnerID: "fixture", Status: task.AttemptStatusSucceeded})
			return
		}
		w.Write([]byte(`{}`))
	}))
	defer api.Close()

	called := false
	adapter := &reportingBudgetAdapter{readOnlyAuditAdapter: &readOnlyAuditAdapter{t: t}, execute: func(_ context.Context, req agent.ExecutionRequest) (*agent.ExecutionResult, error) {
		called = true
		targetModules := filepath.Join(req.WorkingDir, "node_modules")
		if _, err := os.Stat(targetModules); !os.IsNotExist(err) {
			t.Fatalf("expected node_modules to NOT be copied to backend verifier, but it exists")
		}
		return &agent.ExecutionResult{Status: "SUCCEEDED", ExitCode: 0}, nil
	}}
	worker := NewRunner(&config.Config{RunnerID: "fixture", RunnerWorkerRegistryDir: t.TempDir(), RunnerServerURL: api.URL, WorkspacesDir: filepath.Dir(source)}, adapter, api.Client())
	claim := ClaimTaskResponse{
		Task: &task.Task{
			ID:                filepath.Base(source),
			Title:             "Backend runner optimization",
			Instructions:      "1 task backend terfokus, tanpa modifikasi UI, browser screenshot, atau email.",
			ExecutionCategory: task.ExecutionCategoryCode,
		},
		Attempt:           &task.TaskAttempt{ID: "verifier", TaskID: filepath.Base(source), CreatedAt: time.Now()},
		Verification:      &task.VerificationIntent{IsolatedWorkspace: true, SourceCommit: commit},
		RequiresVision:    false,
		PublicationLimits: executionpolicy.PublicationPolicy{ReadOnlyRepository: true, DenyPush: true, DenyDeploy: true},
		TaskAgent:         &TaskAgentCapability{API: api.URL + "/report", Token: "fixture"},
	}
	worker.runAttempt(ctx, claim)

	if !called || completion.Status != task.AttemptStatusSucceeded {
		t.Fatalf("verification attempt did not succeed: called=%t status=%s", called, completion.Status)
	}

	snapshotDir := cuangit.VerificationWorkspacePath(filepath.Dir(source), filepath.Base(source), commit)
	targetModules := filepath.Join(snapshotDir, "node_modules")
	if _, err := os.Stat(targetModules); !os.IsNotExist(err) {
		t.Fatalf("backend verifier snapshot retained unnecessary node_modules copy")
	}
}

// TestFrontendVerifierRetainsDependenciesWithoutRequiresVision verifies that verification
// requiring frontend preserves complete dependencies even when RequiresVision is false.
func TestFrontendVerifierRetainsDependenciesWithoutRequiresVision(t *testing.T) {
	ctx := context.Background()
	source, commit, manifest, lock := setupWarmFrontendSource(t)

	var completion CompleteAttemptRequest
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/report" {
			w.Write([]byte(`{"summary":"Frontend verified","complete_note":"Visual recipe checked","activities":[{"attempt_id":"verifier","activity_type":"agent_report","new_status":"review"}]}`))
			return
		}
		if strings.HasSuffix(r.URL.Path, "/complete") {
			_ = json.NewDecoder(r.Body).Decode(&completion)
			json.NewEncoder(w).Encode(task.TaskAttempt{ID: "verifier", TaskID: filepath.Base(source), RunnerID: "fixture", Status: task.AttemptStatusSucceeded})
			return
		}
		w.Write([]byte(`{}`))
	}))
	defer api.Close()

	called := false
	adapter := &reportingBudgetAdapter{readOnlyAuditAdapter: &readOnlyAuditAdapter{t: t}, execute: func(_ context.Context, req agent.ExecutionRequest) (*agent.ExecutionResult, error) {
		called = true
		targetModules := filepath.Join(req.WorkingDir, "node_modules")
		pkgFile := filepath.Join(targetModules, "fixture-pkg", "index.js")
		if data, err := os.ReadFile(pkgFile); err != nil || string(data) != "export const v = 1;" {
			t.Fatalf("expected frontend dependencies to be present in verifier snapshot, got: %v (data: %s)", err, string(data))
		}
		expectedDigest := dependencyLockFingerprint([]byte(manifest), []byte(lock))
		if !checkLockfileWarm(req.WorkingDir, "node_modules", expectedDigest) {
			t.Fatalf("expected valid lock fingerprint in verifier snapshot")
		}
		return &agent.ExecutionResult{Status: "SUCCEEDED", ExitCode: 0}, nil
	}}
	worker := NewRunner(&config.Config{RunnerID: "fixture", RunnerWorkerRegistryDir: t.TempDir(), RunnerServerURL: api.URL, WorkspacesDir: filepath.Dir(source)}, adapter, api.Client())
	// Task has frontend scope in title & instructions, RequiresVision is false
	claim := ClaimTaskResponse{
		Task: &task.Task{
			ID:                filepath.Base(source),
			Title:             "Frontend component verification",
			Instructions:      "Verify frontend visual components and styling in web package",
			ExecutionCategory: task.ExecutionCategoryCode,
		},
		Attempt:           &task.TaskAttempt{ID: "verifier", TaskID: filepath.Base(source), CreatedAt: time.Now()},
		Verification:      &task.VerificationIntent{IsolatedWorkspace: true, SourceCommit: commit},
		RequiresVision:    false, // NOT relying on RequiresVision alone!
		PublicationLimits: executionpolicy.PublicationPolicy{ReadOnlyRepository: true, DenyPush: true, DenyDeploy: true},
		TaskAgent:         &TaskAgentCapability{API: api.URL + "/report", Token: "fixture"},
	}
	worker.runAttempt(ctx, claim)

	if !called || completion.Status != task.AttemptStatusSucceeded {
		t.Fatalf("frontend verification did not succeed: called=%t status=%s err=%s", called, completion.Status, completion.ErrorMessage)
	}

	snapshotDir := cuangit.VerificationWorkspacePath(filepath.Dir(source), filepath.Base(source), commit)
	targetModules := filepath.Join(snapshotDir, "node_modules")
	if _, err := os.Stat(filepath.Join(targetModules, "fixture-pkg", "index.js")); err != nil {
		t.Fatalf("frontend verifier snapshot missing node_modules: %v", err)
	}
}

// TestFrontendVerifierRetainsDependenciesWithUIProhibitionConstraint verifies that frontend
// verification preserves dependencies even when instructions contain interface modification restrictions
// (e.g. "Do not modify UI", "tanpa modifikasi antarmuka").
func TestFrontendVerifierRetainsDependenciesWithUIProhibitionConstraint(t *testing.T) {
	ctx := context.Background()
	source, commit, manifest, lock := setupWarmFrontendSource(t)

	var completion CompleteAttemptRequest
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/report" {
			w.Write([]byte(`{"summary":"Frontend verified","complete_note":"Checks passed","activities":[{"attempt_id":"verifier","activity_type":"agent_report","new_status":"review"}]}`))
			return
		}
		if strings.HasSuffix(r.URL.Path, "/complete") {
			_ = json.NewDecoder(r.Body).Decode(&completion)
			json.NewEncoder(w).Encode(task.TaskAttempt{ID: "verifier", TaskID: filepath.Base(source), RunnerID: "fixture", Status: task.AttemptStatusSucceeded})
			return
		}
		w.Write([]byte(`{}`))
	}))
	defer api.Close()

	called := false
	adapter := &reportingBudgetAdapter{readOnlyAuditAdapter: &readOnlyAuditAdapter{t: t}, execute: func(_ context.Context, req agent.ExecutionRequest) (*agent.ExecutionResult, error) {
		called = true
		targetModules := filepath.Join(req.WorkingDir, "node_modules")
		pkgFile := filepath.Join(targetModules, "fixture-pkg", "index.js")
		if data, err := os.ReadFile(pkgFile); err != nil || string(data) != "export const v = 1;" {
			t.Fatalf("expected frontend dependencies to be present in verifier snapshot, got: %v (data: %s)", err, string(data))
		}
		expectedDigest := dependencyLockFingerprint([]byte(manifest), []byte(lock))
		if !checkLockfileWarm(req.WorkingDir, "node_modules", expectedDigest) {
			t.Fatalf("expected valid lock fingerprint in verifier snapshot")
		}
		return &agent.ExecutionResult{Status: "SUCCEEDED", ExitCode: 0}, nil
	}}
	worker := NewRunner(&config.Config{RunnerID: "fixture", RunnerWorkerRegistryDir: t.TempDir(), RunnerServerURL: api.URL, WorkspacesDir: filepath.Dir(source)}, adapter, api.Client())

	// Task has frontend scope, but instructions explicitly include UI modification prohibition
	claim := ClaimTaskResponse{
		Task: &task.Task{
			ID:                filepath.Base(source),
			Title:             "Frontend component verification",
			Instructions:      "Run the existing frontend regression tests. Do not modify UI.",
			ExecutionCategory: task.ExecutionCategoryCode,
		},
		Attempt:           &task.TaskAttempt{ID: "verifier", TaskID: filepath.Base(source), CreatedAt: time.Now()},
		Verification:      &task.VerificationIntent{IsolatedWorkspace: true, SourceCommit: commit},
		RequiresVision:    false, // Must not rely solely on RequiresVision!
		PublicationLimits: executionpolicy.PublicationPolicy{ReadOnlyRepository: true, DenyPush: true, DenyDeploy: true},
		TaskAgent:         &TaskAgentCapability{API: api.URL + "/report", Token: "fixture"},
	}
	worker.runAttempt(ctx, claim)

	if !called || completion.Status != task.AttemptStatusSucceeded {
		t.Fatalf("frontend verification did not succeed: called=%t status=%s err=%s", called, completion.Status, completion.ErrorMessage)
	}

	snapshotDir := cuangit.VerificationWorkspacePath(filepath.Dir(source), filepath.Base(source), commit)
	targetModules := filepath.Join(snapshotDir, "node_modules")
	if _, err := os.Stat(filepath.Join(targetModules, "fixture-pkg", "index.js")); err != nil {
		t.Fatalf("frontend verifier snapshot missing node_modules: %v", err)
	}
}

// TestLeaseFailureDoesNotMutateVerificationSnapshot verifies that failing to acquire
// the exclusive verification lease ensures the snapshot is never mutated.
func TestLeaseFailureDoesNotMutateVerificationSnapshot(t *testing.T) {
	ctx := context.Background()
	source, commit, _, _ := setupWarmFrontendSource(t)

	// Pre-create the snapshot directory so .git exists
	snapshotDir := cuangit.VerificationWorkspacePath(filepath.Dir(source), filepath.Base(source), commit)
	if _, err := cuangit.NewClient().PrepareVerificationSnapshot(ctx, source, snapshotDir, commit, commit); err != nil {
		t.Fatal(err)
	}

	// Verify that target does not yet have node_modules
	targetModules := filepath.Join(snapshotDir, "node_modules")
	if _, err := os.Stat(targetModules); !os.IsNotExist(err) {
		t.Fatal("target snapshot unexpectedly already has node_modules")
	}

	// Hold an exclusive lock on the verification lease lockfile in the test process
	lockPath := filepath.Join(snapshotDir, ".git", "cuan-verification.lock")
	lockFile, err := taskprocess.OpenLock(lockPath)
	if err != nil {
		t.Fatal(err)
	}
	defer lockFile.Close()
	if err := syscall.Flock(int(lockFile.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		t.Fatalf("failed to acquire test lease lock: %v", err)
	}

	var completion CompleteAttemptRequest
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if strings.HasSuffix(r.URL.Path, "/complete") {
			_ = json.NewDecoder(r.Body).Decode(&completion)
			json.NewEncoder(w).Encode(task.TaskAttempt{ID: "verifier", TaskID: filepath.Base(source), RunnerID: "fixture", Status: task.AttemptStatusFailed})
			return
		}
		w.Write([]byte(`{}`))
	}))
	defer api.Close()

	adapter := &reportingBudgetAdapter{readOnlyAuditAdapter: &readOnlyAuditAdapter{t: t}, execute: func(_ context.Context, req agent.ExecutionRequest) (*agent.ExecutionResult, error) {
		t.Fatal("agent should not be executed when verification lease acquisition fails")
		return nil, nil
	}}
	worker := NewRunner(&config.Config{RunnerID: "fixture", RunnerWorkerRegistryDir: t.TempDir(), RunnerServerURL: api.URL, WorkspacesDir: filepath.Dir(source)}, adapter, api.Client())

	timeoutCtx, cancel := context.WithTimeout(ctx, 50*time.Millisecond)
	defer cancel()

	claim := ClaimTaskResponse{
		Task: &task.Task{
			ID:                filepath.Base(source),
			Title:             "Frontend component verification",
			Instructions:      "Verify frontend visual components",
			ExecutionCategory: task.ExecutionCategoryCode,
		},
		Attempt:           &task.TaskAttempt{ID: "verifier", TaskID: filepath.Base(source), CreatedAt: time.Now()},
		Verification:      &task.VerificationIntent{IsolatedWorkspace: true, SourceCommit: commit},
		RequiresVision:    true,
		PublicationLimits: executionpolicy.PublicationPolicy{ReadOnlyRepository: true, DenyPush: true, DenyDeploy: true},
		TaskAgent:         &TaskAgentCapability{API: api.URL + "/report", Token: "fixture"},
	}
	worker.runAttempt(timeoutCtx, claim)

	if completion.Status != task.AttemptStatusFailed && completion.Status != task.AttemptStatusInterrupted {
		t.Fatalf("expected attempt to fail due to lease lock contention, got: %s (err: %s)", completion.Status, completion.ErrorMessage)
	}

	// CRITICAL: Verify that the snapshot was NOT mutated by seedWarmDependencies!
	if _, err := os.Stat(targetModules); !os.IsNotExist(err) {
		t.Fatalf("CRITICAL: snapshot was mutated despite verification lease failure; node_modules exists!")
	}
}

func TestVerificationFrontendDependenciesRequiredTable(t *testing.T) {
	dirWithScript := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dirWithScript, "scripts"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dirWithScript, "scripts", "verify-ui.sh"), []byte("#!/bin/sh\nexit 0\n"), 0755); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name       string
		claim      ClaimTaskResponse
		workingDir string
		expected   bool
	}{
		{
			name: "pure backend with unbound verify-ui.sh in repo skips frontend dependencies",
			claim: ClaimTaskResponse{
				Verification:   &task.VerificationIntent{IsolatedWorkspace: true, SourceCommit: strings.Repeat("a", 40)},
				RequiresVision: false,
				VisualRecipe:   nil,
				Task: &task.Task{
					Title:        "Backend runner optimization",
					Instructions: "1 task backend terfokus, tanpa modifikasi UI, browser screenshot, atau email.",
					DoDChecklist: []task.DoDItem{
						{Text: "1 task backend terfokus, tanpa modifikasi UI, browser screenshot, atau email."},
					},
				},
			},
			workingDir: dirWithScript,
			expected:   false,
		},
		{
			name: "pure backend with prohibition in instructions",
			claim: ClaimTaskResponse{
				Verification: &task.VerificationIntent{IsolatedWorkspace: true},
				Task: &task.Task{
					Title:        "Backend runner optimization",
					Instructions: "1 task backend terfokus, tanpa modifikasi UI, browser screenshot, atau email.",
					DoDChecklist: []task.DoDItem{
						{Text: "1 task backend terfokus, tanpa modifikasi UI, browser screenshot, atau email."},
					},
				},
			},
			expected: false,
		},
		{
			name: "pure backend with English modification prohibition",
			claim: ClaimTaskResponse{
				Verification: &task.VerificationIntent{IsolatedWorkspace: true},
				Task: &task.Task{
					Title:        "Database index optimization",
					Instructions: "Optimize queries. Do not modify UI or components.",
				},
			},
			expected: false,
		},
		{
			name: "CLI task with ambiguous tampilan in command output",
			claim: ClaimTaskResponse{
				Verification: &task.VerificationIntent{IsolatedWorkspace: true},
				Task: &task.Task{
					Title:        "Update CLI help",
					Instructions: "Perbaiki tampilan output --help pada terminal",
				},
			},
			expected: false,
		},
		{
			name: "frontend verification with UI modification prohibition in instructions",
			claim: ClaimTaskResponse{
				Verification: &task.VerificationIntent{IsolatedWorkspace: true},
				Task: &task.Task{
					Title:        "Frontend component verification",
					Instructions: "Run the existing frontend regression tests. Do not modify UI.",
				},
			},
			expected: true,
		},
		{
			name: "frontend verification in title",
			claim: ClaimTaskResponse{
				Verification: &task.VerificationIntent{IsolatedWorkspace: true},
				Task: &task.Task{
					Title:        "Verify web frontend styling and components",
					Instructions: "Run tests without modifying UI.",
				},
			},
			expected: true,
		},
		{
			name: "frontend verification in DoD checklist",
			claim: ClaimTaskResponse{
				Verification: &task.VerificationIntent{IsolatedWorkspace: true},
				Task: &task.Task{
					Title:        "Run regression checks",
					Instructions: "Do not touch UI.",
					DoDChecklist: []task.DoDItem{
						{Text: "Verifikasi frontend visual components berjalan sukses"},
					},
				},
			},
			expected: true,
		},
		{
			name: "requires vision flag true",
			claim: ClaimTaskResponse{
				Verification:   &task.VerificationIntent{IsolatedWorkspace: true},
				RequiresVision: true,
			},
			expected: true,
		},
		{
			name: "registered visual recipe",
			claim: ClaimTaskResponse{
				Verification: &task.VerificationIntent{IsolatedWorkspace: true},
				VisualRecipe: &task.VisualRecipe{Script: task.VisualVerificationScript, ScriptHash: strings.Repeat("a", 64), SourceCommit: strings.Repeat("b", 40)},
			},
			expected: true,
		},
		{
			name: "pure PHP backend with PHPUnitTargets skips frontend dependencies",
			claim: ClaimTaskResponse{
				Verification:   &task.VerificationIntent{IsolatedWorkspace: true},
				PHPUnitTargets: "tests/Feature/SmokeTest.php",
				Task: &task.Task{
					Title:        "Run PHPUnit smoke tests",
					Instructions: "Execute tests/Feature/SmokeTest.php and verify backend assertions",
				},
			},
			expected: false,
		},
		{
			name: "frontend verification retains dependencies despite PHPUnitTargets",
			claim: ClaimTaskResponse{
				Verification:   &task.VerificationIntent{IsolatedWorkspace: true},
				PHPUnitTargets: "tests/Feature/SmokeTest.php",
				Task: &task.Task{
					Title:        "Verify frontend regression",
					Instructions: "Run existing frontend tests. Do not modify UI.",
				},
			},
			expected: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			actual := verificationFrontendDependenciesRequired(&tc.claim, tc.workingDir)
			if actual != tc.expected {
				t.Fatalf("expected verificationFrontendDependenciesRequired to be %t, got %t", tc.expected, actual)
			}
		})
	}
}
