package server

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/buchin/cuan/internal/db"
	"github.com/buchin/cuan/internal/project"
	"github.com/buchin/cuan/internal/task"
)

func TestFormatAttemptTerminalLogs(t *testing.T) {
	ctx := context.Background()
	s, _ := setupTestChatServer(t)
	s.taskSvc = task.NewService(task.NewMemoryStore(), nil)

	created, err := s.taskSvc.CreateTask(ctx, "owner", task.CreateTaskRequest{
		ProjectID: "p1",
		Title:     "Build Backend Pipeline",
	})
	if err != nil {
		t.Fatalf("create task failed: %v", err)
	}

	attempt, err := s.taskSvc.CreateAttempt(ctx, created.ID, "author", task.CreateAttemptRequest{
		RunnerID:  "local_terminal",
		AgentName: "agent_1",
	})
	if err != nil {
		t.Fatalf("create attempt failed: %v", err)
	}

	// Append 60 log lines
	var logRecords []task.LogRecord
	for i := 1; i <= 60; i++ {
		logRecords = append(logRecords, task.LogRecord{
			Stream: "stdout",
			Line:   fmt.Sprintf("Step %d execution log output trace", i),
		})
	}
	if err := s.taskSvc.AppendAttemptLogs(ctx, attempt.ID, logRecords, 100); err != nil {
		t.Fatalf("append logs failed: %v", err)
	}

	formatted := s.formatAttemptTerminalLogs(ctx, attempt.ID, 50)
	lines := strings.Split(formatted, "\n")
	if len(lines) != 50 {
		t.Fatalf("expected 50 lines, got %d", len(lines))
	}
	if !strings.Contains(lines[0], "Step 11") {
		t.Fatalf("expected first line to be Step 11, got: %s", lines[0])
	}
	if !strings.Contains(lines[len(lines)-1], "Step 60") {
		t.Fatalf("expected last line to be Step 60, got: %s", lines[len(lines)-1])
	}
	if strings.Contains(formatted, "Step 1 execution log") {
		t.Fatalf("older lines should be trimmed")
	}
}

func TestInboxProjectContextIncludesDeepFailureLogs(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	repo := filepath.Join(root, "repo")
	os.Mkdir(repo, 0700)
	c := exec.Command("git", "-C", repo, "init", "-q")
	_ = c.Run()
	os.WriteFile(filepath.Join(repo, "README.md"), []byte("# Test Repo\n"), 0600)
	_ = exec.Command("git", "-C", repo, "add", "README.md").Run()
	_ = exec.Command("git", "-C", repo, "-c", "user.name=test", "-c", "user.email=test@test.local", "commit", "-m", "init").Run()

	s, _ := setupTestChatServer(t)
	s.cfg.Environment = "staging"
	store := project.NewMemoryStore()
	s.projectSvc = project.NewService(store, nil, "", root)
	_ = store.CreateProject(ctx, &project.Project{ID: "p_logs", Name: "Logs Project", LocalPath: repo})

	s.taskSvc = task.NewService(task.NewMemoryStore(), nil)
	tBlocked, _ := s.taskSvc.CreateTask(ctx, "owner", task.CreateTaskRequest{ProjectID: "p_logs", Title: "Failing Migration Task"})
	attempt, _ := s.taskSvc.CreateAttempt(ctx, tBlocked.ID, "author", task.CreateAttemptRequest{RunnerID: "local_terminal", AgentName: "agent_1"})
	errLog := []task.LogRecord{
		{Stream: "stderr", Line: "FATAL: schema relation 'customers' does not exist"},
		{Stream: "stderr", Line: "migration rollback executed with error exit code 1"},
	}
	_ = s.taskSvc.AppendAttemptLogs(ctx, attempt.ID, errLog, 100)

	statusFailed := task.AttemptStatusFailed
	exitCode := 1
	var err error
	_, err = s.taskSvc.UpdateAttemptStatus(ctx, attempt.ID, "runner", task.UpdateAttemptRequest{
		Status:       statusFailed,
		ExitCode:     &exitCode,
		ErrorMessage: "FATAL: schema relation 'customers' does not exist",
	})
	if err != nil {
		t.Fatalf("update attempt status failed: %v", err)
	}

	statusBlocked := task.TaskStatusBlocked
	_, err = s.taskSvc.UpdateTask(ctx, tBlocked.ID, "user", task.UpdateTaskRequest{Status: &statusBlocked})
	if err != nil {
		t.Fatalf("update task status failed: %v", err)
	}

	room := db.GenerateUUID()
	s.projectStudioRepo.CreateRoom(ctx, &db.ProjectRoom{ID: room, ProjectID: "p_logs", Title: "SM Room", Topic: "discussion"})
	msg := db.ProjectMessage{ID: db.GenerateUUID(), RoomID: room, AuthorType: "user", Content: "Status check", CreatedAt: time.Now()}
	s.projectStudioRepo.CreateMessage(ctx, &msg)

	q := db.InboxRequest{RoomID: room, MessageID: msg.ID}
	_, snapshot := s.inboxProjectContext(ctx, q)

	if !strings.Contains(snapshot, "Active Blockers / Failed Attempts Needing Review:") {
		t.Fatalf("expected active blockers section in snapshot")
	}
	if !strings.Contains(snapshot, "FATAL: schema relation 'customers' does not exist") {
		t.Fatalf("expected deep failure log in snapshot, got: %s", snapshot)
	}
}

func TestInboxLinksResolvesTaskAndLogs(t *testing.T) {
	ctx := context.Background()
	s, _ := setupTestChatServer(t)
	s.taskSvc = task.NewService(task.NewMemoryStore(), nil)

	tFailed, _ := s.taskSvc.CreateTask(ctx, "owner", task.CreateTaskRequest{
		ProjectID: "p_test",
		Title:     "API Endpoint Deploy",
	})
	att, _ := s.taskSvc.CreateAttempt(ctx, tFailed.ID, "author", task.CreateAttemptRequest{RunnerID: "local_terminal", AgentName: "agent_runner"})
	_ = s.taskSvc.AppendAttemptLogs(ctx, att.ID, []task.LogRecord{
		{Stream: "stdout", Line: "Compiling typescript..."},
		{Stream: "stderr", Line: "Error TS2304: Cannot find name 'UndefinedModule'"},
	}, 100)

	msgText := fmt.Sprintf("Kenapa ya task /tasks/%s ini gagal terus?", tFailed.ID)
	resolved := s.resolveInternalCuanLinks(ctx, msgText, "room_1")

	if !strings.Contains(resolved, "[Referenced Internal Cuan Task:") {
		t.Fatalf("expected referenced task in resolved links, got: %s", resolved)
	}
	if !strings.Contains(resolved, "Error TS2304: Cannot find name 'UndefinedModule'") {
		t.Fatalf("expected deep failure terminal logs in resolved links, got: %s", resolved)
	}

	// Bare ID reference (e.g. "cek task_xxxx")
	bareText := fmt.Sprintf("Tolong cek %s kenapa error", tFailed.ID)
	resolvedBare := s.resolveInternalCuanLinks(ctx, bareText, "room_1")
	if !strings.Contains(resolvedBare, "[Referenced Internal Cuan Task:") {
		t.Fatalf("expected referenced task from bare ID in resolved links, got: %s", resolvedBare)
	}
}
