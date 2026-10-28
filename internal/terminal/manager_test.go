package terminal

import (
	"context"
	"errors"
	"github.com/buchin/cuan/internal/taskprocess"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestManager_GetOrCreateAndReuse(t *testing.T) {
	tempBase := t.TempDir()

	mgr := NewManager(ManagerConfig{
		WorkspacesDir: tempBase,
	})
	defer mgr.CloseAll()

	ctx := context.Background()
	taskID := "task-alpha-1"

	// 1. First call creates session
	sess1, err := mgr.GetOrCreateSession(ctx, taskID, ModeInteractive, 80, 24)
	if err != nil {
		t.Fatalf("GetOrCreateSession 1: %v", err)
	}

	if sess1.TaskID() != taskID {
		t.Fatalf("expected taskID %s, got %s", taskID, sess1.TaskID())
	}

	// Verify workspace directory was created
	expectedDir := filepath.Join(tempBase, taskID)
	if fi, err := os.Stat(expectedDir); err != nil || !fi.IsDir() {
		t.Fatalf("expected directory %s to exist as dir", expectedDir)
	}

	// 2. Second call reuses session
	sess2, err := mgr.GetOrCreateSession(ctx, taskID, ModeInteractive, 80, 24)
	if err != nil {
		t.Fatalf("GetOrCreateSession 2: %v", err)
	}

	if sess1.ID() != sess2.ID() {
		t.Fatalf("expected same session ID on reuse, got %s vs %s", sess1.ID(), sess2.ID())
	}

	active := mgr.ActiveSessions()
	if len(active) != 1 {
		t.Fatalf("expected 1 active session, got %d", len(active))
	}
}

func TestManager_SecurityPathTraversalPrevention(t *testing.T) {
	tempBase := t.TempDir()

	mgr := NewManager(ManagerConfig{
		WorkspacesDir: tempBase,
	})
	defer mgr.CloseAll()

	badTaskIDs := []string{
		"../escape",
		"../../etc/passwd",
		"foo/bar",
		"task;rm -rf /",
		"task$VAR",
		"task`id`",
		"",
	}

	for _, badID := range badTaskIDs {
		_, err := mgr.ResolveTaskWorkspace(badID)
		if err == nil {
			t.Errorf("SECURITY RISK: expected error for malicious task ID %q, got nil", badID)
		}
	}
}

func TestManager_CloseSession(t *testing.T) {
	tempBase := t.TempDir()

	mgr := NewManager(ManagerConfig{
		WorkspacesDir: tempBase,
	})
	defer mgr.CloseAll()

	ctx := context.Background()
	taskID := "task-to-close"

	sess, err := mgr.GetOrCreateSession(ctx, taskID, ModeInteractive, 80, 24)
	if err != nil {
		t.Fatalf("GetOrCreateSession: %v", err)
	}

	if sess.IsClosed() {
		t.Fatal("expected session to be running")
	}

	// Close session
	if err := mgr.CloseSession(taskID); err != nil {
		t.Fatalf("CloseSession: %v", err)
	}

	if !sess.IsClosed() {
		t.Fatal("expected underlying session to be closed")
	}

	_, exists := mgr.GetSession(taskID)
	if exists {
		t.Fatal("expected session to be removed from manager map")
	}
}

func TestManager_IdleSweeper(t *testing.T) {
	tempBase := t.TempDir()

	mgr := NewManager(ManagerConfig{
		WorkspacesDir: tempBase,
		IdleTimeout:   50 * time.Millisecond,
	})
	defer mgr.CloseAll()

	ctx := context.Background()
	taskID := "task-idle-sweep"

	sess, err := mgr.GetOrCreateSession(ctx, taskID, ModeWatch, 80, 24)
	if err != nil {
		t.Fatalf("GetOrCreateSession: %v", err)
	}

	// No subscribers attached, sleep past idle timeout
	time.Sleep(100 * time.Millisecond)

	cleaned := mgr.CleanIdleSessions(50 * time.Millisecond)
	if cleaned != 1 {
		t.Fatalf("expected 1 cleaned idle session, got %d", cleaned)
	}

	if !sess.IsClosed() {
		t.Fatal("expected idle session to be closed after sweep")
	}
}

func TestManager_RunTaskAgent(t *testing.T) {
	tempBase := t.TempDir()

	mgr := NewManager(ManagerConfig{
		WorkspacesDir: tempBase,
	})
	defer mgr.CloseAll()

	ctx := context.Background()
	taskID := "task-agent-run-1"

	exitCh := make(chan *TaskRunHistory, 1)
	sess, err := mgr.RunTaskAgent(ctx, TaskAgentRunConfig{
		TaskID:    taskID,
		AgentID:   "worker-flash",
		AgentName: "Antigravity Flash",
		Command:   "shell",
		Prompt:    "Test run agent",
		OnExit: func(h *TaskRunHistory) {
			exitCh <- h
		},
	})
	if err != nil {
		t.Fatalf("RunTaskAgent failed: %v", err)
	}

	if sess.TaskID() != taskID {
		t.Fatalf("expected task ID %s, got %s", taskID, sess.TaskID())
	}
	if sess.IsClosed() {
		t.Fatalf("expected session to be active")
	}

	// Verify history was initialized with running status
	histories := mgr.GetTaskHistory(taskID)
	if len(histories) != 1 {
		t.Fatalf("expected 1 history record, got %d", len(histories))
	}
	if histories[0].Status != "running" {
		t.Errorf("expected history status 'running', got %s", histories[0].Status)
	}

	// Close session to trigger process exit
	_ = sess.Close()

	select {
	case h := <-exitCh:
		if h.FinishedAt.IsZero() {
			t.Errorf("expected non-zero FinishedAt timestamp")
		}
	case <-time.After(3 * time.Second):
		t.Log("process exit monitor completed or timed out")
	}
}

func TestManager_ProjectConcurrencyLock(t *testing.T) {
	tempBase := t.TempDir()
	mgr := NewManager(ManagerConfig{WorkspacesDir: tempBase})
	defer mgr.CloseAll()

	ctx := context.Background()
	projectID := "proj-shared-123"

	sess1, err := mgr.RunTaskAgent(ctx, TaskAgentRunConfig{
		TaskID:    "task-proj-1",
		ProjectID: projectID,
		AgentID:   "worker-flash",
		AgentName: "Antigravity Flash",
		Command:   "shell",
		Prompt:    "Task 1",
	})
	if err != nil {
		t.Fatalf("first task failed: %v", err)
	}

	active, err := mgr.ProjectExecutionActive(projectID)
	if err != nil || !active {
		t.Fatalf("expected project to be active, got active=%v, err=%v", active, err)
	}

	// Attempt to run second task on the same project - should fail with taskprocess.ErrActive
	_, err = mgr.RunTaskAgent(ctx, TaskAgentRunConfig{
		TaskID:    "task-proj-2",
		ProjectID: projectID,
		AgentID:   "worker-flash",
		AgentName: "Antigravity Flash",
		Command:   "shell",
		Prompt:    "Task 2",
	})
	if err == nil {
		t.Fatal("expected second task on same project to be rejected, but it succeeded")
	}

	// Close first task session
	_ = sess1.Close()
	time.Sleep(100 * time.Millisecond)

	// Now second task should be able to run
	sess2, err := mgr.RunTaskAgent(ctx, TaskAgentRunConfig{
		TaskID:    "task-proj-2",
		ProjectID: projectID,
		AgentID:   "worker-flash",
		AgentName: "Antigravity Flash",
		Command:   "shell",
		Prompt:    "Task 2",
	})
	if err != nil {
		t.Fatalf("second task should succeed after first task completes: %v", err)
	}
	_ = sess2.Close()
}

// This fixture checks process/argv invariants, not provider authentication.
func TestS25AgentOutlivesRequestAndPromptIsNotShellCode(t *testing.T) {
	root := t.TempDir()
	marker := filepath.Join(root, "must-not-exist")
	bin := filepath.Join(root, "agy")
	script := "#!/bin/sh\n[ \"$1\" = --dangerously-skip-permissions ] || exit 21\n[ \"$2\" = --prompt-interactive ] || exit 22\n[ \"$4\" = --add-dir ] && [ \"$5\" = \"$PWD\" ] || exit 23\nprintf '%s\\n' \"$3\"\nsleep 0.2\nprintf 'PROCESS_COMPLETED\\n'\n"
	if err := os.WriteFile(bin, []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", root+string(os.PathListSeparator)+os.Getenv("PATH"))
	mgr := NewManager(ManagerConfig{WorkspacesDir: filepath.Join(root, "work")})
	defer mgr.CloseAll()
	request, cancel := context.WithCancel(context.Background())
	session, err := mgr.RunTaskAgent(request, TaskAgentRunConfig{TaskID: "invariant-task", AgentID: "fixture", Command: "agy", Prompt: "$(touch " + marker + ")"})
	if err != nil {
		cancel()
		t.Fatal(err)
	}
	cancel()
	done := make(chan struct{})
	go func() { _ = session.PTY().Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("process did not finish")
	}
	if session.PTY().ExitCode() != 0 {
		t.Fatal("HTTP request cancellation killed the agent")
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatal("prompt was evaluated as shell code")
	}
}

func TestManager_ReportedFinishIgnoresStaleInputAndAttempt(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "agy"), []byte("#!/bin/sh\nwhile read -r line; do printf '%s\\n' \"$line\"; done\n"), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", root+string(os.PathListSeparator)+os.Getenv("PATH"))
	mgr := NewManager(ManagerConfig{WorkspacesDir: filepath.Join(root, "work")})
	defer mgr.CloseAll()
	exited := make(chan *TaskRunHistory, 1)
	sess, err := mgr.RunTaskAgent(context.Background(), TaskAgentRunConfig{TaskID: "reported-task", AttemptID: "attempt-current", Command: "agy", Prompt: "interactive", OnExit: func(h *TaskRunHistory) { exited <- h }})
	if err != nil {
		t.Fatal(err)
	}
	before := time.Now().Add(-time.Second)
	sub, _, err := sess.Subscribe("writer", ModeInteractive)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = sess.WriteInput(sub.ID, []byte("later instruction\n")); err != nil {
		t.Fatal(err)
	}
	if mgr.FinishReportedSession("reported-task", "attempt-old", time.Now()) {
		t.Fatal("stopped another attempt")
	}
	if mgr.FinishReportedSession("reported-task", "attempt-current", before) {
		t.Fatal("stopped a later user turn")
	}
	if !mgr.FinishReportedSession("reported-task", "attempt-current", time.Now()) {
		t.Fatal("matching final report did not close session")
	}
	select {
	case h := <-exited:
		if h.EndReason != "agent_report" {
			t.Fatal(h.EndReason)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("reported process did not terminate")
	}
}

func TestManagerExecutionOwnershipSurvivesReconstruction(t *testing.T) {
	base := t.TempDir()
	bin := t.TempDir()
	workspace := t.TempDir()
	script := "#!/bin/sh\necho LIVE_OWNERSHIP\ntrap '' HUP TERM\nwhile :; do sleep 0.05; done\n"
	if err := os.WriteFile(filepath.Join(bin, "agy"), []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+":/usr/bin:/bin")
	first := NewManager(ManagerConfig{WorkspacesDir: base})
	defer first.CloseAll()
	ready := make(chan struct{}, 1)
	_, err := first.RunTaskAgent(context.Background(), TaskAgentRunConfig{TaskID: "ownership-task", Command: "agy", WorkDir: workspace, OnOutput: func(data []byte) {
		if strings.Contains(string(data), "LIVE_OWNERSHIP") {
			select {
			case ready <- struct{}{}:
			default:
			}
		}
	}})
	if err != nil {
		t.Fatal(err)
	}
	select {
	case <-ready:
	case <-time.After(3 * time.Second):
		t.Fatal("agent did not become active")
	}
	second := NewManager(ManagerConfig{WorkspacesDir: base})
	defer second.CloseAll()
	if active, err := second.TaskExecutionActive("ownership-task"); err != nil || !active {
		t.Fatal("reconstructed manager lost live process ownership", err)
	}
	if _, err := second.RunTaskAgent(context.Background(), TaskAgentRunConfig{TaskID: "ownership-task", Command: "agy", WorkDir: workspace}); !errors.Is(err, taskprocess.ErrActive) {
		t.Fatal("replacement overlapped live agent", err)
	}
	first.CloseAll()
	deadline := time.Now().Add(3 * time.Second)
	for {
		active, err := second.TaskExecutionActive("ownership-task")
		if err != nil {
			t.Fatal(err)
		}
		if !active {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("execution ownership did not release")
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestManagerStopsOnlyMatchingQuotaAttempt(t *testing.T) {
	bin := t.TempDir()
	if err := os.WriteFile(filepath.Join(bin, "agy"), []byte("#!/bin/sh\nwhile :; do sleep .1; done\n"), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+":/usr/bin:/bin")
	mgr := NewManager(ManagerConfig{WorkspacesDir: t.TempDir()})
	defer mgr.CloseAll()
	exits := make(chan *TaskRunHistory, 1)
	_, err := mgr.RunTaskAgent(context.Background(), TaskAgentRunConfig{TaskID: "quota-task", AttemptID: "current", Command: "agy", WorkDir: t.TempDir(), OnExit: func(h *TaskRunHistory) { exits <- h }})
	if err != nil {
		t.Fatal(err)
	}
	if mgr.StopQuotaAttempt("quota-task", "old") {
		t.Fatal("stale attempt stopped current work")
	}
	if !mgr.StopQuotaAttempt("quota-task", "current") {
		t.Fatal("quota attempt not stopped")
	}
	select {
	case h := <-exits:
		if h.EndReason != "provider_quota" || h.Status != "failed" {
			t.Fatal(h)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("quota process not terminated")
	}
	if mgr.StopQuotaAttempt("quota-task", "current") {
		t.Fatal("duplicate quota stop accepted")
	}
}

func TestRunTaskAgentAutoProvisionsOnboardingAndTrustedWorkspaces(t *testing.T) {
	root := t.TempDir()
	bin := filepath.Join(root, "bin")
	if err := os.MkdirAll(bin, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(bin, "agy"), []byte("#!/bin/sh\nexit 0\n"), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+":/usr/bin:/bin")

	homeDir := filepath.Join(root, "account_home")
	tokenPath := filepath.Join(homeDir, ".gemini", "antigravity-cli", "antigravity-oauth-token")
	if err := os.MkdirAll(filepath.Dir(tokenPath), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(tokenPath, []byte("fake-token-with-at-least-16-bytes"), 0600); err != nil {
		t.Fatal(err)
	}

	workDir := filepath.Join(root, "task_workspace")
	if err := os.MkdirAll(workDir, 0700); err != nil {
		t.Fatal(err)
	}

	mgr := NewManager(ManagerConfig{WorkspacesDir: t.TempDir()})
	defer mgr.CloseAll()

	sess, err := mgr.RunTaskAgent(context.Background(), TaskAgentRunConfig{
		TaskID:  "task-autoprovision",
		Command: "agy",
		Prompt:  "do work",
		WorkDir: workDir,
		Env:     []string{"HOME=" + homeDir},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer sess.Close()

	// Verify settings.json was provisioned with trustedWorkspaces including workDir
	settingsFile := filepath.Join(homeDir, ".gemini", "antigravity-cli", "settings.json")
	data, err := os.ReadFile(settingsFile)
	if err != nil {
		t.Fatalf("settings.json not created: %v", err)
	}
	if !strings.Contains(string(data), workDir) {
		t.Fatalf("settings.json missing workDir %s, got: %s", workDir, string(data))
	}

	// Verify onboarding.json was created with onboardingComplete true
	onboardingFile := filepath.Join(homeDir, ".gemini", "antigravity-cli", "cache", "onboarding.json")
	onboardingData, err := os.ReadFile(onboardingFile)
	if err != nil {
		t.Fatalf("onboarding.json not created: %v", err)
	}
	if !strings.Contains(string(onboardingData), `"onboardingComplete": true`) {
		t.Fatalf("onboarding.json missing onboardingComplete: true, got: %s", string(onboardingData))
	}
}

func TestEnsureResourceGuardrailEnv(t *testing.T) {
	// 1. Empty env injects default GOMAXPROCS=1, NODE_OPTIONS, GOMEMLIMIT=400MiB, and GOGC=50
	res := ensureResourceGuardrailEnv(nil)
	joined := strings.Join(res, ";")
	if !strings.Contains(joined, "GOMAXPROCS=1") {
		t.Errorf("expected GOMAXPROCS=1, got: %v", res)
	}
	if !strings.Contains(joined, "NODE_OPTIONS=--max-old-space-size=512") {
		t.Errorf("expected NODE_OPTIONS, got: %v", res)
	}
	if !strings.Contains(joined, "GOMEMLIMIT=400MiB") {
		t.Errorf("expected GOMEMLIMIT=400MiB, got: %v", res)
	}
	if !strings.Contains(joined, "GOGC=50") {
		t.Errorf("expected GOGC=50, got: %v", res)
	}

	// 2. Existing GOMAXPROCS is preserved
	custom := []string{"GOMAXPROCS=4", "FOO=bar"}
	res2 := ensureResourceGuardrailEnv(custom)
	joined2 := strings.Join(res2, ";")
	if !strings.Contains(joined2, "GOMAXPROCS=4") || strings.Contains(joined2, "GOMAXPROCS=1") {
		t.Errorf("expected custom GOMAXPROCS=4 preserved, got: %v", res2)
	}
	if !strings.Contains(joined2, "NODE_OPTIONS=--max-old-space-size=512") {
		t.Errorf("expected NODE_OPTIONS added to custom env, got: %v", res2)
	}
	if !strings.Contains(joined2, "GOMEMLIMIT=400MiB") {
		t.Errorf("expected GOMEMLIMIT added to custom env, got: %v", res2)
	}
	if !strings.Contains(joined2, "GOGC=50") {
		t.Errorf("expected GOGC added to custom env, got: %v", res2)
	}

	// 3. Existing custom variables are preserved
	customAll := []string{"GOMAXPROCS=2", "NODE_OPTIONS=--max-old-space-size=1024", "GOMEMLIMIT=800MiB", "GOGC=80"}
	res3 := ensureResourceGuardrailEnv(customAll)
	if len(res3) != 4 {
		t.Errorf("expected no additional env vars, got %v", res3)
	}
}
