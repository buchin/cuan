package server

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"

	"github.com/buchin/cuan/internal/agent"
	"github.com/buchin/cuan/internal/config"
	"github.com/buchin/cuan/internal/project"
	"github.com/buchin/cuan/internal/task"
	"github.com/buchin/cuan/internal/terminal"
)

func setupTestProjectRepo(t *testing.T) (string, func(...string) string) {
	t.Helper()
	source := t.TempDir()
	run := func(args ...string) string {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", source}, args...)...)
		cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=Tester", "GIT_AUTHOR_EMAIL=tester@example.com", "GIT_COMMITTER_NAME=Tester", "GIT_COMMITTER_EMAIL=tester@example.com")
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v failed: %s (%v)", args, string(out), err)
		}
		return strings.TrimSpace(string(out))
	}
	run("init", "--initial-branch=main")
	_ = os.WriteFile(filepath.Join(source, "README.md"), []byte("# Project Root\n"), 0644)
	run("add", "README.md")
	run("commit", "-m", "Initial commit")
	run("update-ref", "refs/remotes/origin/main", "HEAD")
	return source, run
}

func TestProjectTerminal_WorktreeIsolationAndUpstreamDetection(t *testing.T) {
	source, gitRun := setupTestProjectRepo(t)
	tempBase := t.TempDir()

	cfg := &config.Config{
		WorkspacesDir: tempBase,
		AppMode:       "studio",
	}

	srv := NewServer(cfg, nil)
	projectStore := project.NewMemoryStore()
	projSvc := project.NewService(projectStore, nil, "fixture-secret", tempBase)
	srv.projectSvc = projSvc

	projID := "proj_terminal_worktree_drill"
	err := projSvc.Store().CreateProject(context.Background(), &project.Project{
		ID:            projID,
		Name:          "Lifeline Rescue Project",
		RepoURL:       "https://github.com/buchin/test-repo.git",
		LocalPath:     source,
		DefaultBranch: "main",
	})
	if err != nil {
		t.Fatalf("create project failed: %v", err)
	}

	proj, err := projSvc.Get(context.Background(), projID)
	if err != nil {
		t.Fatalf("get project failed: %v", err)
	}

	// 1. Prepare Dedicated Project Terminal Workspace
	workDir, err := projSvc.ProjectTerminalWorkspace(context.Background(), projID, proj)
	if err != nil {
		t.Fatalf("ProjectTerminalWorkspace failed: %v", err)
	}

	if !strings.Contains(workDir, "terminal-workspaces") {
		t.Fatalf("expected workDir to be inside terminal-workspaces, got %s", workDir)
	}

	// Verify dedicated branch name
	out, err := exec.Command("git", "-C", workDir, "symbolic-ref", "--short", "HEAD").Output()
	if err != nil {
		t.Fatalf("get branch failed: %v", err)
	}
	expectedBranch := "cuan/terminal-" + projID
	if strings.TrimSpace(string(out)) != expectedBranch {
		t.Fatalf("expected branch %s, got %s", expectedBranch, strings.TrimSpace(string(out)))
	}

	// 2. Write uncommitted scratch files in dedicated terminal worktree
	scratchFile := filepath.Join(workDir, "rescue_scratch.py")
	if err := os.WriteFile(scratchFile, []byte("print('uncommitted rescue logic')\n"), 0644); err != nil {
		t.Fatalf("write scratch file failed: %v", err)
	}

	// 3. Inspect initial upstream status (should be in sync)
	ts := httptest.NewServer(srv.Router())
	defer ts.Close()

	req, _ := http.NewRequest("GET", ts.URL+"/api/v1/projects/"+projID+"/terminal/status", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET terminal status failed: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var initialStatus struct {
		Upstream GitIntegrityReport `json:"upstream"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&initialStatus); err != nil {
		t.Fatalf("decode status failed: %v", err)
	}
	resp.Body.Close()

	if initialStatus.Upstream.BehindCount != 0 {
		t.Fatalf("expected behind_count 0 initially, got %d", initialStatus.Upstream.BehindCount)
	}

	// 4. Simulate a new commit pushed to upstream (origin/main)
	_ = os.WriteFile(filepath.Join(source, "feature.txt"), []byte("new upstream feature\n"), 0644)
	gitRun("add", "feature.txt")
	gitRun("commit", "-m", "Remote upstream commit")
	gitRun("update-ref", "refs/remotes/origin/main", "HEAD")

	// 5. Trigger non-destructive sync-upstream
	syncReq, _ := http.NewRequest("POST", ts.URL+"/api/v1/projects/"+projID+"/terminal/sync-upstream", nil)
	syncResp, err := http.DefaultClient.Do(syncReq)
	if err != nil {
		t.Fatalf("POST sync-upstream failed: %v", err)
	}
	if syncResp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 from sync-upstream, got %d", syncResp.StatusCode)
	}
	var syncData struct {
		Status   string             `json:"status"`
		Upstream GitIntegrityReport `json:"upstream"`
	}
	if err := json.NewDecoder(syncResp.Body).Decode(&syncData); err != nil {
		t.Fatalf("decode sync response failed: %v", err)
	}
	syncResp.Body.Close()

	// 6. Verify upstream tracking detected the new upstream commit
	if syncData.Upstream.BehindCount != 1 {
		t.Fatalf("expected behind_count 1 after upstream commit, got %d (sync_status: %s)", syncData.Upstream.BehindCount, syncData.Upstream.SyncStatus)
	}
	if !strings.Contains(syncData.Upstream.SyncStatus, "behind by 1") {
		t.Fatalf("expected sync_status to indicate 'behind by 1', got %s", syncData.Upstream.SyncStatus)
	}

	// 7. Verify uncommitted scratch file was preserved safely without auto-overwrite
	scratchBytes, err := os.ReadFile(scratchFile)
	if err != nil {
		t.Fatalf("scratch file was lost or deleted: %v", err)
	}
	if !strings.Contains(string(scratchBytes), "uncommitted rescue logic") {
		t.Fatalf("scratch file content was corrupted: %s", string(scratchBytes))
	}
}

func TestProjectTerminal_WebSocketPersistenceAndReattach(t *testing.T) {
	source, _ := setupTestProjectRepo(t)
	tempBase := t.TempDir()

	cfg := &config.Config{
		WorkspacesDir: tempBase,
		AppMode:       "studio",
	}

	srv := NewServer(cfg, nil)
	projectStore := project.NewMemoryStore()
	projSvc := project.NewService(projectStore, nil, "fixture-secret", tempBase)
	srv.projectSvc = projSvc

	projID := "proj_persistent_ws_drill"
	_ = projSvc.Store().CreateProject(context.Background(), &project.Project{
		ID:            projID,
		Name:          "Persistence WS Project",
		RepoURL:       "https://github.com/buchin/test-repo.git",
		LocalPath:     source,
		DefaultBranch: "main",
	})

	ts := httptest.NewServer(srv.Router())
	defer ts.Close()

	wsURL := "ws" + strings.TrimPrefix(ts.URL, "http") + "/api/v1/projects/" + projID + "/terminal"

	// 1. Client 1 connects to persistent project terminal
	conn1, resp, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("dial project terminal failed: %v", err)
	}
	if resp.StatusCode != http.StatusSwitchingProtocols {
		t.Fatalf("expected 101, got %d", resp.StatusCode)
	}

	// Read initial connected handshake message
	_, payload, err := conn1.ReadMessage()
	if err != nil {
		t.Fatalf("read handshake from conn1 failed: %v", err)
	}
	var hs terminal.TerminalMessage
	_ = json.Unmarshal(payload, &hs)
	if hs.Type != "connected" {
		t.Fatalf("expected connected type, got %s", hs.Type)
	}

	// Send echo command from Client 1
	_ = conn1.WriteJSON(terminal.TerminalMessage{
		Type: "input",
		Data: "echo 'PERSISTENCE_DRILL_TOKEN_1'\n",
	})

	// Read binary PTY output until token appears
	var captured strings.Builder
	timeout := time.After(3 * time.Second)
	found := false
	for !found {
		select {
		case <-timeout:
			t.Fatalf("timed out waiting for echo from conn1, got: %s", captured.String())
		default:
			_ = conn1.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
			msgType, data, err := conn1.ReadMessage()
			if err != nil {
				if strings.Contains(err.Error(), "timeout") {
					continue
				}
				t.Fatalf("conn1.ReadMessage error: %v", err)
			}
			if msgType == websocket.BinaryMessage {
				captured.Write(data)
				if strings.Contains(captured.String(), "PERSISTENCE_DRILL_TOKEN_1") {
					found = true
				}
			}
		}
	}

	// 2. Client 1 disconnects (closes browser tab)
	_ = conn1.Close()

	// Verify that the backend persistent session is still alive in terminalManager or via SocketBridge!
	sess, exists := srv.terminalManager.GetProjectSession(projID)
	if (!exists || sess == nil || sess.IsClosed()) && !terminal.ProjectSocketExists(projID) {
		t.Fatal("expected project terminal session to remain running in background after client disconnect")
	}

	// 3. Client 2 reconnects (re-attaches)
	conn2, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("re-attach dial failed: %v", err)
	}
	defer conn2.Close()

	// Read handshake
	_, _, err = conn2.ReadMessage()
	if err != nil {
		t.Fatalf("read handshake from conn2 failed: %v", err)
	}

	// Next frame should be binary scrollback history containing PERSISTENCE_DRILL_TOKEN_1
	msgType, histData, err := conn2.ReadMessage()
	if err != nil {
		t.Fatalf("read history buffer from conn2 failed: %v", err)
	}
	if msgType != websocket.BinaryMessage || !strings.Contains(string(histData), "PERSISTENCE_DRILL_TOKEN_1") {
		t.Fatalf("expected history buffer to contain 'PERSISTENCE_DRILL_TOKEN_1', got type=%d data=%q", msgType, string(histData))
	}

	// Send new command to reconnected session
	_ = conn2.WriteJSON(terminal.TerminalMessage{
		Type: "input",
		Data: "echo 'PERSISTENCE_DRILL_TOKEN_2'\n",
	})

	var captured2 strings.Builder
	deadline2 := time.Now().Add(4 * time.Second)
	_ = conn2.SetReadDeadline(deadline2)
	found2 := false
	for !found2 && time.Now().Before(deadline2) {
		msgType, data, err := conn2.ReadMessage()
		if err != nil {
			t.Fatalf("conn2.ReadMessage error: %v (captured: %q)", err, captured2.String())
		}
		if msgType == websocket.BinaryMessage {
			captured2.Write(data)
			if strings.Contains(captured2.String(), "PERSISTENCE_DRILL_TOKEN_2") {
				found2 = true
			}
		}
	}
	if !found2 {
		t.Fatalf("timed out waiting for second echo, got: %s", captured2.String())
	}

	// Clean up project terminal session explicitly via DELETE endpoint
	delReq, _ := http.NewRequest("DELETE", ts.URL+"/api/v1/projects/"+projID+"/terminal", nil)
	delResp, err := http.DefaultClient.Do(delReq)
	if err != nil || delResp.StatusCode != http.StatusOK {
		t.Fatalf("DELETE /api/v1/projects/{id}/terminal failed: %v (code %d)", err, delResp.StatusCode)
	}

	if _, exists := srv.terminalManager.GetProjectSession(projID); exists {
		t.Fatal("expected project session to be removed after explicit DELETE")
	}
}

func TestProjectTerminal_BrainFailoverRotation(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	srv, token := setupTestChatServer(t)
	owner, err := srv.authSvc.ValidateToken(context.Background(), token)
	if err != nil {
		t.Fatalf("failed to validate token: %v", err)
	}

	tempBase := t.TempDir()
	srv.cfg.WorkspacesDir = tempBase
	projectStore := project.NewMemoryStore()
	projSvc := project.NewService(projectStore, nil, "fixture-secret", tempBase)
	srv.projectSvc = projSvc

	projID := "proj_failover_drill"
	_ = projSvc.Store().CreateProject(context.Background(), &project.Project{
		ID:        projID,
		Name:      "Brain Failover Project",
		LocalPath: tempBase,
	})

	// Pre-create persistent terminal session
	sess, err := srv.terminalManager.GetOrCreateProjectSession(context.Background(), projID, tempBase, "/bin/sh", 80, 24)
	if err != nil {
		t.Fatalf("failed to create session: %v", err)
	}
	defer func() { _ = srv.terminalManager.CloseAll() }()

	ts := httptest.NewServer(srv.Router())
	defer ts.Close()

	// Helper to create working mock brain account
	makeBrain := func(label, model string) *agent.ProviderAccount {
		t.Helper()
		p, err := agent.CreateProviderAccount(owner.ID, label)
		if err != nil {
			t.Fatalf("CreateProviderAccount failed: %v", err)
		}
		home, _ := agent.ProviderAccountHome(owner.ID, p.ID)
		_ = os.MkdirAll(agent.AntigravityConfigDirPath(home), 0700)
		_ = os.WriteFile(agent.AntigravityCredentialsFilePath(home), []byte(`{"access_token":"fixture-valid-test-token"}`), 0600)

		file := filepath.Join(agent.ConfigHome(), ".local", "share", "cuan", "accounts", p.ID, "account.json")
		data, err := os.ReadFile(file)
		if err == nil {
			var meta map[string]interface{}
			if err := json.Unmarshal(data, &meta); err == nil {
				meta["state"] = "unknown"
				if model != "" {
					meta["model_id"] = model
				}
				updated, _ := json.Marshal(meta)
				_ = os.WriteFile(file, updated, 0600)
			}
		}
		p, _ = agent.GetProviderAccount(owner.ID, p.ID)
		return p
	}

	acct1 := makeBrain("primary-limited", "gemini-1.5-pro")
	acct2 := makeBrain("backup-healthy", "gemini-1.5-flash")

	// Mark primary as quota limited
	if err := agent.RecordProviderAccountQuotaWithDuration(owner.ID, acct1.ID, 15*time.Minute, ""); err != nil {
		t.Fatalf("RecordProviderAccountQuotaWithDuration failed: %v", err)
	}

	// Call rotate-brain endpoint with authenticated session
	rotateReq, _ := http.NewRequest("POST", ts.URL+"/api/v1/projects/"+projID+"/terminal/rotate-brain", nil)
	rotateReq.Header.Set("Authorization", "Bearer "+token)

	rotateResp, err := http.DefaultClient.Do(rotateReq)
	if err != nil {
		t.Fatalf("rotate-brain request failed: %v", err)
	}
	defer rotateResp.Body.Close()

	if rotateResp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 from rotate-brain, got %d", rotateResp.StatusCode)
	}

	var rotateData struct {
		Status    string `json:"status"`
		AccountID string `json:"account_id"`
		ModelID   string `json:"model_id"`
	}
	if err := json.NewDecoder(rotateResp.Body).Decode(&rotateData); err != nil {
		t.Fatalf("decode rotate-brain response failed: %v", err)
	}

	if rotateData.Status != "rotated" {
		t.Fatalf("expected status 'rotated', got %s", rotateData.Status)
	}
	if rotateData.AccountID != acct2.ID {
		t.Fatalf("expected rotation to %s, got %s", acct2.ID, rotateData.AccountID)
	}

	// Verify that the bash PTY session is STILL alive!
	if sess.IsClosed() {
		t.Fatal("terminal session was closed or interrupted during brain failover rotation")
	}
}

func TestProjectTerminal_DedicatedTaskWebSocketAutoRoute(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	srv, token := setupTestChatServer(t)

	tempBase := t.TempDir()
	srv.cfg.WorkspacesDir = tempBase
	projectStore := project.NewMemoryStore()
	projSvc := project.NewService(projectStore, nil, "fixture-secret", tempBase)
	srv.projectSvc = projSvc

	projID := fmt.Sprintf("proj_auto_route_%d", time.Now().UnixNano())
	_ = terminal.TerminateProjectWorker(projID)
	defer terminal.TerminateProjectWorker(projID)

	_ = projSvc.Store().CreateProject(context.Background(), &project.Project{
		ID:        projID,
		Name:      "AutoRoute Project",
		LocalPath: tempBase,
	})

	// Create task directly in task store without starting any runner
	createdTask, err := srv.taskSvc.CreateTask(context.Background(), "owner", task.CreateTaskRequest{
		Title:        "Terminal · AutoRoute Project",
		ProjectID:    projID,
		ProjectIDs:   []string{projID},
		Instructions: "Interactive terminal session",
	})
	if err != nil {
		t.Fatalf("failed to create task: %v", err)
	}

	ts := httptest.NewServer(srv.Router())
	defer ts.Close()

	// Connect to /api/v1/tasks/{id}/terminal
	wsURL := "ws" + strings.TrimPrefix(ts.URL, "http") + "/api/v1/tasks/" + createdTask.ID + "/terminal"

	dialer := websocket.DefaultDialer
	headers := http.Header{}
	headers.Set("Authorization", "Bearer "+token)

	conn, resp, err := dialer.Dial(wsURL, headers)
	if err != nil {
		if resp != nil {
			body, _ := io.ReadAll(resp.Body)
			t.Fatalf("dial project terminal ws via task route failed: %v, status: %d, body: %s", err, resp.StatusCode, string(body))
		}
		t.Fatalf("dial project terminal ws via task route failed: %v", err)
	}
	defer conn.Close()

	if resp.StatusCode != http.StatusSwitchingProtocols {
		t.Fatalf("expected 101 Switching Protocols, got %d", resp.StatusCode)
	}

	// Send binary command: 0x00 echo 'PROJECT_TERMINAL_AUTO_ROUTE_OK'\n
	cmd := append([]byte{0x00}, []byte("echo 'PROJECT_TERMINAL_AUTO_ROUTE_OK'\n")...)
	if err := conn.WriteMessage(websocket.BinaryMessage, cmd); err != nil {
		t.Fatalf("write command failed: %v", err)
	}

	found := false
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		_ = conn.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
		_, payload, err := conn.ReadMessage()
		if err != nil {
			var netErr net.Error
			if errors.As(err, &netErr) && netErr.Timeout() {
				continue
			}
			break
		}
		if strings.Contains(string(payload), "PROJECT_TERMINAL_AUTO_ROUTE_OK") {
			found = true
			break
		}
	}

	if !found {
		t.Fatal("expected PROJECT_TERMINAL_AUTO_ROUTE_OK in output stream")
	}

	// Verify task detail status is doing and running
	detail, err := srv.taskSvc.GetTaskDetail(context.Background(), createdTask.ID)
	if err != nil {
		t.Fatalf("GetTaskDetail failed: %v", err)
	}
	if detail.Status != task.TaskStatusDoing {
		t.Fatalf("expected task status doing, got %s", detail.Status)
	}
	if detail.LatestAttempt == nil || detail.LatestAttempt.Status != task.AttemptStatusRunning {
		t.Fatalf("expected attempt status running, got %v", detail.LatestAttempt)
	}
}

func TestProjectTerminal_DedicatedTaskDoneAutoReopensOnWebSocketConnect(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	srv, token := setupTestChatServer(t)

	tempBase := t.TempDir()
	srv.cfg.WorkspacesDir = tempBase
	projectStore := project.NewMemoryStore()
	projSvc := project.NewService(projectStore, nil, "fixture-secret", tempBase)
	srv.projectSvc = projSvc

	projID := fmt.Sprintf("proj_reopen_%d", time.Now().UnixNano())
	_ = terminal.TerminateProjectWorker(projID)
	defer terminal.TerminateProjectWorker(projID)

	_ = projSvc.Store().CreateProject(context.Background(), &project.Project{
		ID:        projID,
		Name:      "Reopen Project",
		LocalPath: tempBase,
	})

	// Create task directly in task store
	createdTask, err := srv.taskSvc.CreateTask(context.Background(), "owner", task.CreateTaskRequest{
		Title:        "Terminal · Reopen Project",
		ProjectID:    projID,
		ProjectIDs:   []string{projID},
		Instructions: "Interactive terminal session",
	})
	if err != nil {
		t.Fatalf("failed to create task: %v", err)
	}

	// Mark task as DONE in store (simulating an exit code 0 or completed command attempt)
	tObj, err := srv.taskSvc.Store().GetTask(context.Background(), createdTask.ID)
	if err != nil {
		t.Fatalf("failed to get task: %v", err)
	}
	tObj.Status = task.TaskStatusDone
	if err := srv.taskSvc.Store().UpdateTask(context.Background(), tObj); err != nil {
		t.Fatalf("failed to mark task done in store: %v", err)
	}

	ts := httptest.NewServer(srv.Router())
	defer ts.Close()

	// Connect to /api/v1/tasks/{id}/terminal
	wsURL := "ws" + strings.TrimPrefix(ts.URL, "http") + "/api/v1/tasks/" + createdTask.ID + "/terminal"

	dialer := websocket.DefaultDialer
	headers := http.Header{}
	headers.Set("Authorization", "Bearer "+token)

	conn, resp, err := dialer.Dial(wsURL, headers)
	if err != nil {
		if resp != nil {
			body, _ := io.ReadAll(resp.Body)
			t.Fatalf("dial project terminal ws on done task failed: %v, status: %d, body: %s", err, resp.StatusCode, string(body))
		}
		t.Fatalf("dial project terminal ws on done task failed: %v", err)
	}
	defer conn.Close()

	if resp.StatusCode != http.StatusSwitchingProtocols {
		t.Fatalf("expected 101 Switching Protocols, got %d", resp.StatusCode)
	}

	// Verify task detail status was auto-reactivated to doing
	detail, err := srv.taskSvc.GetTaskDetail(context.Background(), createdTask.ID)
	if err != nil {
		t.Fatalf("GetTaskDetail failed: %v", err)
	}
	if detail.Status != task.TaskStatusDoing {
		t.Fatalf("expected task status doing, got %s", detail.Status)
	}
}

func TestProjectTerminal_DedicatedTaskBrainTargetedLaunch(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	srv, token := setupTestChatServer(t)

	owner, err := srv.authSvc.ValidateToken(context.Background(), token)
	if err != nil {
		t.Fatalf("failed to validate token: %v", err)
	}

	tempBase := t.TempDir()
	srv.cfg.WorkspacesDir = tempBase
	projectStore := project.NewMemoryStore()
	projSvc := project.NewService(projectStore, nil, "fixture-secret", tempBase)
	srv.projectSvc = projSvc

	projID := fmt.Sprintf("proj_brain_launch_%d", time.Now().UnixNano())
	_ = terminal.TerminateProjectWorker(projID)
	defer terminal.TerminateProjectWorker(projID)

	_ = projSvc.Store().CreateProject(context.Background(), &project.Project{
		ID:        projID,
		Name:      "Brain Launch Project",
		LocalPath: tempBase,
	})

	// Configure a test provider account
	acct, err := agent.CreateProviderAccount(owner.ID, "Google Antigravity Brain")
	if err != nil {
		t.Fatalf("CreateProviderAccount failed: %v", err)
	}
	home, _ := agent.ProviderAccountHome(owner.ID, acct.ID)
	_ = os.MkdirAll(agent.AntigravityConfigDirPath(home), 0700)
	_ = os.WriteFile(agent.AntigravityCredentialsFilePath(home), []byte(`{"access_token":"fixture-valid-test-token"}`), 0600)

	// Create task directly in task store
	createdTask, err := srv.taskSvc.CreateTask(context.Background(), owner.ID, task.CreateTaskRequest{
		Title:        "Terminal · Google Antigravity Brain",
		ProjectID:    projID,
		ProjectIDs:   []string{projID},
		Instructions: "Interactive AI Brain terminal session",
	})
	if err != nil {
		t.Fatalf("failed to create task: %v", err)
	}

	ts := httptest.NewServer(srv.Router())
	defer ts.Close()

	// Launch via POST /api/v1/tasks/{id}/run-agent with account_id and runner_id=local_terminal
	runPayload := map[string]any{
		"request_id": "80000000-0000-4000-8000-000000000001",
		"runner_id":  "local_terminal",
		"account_id": acct.ID,
	}
	bodyBytes, _ := json.Marshal(runPayload)

	req, err := http.NewRequest(http.MethodPost, ts.URL+"/api/v1/tasks/"+createdTask.ID+"/run-agent", bytes.NewReader(bodyBytes))
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("run-agent request failed: %v", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d: %s", resp.StatusCode, string(respBody))
	}

	var res map[string]any
	if err := json.Unmarshal(respBody, &res); err != nil {
		t.Fatalf("failed to unmarshal run-agent response: %v", err)
	}

	if res["status"] != "running" {
		t.Fatalf("expected status 'running', got %v", res["status"])
	}
}

func TestProjectTerminal_AgentReportDoesNotCloseInteractiveSession(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	srv, _ := setupTestChatServer(t)

	// Create project in store
	projID := "proj_term_persistent_test"
	err := srv.projectSvc.Store().CreateProject(context.Background(), &project.Project{
		ID:        projID,
		Name:      "Persistent Project",
		LocalPath: t.TempDir(),
	})
	if err != nil {
		t.Fatalf("failed to create project: %v", err)
	}

	// Create task directly in task store
	createdTask, err := srv.taskSvc.CreateTask(context.Background(), "owner", task.CreateTaskRequest{
		Title:        "Terminal · Persistent Project",
		ProjectID:    projID,
		ProjectIDs:   []string{projID},
		Instructions: "Interactive terminal session",
	})
	if err != nil {
		t.Fatalf("failed to create task: %v", err)
	}

	// Set task status to doing
	doing := task.TaskStatusDoing
	_, err = srv.taskSvc.UpdateTask(context.Background(), createdTask.ID, "owner", task.UpdateTaskRequest{
		Status: &doing,
	})
	if err != nil {
		t.Fatalf("failed to set task status to doing: %v", err)
	}

	// Create attempt
	att, err := srv.taskSvc.CreateAttempt(context.Background(), createdTask.ID, "runner", task.CreateAttemptRequest{
		RunnerID:  "local_terminal",
		AgentName: "agent_cuan",
	})
	if err != nil {
		t.Fatalf("failed to create attempt: %v", err)
	}

	// Start attempt
	_, err = srv.taskSvc.UpdateAttemptStatus(context.Background(), att.ID, "runner", task.UpdateAttemptRequest{
		Status: task.AttemptStatusRunning,
	})
	if err != nil {
		t.Fatalf("failed to set attempt running: %v", err)
	}

	// Issue task agent grant
	_, _, agentToken, cleanup, err := srv.issueTaskAgent(context.Background(), createdTask.ID, att.ID, "agent_cuan")
	if err != nil {
		t.Fatalf("issueTaskAgent failed: %v", err)
	}
	defer cleanup()

	// Submit an agent report with state "done"
	payload, _ := json.Marshal(map[string]any{
		"state":         "done",
		"summary":       "Finished inspection turn",
		"complete_note": "Observed live health and state",
		"dod_checklist": []map[string]any{
			{"text": "Verify the result", "completed": true, "evidence": "Live output verified"},
		},
	})
	req := httptest.NewRequest("POST", "/api/v1/task-agent/"+createdTask.ID, bytes.NewReader(payload))
	req.Header.Set("Authorization", "Bearer "+agentToken)
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("agent report failed with code %d: %s", w.Code, w.Body.String())
	}

	// ReconcileLocalTask should NOT change task status to done
	err = srv.taskSvc.ReconcileLocalTask(context.Background(), createdTask.ID, att.ID)
	if err != nil {
		t.Fatalf("ReconcileLocalTask failed: %v", err)
	}

	detail, err := srv.taskSvc.GetTaskDetail(context.Background(), createdTask.ID)
	if err != nil {
		t.Fatalf("GetTaskDetail failed: %v", err)
	}

	if detail.Status == task.TaskStatusDone {
		t.Fatalf("expected dedicated terminal task status to remain active (not 'done'), got %s", detail.Status)
	}
}

func TestProjectTerminalGitCredentialsEndpoint(t *testing.T) {
	srv, _, _ := setupTestServer(t)
	ctx := context.Background()

	// Seed shared repository token resolver on projectSvc
	srv.projectSvc.SetSharedRepositoryAccess(func(context.Context) (string, error) {
		return "ghp_live_owner_token_123", nil
	})
	projID := "proj_terminal_git_creds"
	if err := srv.projectSvc.Store().CreateProject(ctx, &project.Project{
		ID:            projID,
		Name:          "Terminal Git Creds Test",
		LocalPath:     t.TempDir(),
		RepoURL:       "https://github.com/example/repo",
		DefaultBranch: "dev",
	}); err != nil {
		t.Fatalf("CreateProject failed: %v", err)
	}

	env := srv.projectTerminalGitEnv(nil, projID, "owner_1")
	var askpassToken string
	for _, entry := range env {
		if strings.HasPrefix(entry, "CUAN_GIT_ASKPASS_TOKEN=") {
			askpassToken = strings.TrimPrefix(entry, "CUAN_GIT_ASKPASS_TOKEN=")
		}
		if strings.Contains(entry, "ghp_live_owner_token_123") {
			t.Fatalf("raw GitHub token must never appear in terminal env: %s", entry)
		}
	}
	if askpassToken == "" {
		t.Fatalf("expected CUAN_GIT_ASKPASS_TOKEN in projectTerminalGitEnv, got %v", env)
	}

	// 1. Valid scoped token returns the repository token
	req := httptest.NewRequest("GET", "/api/v1/projects/"+projID+"/terminal/git-credentials", nil)
	req.Header.Set("Authorization", "Bearer "+askpassToken)
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d: %s", w.Code, w.Body.String())
	}
	var resp struct {
		Token string `json:"token"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil || resp.Token != "ghp_live_owner_token_123" {
		t.Fatalf("unexpected credential response: %+v err=%v", resp, err)
	}
	if cc := w.Header().Get("Cache-Control"); cc != "no-store" {
		t.Fatalf("expected Cache-Control: no-store, got %q", cc)
	}

	// 2. Wrong project ID must be rejected
	reqWrongProj := httptest.NewRequest("GET", "/api/v1/projects/other-proj/terminal/git-credentials", nil)
	reqWrongProj.Header.Set("Authorization", "Bearer "+askpassToken)
	wWrong := httptest.NewRecorder()
	srv.Router().ServeHTTP(wWrong, reqWrongProj)
	if wWrong.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for mismatched project, got %d", wWrong.Code)
	}

	// 3. Missing or tampered token must be rejected
	reqBad := httptest.NewRequest("GET", "/api/v1/projects/"+projID+"/terminal/git-credentials", nil)
	reqBad.Header.Set("Authorization", "Bearer tampered.token")
	wBad := httptest.NewRecorder()
	srv.Router().ServeHTTP(wBad, reqBad)
	if wBad.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for tampered token, got %d", wBad.Code)
	}
}
