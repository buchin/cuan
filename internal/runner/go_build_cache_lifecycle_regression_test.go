package runner

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/buchin/cuan/internal/agent"
	"github.com/buchin/cuan/internal/config"
	"github.com/buchin/cuan/internal/project"
	"github.com/buchin/cuan/internal/task"
)

type mockCompletionSimpleAdapter struct {
	scratch string
	failure bool
}

func (a *mockCompletionSimpleAdapter) Name() string             { return "mock" }
func (a *mockCompletionSimpleAdapter) Version() (string, error) { return "1.0", nil }
func (a *mockCompletionSimpleAdapter) Execute(ctx context.Context, req agent.ExecutionRequest) (*agent.ExecutionResult, error) {
	a.scratch = req.Env["GOTMPDIR"]
	if a.scratch == "" {
		return nil, fmt.Errorf("managed compiler scratch missing")
	}
	if err := os.WriteFile(filepath.Join(a.scratch, "compiler-object"), []byte("regenerable compiler work"), 0600); err != nil {
		return nil, err
	}
	if a.failure {
		return nil, context.DeadlineExceeded
	}
	return &agent.ExecutionResult{Status: "SUCCEEDED", ExitCode: 0}, nil
}

func TestRunAttemptReleasesGoBuildCacheMetadataBeforeCompletionHandled(t *testing.T) {
	for _, failure := range []bool{false, true} {
		t.Run(fmt.Sprintf("execution_failure_%v", failure), func(t *testing.T) { testRunAttemptBuildResourcesReleased(t, failure, false) })
	}
	t.Run("explicit_scratch_preserved", func(t *testing.T) { testRunAttemptBuildResourcesReleased(t, false, true) })
}

func testRunAttemptBuildResourcesReleased(t *testing.T, failure, custom bool) {
	tempDir := t.TempDir()
	workspacesDir := filepath.Join(tempDir, "workspaces")
	if err := os.MkdirAll(workspacesDir, 0700); err != nil {
		t.Fatal(err)
	}
	workingDir := filepath.Join(workspacesDir, "task-1")
	if err := os.MkdirAll(workingDir, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(workingDir, "go.mod"), []byte("module fixture\n\ngo 1.22\n"), 0600); err != nil {
		t.Fatal(err)
	}

	ownerID := "test-owner-developer"
	digest := sha256.Sum256([]byte(ownerID))
	gatePath := filepath.Join(tempDir, "go-build-runtime", fmt.Sprintf("%x", digest), ".build-cache.lock")

	var mu sync.Mutex
	var gateContentAtCompletion []byte
	var gateFlockErrAtCompletion error
	var completionReceived bool
	adapter := &mockCompletionSimpleAdapter{failure: failure}
	var scratchErrAtCompletion error

	ctx, cancelWorker := context.WithCancel(context.Background())
	defer cancelWorker()

	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		switch {
		case strings.HasSuffix(r.URL.Path, "/complete"):
			completionReceived = true
			_, scratchErrAtCompletion = os.Stat(adapter.scratch)
			gateContentAtCompletion, _ = os.ReadFile(gatePath)
			if fd, err := syscall.Open(gatePath, syscall.O_RDWR, 0600); err == nil {
				gateFlockErrAtCompletion = syscall.Flock(fd, syscall.LOCK_EX|syscall.LOCK_NB)
				syscall.Close(fd)
			}
			// Simulate supervisor cancellation directive after terminal completion acknowledged
			cancelWorker()
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(task.TaskAttempt{ID: "att-1", TaskID: "task-1", Status: task.AttemptStatusSucceeded})
		case strings.HasSuffix(r.URL.Path, "/logs"):
			w.WriteHeader(http.StatusOK)
		default:
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]any{})
		}
	}))
	defer api.Close()

	worker := NewRunner(&config.Config{
		RunnerID:                "test-runner-1",
		RunnerServerURL:         api.URL,
		RunnerWorkerRegistryDir: t.TempDir(),
		WorkspacesDir:           workspacesDir,
	}, adapter, api.Client())

	now := time.Now().UTC()
	claim := ClaimTaskResponse{
		Claimed:    true,
		OwnerID:    ownerID,
		WorkingDir: workingDir,
		Task: &task.Task{
			ID:                "task-1",
			ExecutionCategory: task.ExecutionCategoryReadOnly,
			CreatedAt:         now,
		},
		Attempt: &task.TaskAttempt{
			ID:            "att-1",
			TaskID:        "task-1",
			AttemptNumber: 1,
			CreatedAt:     now,
			StartedAt:     &now,
		},
	}

	customPath := ""
	if custom {
		customPath = t.TempDir()
		claim.Environment = "GOTMPDIR=" + customPath + "\n"
	}
	worker.runAttempt(ctx, claim)

	mu.Lock()
	defer mu.Unlock()

	if !completionReceived {
		t.Fatal("completion endpoint was not called")
	}
	if custom {
		if adapter.scratch != customPath || scratchErrAtCompletion != nil {
			t.Fatal("explicit scratch was replaced or removed", scratchErrAtCompletion)
		}
	} else if adapter.scratch == "" || !os.IsNotExist(scratchErrAtCompletion) {
		t.Fatalf("compiler scratch survived completion: %v", scratchErrAtCompletion)
	}
	if len(gateContentAtCompletion) == 0 {
		t.Fatalf("gate %s was zero bytes when completion was received by server; deferred release did not persist metadata before completion handoff", gatePath)
	}

	var meta struct {
		Version    int   `json:"version"`
		LastUsedAt int64 `json:"last_used_at"`
	}
	if err := json.Unmarshal(gateContentAtCompletion, &meta); err != nil {
		t.Fatalf("gate content is not valid json: %v (%s)", err, string(gateContentAtCompletion))
	}
	if meta.Version != 1 || meta.LastUsedAt <= 0 {
		t.Fatalf("unexpected metadata in gate: %+v", meta)
	}
	if gateFlockErrAtCompletion != nil {
		t.Fatalf("gate lock remained held during completion: %v", gateFlockErrAtCompletion)
	}
}

func TestRunNativeReleaseReleasesGoBuildCacheMetadataBeforeCompletionHandled(t *testing.T) {
	tempDir := t.TempDir()
	workspacesDir := filepath.Join(tempDir, "workspaces")
	if err := os.MkdirAll(workspacesDir, 0700); err != nil {
		t.Fatal(err)
	}
	root := filepath.Join(tempDir, "release-repo")
	if err := os.MkdirAll(root, 0700); err != nil {
		t.Fatal(err)
	}

	runGit := func(args ...string) string {
		cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v failed: %s", args, out)
		}
		return strings.TrimSpace(string(out))
	}
	runGit("init", "--initial-branch=production")
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module releasetest\n\ngo 1.22\n"), 0600); err != nil {
		t.Fatal(err)
	}
	runGit("add", ".")
	runGit("-c", "user.name=Fixture Human", "-c", "user.email=human@example.invalid", "commit", "-m", "Release Initial")
	commit := runGit("rev-parse", "HEAD")

	ownerID := "test-owner-publisher"
	digest := sha256.Sum256([]byte(ownerID))
	gatePath := filepath.Join(tempDir, "go-build-runtime", fmt.Sprintf("%x", digest), ".build-cache.lock")

	marker := filepath.Join(tempDir, "release-marker")
	manifest := "release payload"
	hash := sha256.Sum256([]byte(manifest))

	var mu sync.Mutex
	var gateContentAtCompletion []byte
	var gateFlockErrAtCompletion error
	var completionReceived bool

	ctx, cancelWorker := context.WithCancel(context.Background())
	defer cancelWorker()

	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		switch {
		case strings.HasSuffix(r.URL.Path, "/logs"):
			w.WriteHeader(http.StatusOK)
		case strings.HasSuffix(r.URL.Path, "/publication/verify"):
			_ = os.WriteFile(marker, []byte("published\n"), 0600)
			json.NewEncoder(w).Encode(map[string]any{
				"verified": true,
				"receipt": task.DeploymentReceipt{
					Version:     1,
					TaskID:      "task-rel",
					AttemptID:   "att-rel",
					CommitSHA:   commit,
					ReleaseSHA:  commit,
					Environment: "production",
					Health:      "healthy",
				},
			})
		case strings.HasSuffix(r.URL.Path, "/complete"):
			completionReceived = true
			gateContentAtCompletion, _ = os.ReadFile(gatePath)
			if fd, err := syscall.Open(gatePath, syscall.O_RDWR, 0600); err == nil {
				gateFlockErrAtCompletion = syscall.Flock(fd, syscall.LOCK_EX|syscall.LOCK_NB)
				syscall.Close(fd)
			}
			cancelWorker()
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(task.TaskAttempt{ID: "att-rel", TaskID: "task-rel", Status: task.AttemptStatusSucceeded})
		default:
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]any{})
		}
	}))
	defer api.Close()

	worker := NewRunner(&config.Config{
		RunnerID:                "native",
		RunnerToken:             "fixture",
		RunnerServerURL:         api.URL,
		RunnerWorkerRegistryDir: t.TempDir(),
		WorkspacesDir:           workspacesDir,
	}, nil, api.Client())

	now := time.Now().UTC()
	claim := ClaimTaskResponse{
		OwnerID: ownerID,
		Task:    &task.Task{ID: "task-rel", CreatedAt: now},
		Attempt: &task.TaskAttempt{
			ID:        "att-rel",
			TaskID:    "task-rel",
			RunnerID:  "native",
			CreatedAt: now,
			StartedAt: &now,
		},
		NativeRelease: &task.NativeReleaseIntent{
			SourceCommit: commit,
			TargetBranch: "production",
			ManifestHash: hex.EncodeToString(hash[:]),
		},
		NativeReleaseExecution: &NativeReleaseExecution{
			Workspace: &project.PreparedNativeReleaseWorkspace{
				Path:         root,
				SourceCommit: commit,
				TargetCommit: commit,
				Branch:       "production",
				ManifestHash: hex.EncodeToString(hash[:]),
			},
			Environment: []string{"MARKER=" + marker},
		},
	}

	worker.runAttempt(ctx, claim)

	mu.Lock()
	defer mu.Unlock()

	if !completionReceived {
		t.Fatal("completion endpoint was not called")
	}
	if len(gateContentAtCompletion) == 0 {
		t.Fatalf("publisher gate %s was zero bytes when completion was received by server; deferred release did not persist metadata before completion handoff", gatePath)
	}

	var meta struct {
		Version    int   `json:"version"`
		LastUsedAt int64 `json:"last_used_at"`
	}
	if err := json.Unmarshal(gateContentAtCompletion, &meta); err != nil {
		t.Fatalf("gate content is not valid json: %v (%s)", err, string(gateContentAtCompletion))
	}
	if meta.Version != 1 || meta.LastUsedAt <= 0 {
		t.Fatalf("unexpected metadata in publisher gate: %+v", meta)
	}
	if gateFlockErrAtCompletion != nil {
		t.Fatalf("publisher gate lock remained held during completion: %v", gateFlockErrAtCompletion)
	}
}
