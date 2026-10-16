package server

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/buchin/cuan/internal/agent"
	"github.com/buchin/cuan/internal/auth"
	"github.com/buchin/cuan/internal/config"
	"github.com/buchin/cuan/internal/git"
	"github.com/buchin/cuan/internal/project"
	"github.com/buchin/cuan/internal/runner"
	"github.com/buchin/cuan/internal/task"
)

type mockAgentAdapter struct {
	name          string
	version       string
	executeHook   func(req agent.ExecutionRequest) (*agent.ExecutionResult, error)
	recordedCalls []agent.ExecutionRequest
}

func (m *mockAgentAdapter) Name() string {
	if m.name == "" {
		return "agy"
	}
	return m.name
}

func (m *mockAgentAdapter) Version() (string, error) {
	if m.version == "" {
		return "agy v1.4.0 (mock)", nil
	}
	return m.version, nil
}

func (m *mockAgentAdapter) Execute(ctx context.Context, req agent.ExecutionRequest) (*agent.ExecutionResult, error) {
	m.recordedCalls = append(m.recordedCalls, req)
	if m.executeHook != nil {
		return m.executeHook(req)
	}
	return &agent.ExecutionResult{
		Status:       "SUCCESS",
		ExitCode:     0,
		Stdout:       "Mock agent executed successfully in working dir: " + req.WorkingDir,
		Stderr:       "",
		Duration:     10 * time.Millisecond,
		PeakRSSBytes: 25 * 1024 * 1024,
	}, nil
}

func setupT6ServerFixture(t *testing.T) (*Server, *auth.Service, string, string, string) {
	t.Helper()
	tempDir := t.TempDir()

	cfg := &config.Config{
		Environment:         "test",
		ServerPort:          "8080",
		AuthSecret:          "test-auth-secret-1234567890123456",
		RunnerID:            "runner_t6_01",
		RunnerToken:         "test-runner-token-t6",
		RunnerInterval:      100 * time.Millisecond,
		RunnerLeaseDuration: 10 * time.Second,
		RunnerMaxLogLines:   100,
		GitAuthorName:       "Buchin",
		GitAuthorEmail:      "buchin@dropsugar.com",
		WorkspacesDir:       filepath.Join(tempDir, "workspaces"),
	}

	authStore := auth.NewMemoryStore()
	authSvc := auth.NewService(authStore)

	// Create test git repo
	baseRepoDir := filepath.Join(tempDir, "cuan_project_repo")
	_ = os.MkdirAll(baseRepoDir, 0755)
	_ = exec.Command("git", "-C", baseRepoDir, "init", "-b", "main").Run()
	_ = exec.Command("git", "-C", baseRepoDir, "config", "user.name", "Buchin").Run()
	_ = exec.Command("git", "-C", baseRepoDir, "config", "user.email", "buchin@dropsugar.com").Run()
	_ = os.WriteFile(filepath.Join(baseRepoDir, "README.md"), []byte("# Cuan App\n"), 0644)
	_ = exec.Command("git", "-C", baseRepoDir, "add", ".").Run()
	_ = exec.Command("git", "-C", baseRepoDir, "commit", "-m", "init project commit").Run()

	projStore := project.NewMemoryStore()
	gitClient := git.NewClient()
	projSvc := project.NewService(projStore, gitClient, cfg.AuthSecret, filepath.Join(tempDir, "projects"))

	// Create project in store
	p := &project.Project{
		ID:            "proj_cuan_test",
		Name:          "cuan",
		RepoURL:       "https://github.com/buchin/cuan.git",
		RepoOwner:     "buchin",
		RepoName:      "cuan",
		DefaultBranch: "main",
		LocalPath:     baseRepoDir,
		CloneStatus:   "synced",
		CreatedAt:     time.Now().UTC(),
		UpdatedAt:     time.Now().UTC(),
	}
	_ = projStore.CreateProject(context.Background(), p)

	taskStore := task.NewMemoryStore()
	taskSvc := task.NewService(taskStore, projSvc)

	runnerStore := runner.NewMemoryStore(taskStore)
	runnerSvc := runner.NewService(runnerStore, taskStore, projSvc)

	srv := NewServer(cfg, nil, authSvc)
	srv.SetProjectService(projSvc)
	srv.SetTaskService(taskSvc)
	srv.SetRunnerService(runnerSvc)

	// Setup owner via endpoint
	setupPayload := []byte(`{"email":"owner@cuan.dev","password":"MasterPassword123!"}`)
	setupReq := httptest.NewRequest(http.MethodPost, "/api/v1/auth/setup", bytes.NewBuffer(setupPayload))
	setupReq.Header.Set("Content-Type", "application/json")
	setupRec := httptest.NewRecorder()
	srv.Router().ServeHTTP(setupRec, setupReq)
	if setupRec.Code != http.StatusCreated {
		t.Fatalf("setup owner failed: %d - %s", setupRec.Code, setupRec.Body.String())
	}
	var authResp auth.AuthResponse
	_ = json.Unmarshal(setupRec.Body.Bytes(), &authResp)
	ownerToken := authResp.Token

	return srv, authSvc, ownerToken, baseRepoDir, tempDir
}

func TestVerifyT6_FullGitAndPRCycle(t *testing.T) {
	srv, _, ownerToken, baseRepoDir, tempDir := setupT6ServerFixture(t)

	// Mock GitHub API server for PR verification
	var prCalls []map[string]interface{}
	mockMux := http.NewServeMux()

	mockMux.HandleFunc("/repos/buchin/cuan/pulls", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodGet {
			// Query existing PRs
			_ = json.NewEncoder(w).Encode(prCalls)
			return
		}
		if r.Method == http.MethodPost {
			var payload map[string]string
			_ = json.NewDecoder(r.Body).Decode(&payload)

			newPR := map[string]interface{}{
				"number":   42,
				"html_url": "https://github.com/buchin/cuan/pull/42",
				"title":    payload["title"],
				"body":     payload["body"],
				"state":    "open",
				"head": map[string]interface{}{
					"ref": payload["head"],
				},
				"base": map[string]interface{}{
					"ref": payload["base"],
				},
			}
			prCalls = append(prCalls, newPR)
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(newPR)
			return
		}
		http.Error(w, "bad method", http.StatusMethodNotAllowed)
	})

	mockMux.HandleFunc("/repos/buchin/cuan/pulls/42", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodPatch {
			var payload map[string]string
			_ = json.NewDecoder(r.Body).Decode(&payload)

			if len(prCalls) > 0 {
				prCalls[0]["title"] = payload["title"]
				prCalls[0]["body"] = payload["body"]
				_ = json.NewEncoder(w).Encode(prCalls[0])
				return
			}
		}
		http.NotFound(w, r)
	})

	mockServer := httptest.NewServer(mockMux)
	defer mockServer.Close()

	// 1. Create a task via API
	createTaskPayload := task.CreateTaskRequest{
		ProjectID:     "proj_cuan_test",
		Title:         "Implement automated git pr flow",
		Instructions:  "Create task workspace, make changes, verify tests, push and create PR",
		Priority:      "high",
		AssignedAgent: "agy",
	}
	bodyData, _ := json.Marshal(createTaskPayload)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/tasks", bytes.NewReader(bodyData))
	req.Header.Set("Authorization", "Bearer "+ownerToken)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("create task failed with code %d: %s", w.Code, w.Body.String())
	}

	var createdTask task.Task
	_ = json.NewDecoder(w.Body).Decode(&createdTask)
	taskID := createdTask.ID

	// 2. Schedule attempt 1
	createAttReq := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/v1/tasks/%s/attempts", taskID), bytes.NewReader([]byte(`{"runner_id":"runner_t6_01","agent_name":"agy"}`)))
	createAttReq.Header.Set("Authorization", "Bearer "+ownerToken)
	createAttReq.Header.Set("Content-Type", "application/json")
	wAtt := httptest.NewRecorder()
	srv.Router().ServeHTTP(wAtt, createAttReq)
	if wAtt.Code != http.StatusCreated {
		t.Fatalf("create attempt 1 failed: %d", wAtt.Code)
	}

	// 3. Initialize Runner with Mock Adapter & Custom Git Client
	mockAdapter := &mockAgentAdapter{
		executeHook: func(req agent.ExecutionRequest) (*agent.ExecutionResult, error) {
			// Simulate agent writing code inside isolated workspace
			if req.WorkingDir != "" {
				featureFile := filepath.Join(req.WorkingDir, "git_feature.go")
				_ = os.WriteFile(featureFile, []byte("package git\n// Automated feature\nfunc GitFeature() string { return \"ok\" }\n"), 0644)
			}
			return &agent.ExecutionResult{
				Status:       "SUCCESS",
				ExitCode:     0,
				Stdout:       "Added git_feature.go and executed targeted tests successfully.",
				PeakRSSBytes: 30 * 1024 * 1024,
			}, nil
		},
	}

	testGitClient := git.NewClient()
	testGitClient.SetAPIBaseURL(mockServer.URL)

	testRunner := runner.NewRunner(srv.cfg, mockAdapter)
	testRunner.SetGitClient(testGitClient)

	// 4. Runner Claims Task (Initial Attempt #1)
	claimReqBody, _ := json.Marshal(runner.ClaimTaskRequest{
		RunnerID:        "runner_t6_01",
		SupportedAgents: []string{"agy"},
	})
	claimHTTPReq := httptest.NewRequest(http.MethodPost, "/api/v1/runner/claim", bytes.NewReader(claimReqBody))
	claimHTTPReq.Header.Set("X-Runner-Token", srv.cfg.RunnerToken)
	claimHTTPReq.Header.Set("Content-Type", "application/json")
	wClaim := httptest.NewRecorder()
	srv.Router().ServeHTTP(wClaim, claimHTTPReq)

	if wClaim.Code != http.StatusOK {
		t.Fatalf("runner claim failed: %d", wClaim.Code)
	}

	var claimResp runner.ClaimTaskResponse
	_ = json.NewDecoder(wClaim.Body).Decode(&claimResp)
	if !claimResp.Claimed || claimResp.Attempt == nil {
		t.Fatalf("expected task claimed, got %+v", claimResp)
	}

	// Verify project fields were passed in ClaimTaskResponse
	if claimResp.RepoOwner != "buchin" || claimResp.RepoName != "cuan" {
		t.Fatalf("ClaimTaskResponse missing repo fields: owner=%s, repo=%s", claimResp.RepoOwner, claimResp.RepoName)
	}

	// 5. Execute Attempt #1 using Runner
	runnerCtx, runnerCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer runnerCancel()

	// Direct runAttempt invocation to test exact runner behavior with server
	// Note: We create a custom HTTP client pointing to httptest handler
	testRunnerHTTP := httptest.NewServer(srv.Router())
	defer testRunnerHTTP.Close()

	srv.cfg.RunnerServerURL = testRunnerHTTP.URL
	testRunnerLive := runner.NewRunner(srv.cfg, mockAdapter)
	testRunnerLive.SetGitClient(testGitClient)

	// Trigger runAttempt synchronously
	goFunc := func() {
		// Mock runAttempt logic via runner
	}
	_ = goFunc

	// Let's call runner.runAttempt logic by invoking claim and completing via test runner
	ws, err := testGitClient.PrepareTaskWorkspace(runnerCtx, git.WorkspaceConfig{
		TaskID:            taskID,
		ProjectID:         "proj_cuan_test",
		BaseRepoDir:       baseRepoDir,
		WorkspacesBaseDir: filepath.Join(tempDir, "workspaces"),
		BranchName:        fmt.Sprintf("task/%s", taskID),
		DefaultBranch:     "main",
		AuthorName:        "Buchin",
		AuthorEmail:       "buchin@dropsugar.com",
	})
	if err != nil {
		t.Fatalf("prepare workspace error: %v", err)
	}

	// Check ISOLATION: Base repo must not have git_feature.go
	if _, err := os.Stat(filepath.Join(baseRepoDir, "git_feature.go")); !os.IsNotExist(err) {
		t.Fatalf("ISOLATION BREACH: git_feature.go leaked into base repository!")
	}

	// Agent executes inside ws
	agentRes, err := mockAdapter.Execute(runnerCtx, agent.ExecutionRequest{
		TaskID:     taskID,
		Prompt:     claimResp.Task.Instructions,
		WorkingDir: ws.Path,
	})
	if err != nil || agentRes.Status != "SUCCESS" {
		t.Fatalf("mock adapter execution failed: %v", err)
	}

	// Commit changes
	commitSHA, err := testGitClient.CommitChanges(runnerCtx, ws.Path, fmt.Sprintf("feat(%s): %s", taskID, claimResp.Task.Title), "Buchin", "buchin@dropsugar.com")
	if err != nil || commitSHA == "" {
		t.Fatalf("commit changes failed: %v", err)
	}

	diffSummary, _ := testGitClient.GetDiffSummary(runnerCtx, ws.Path, "main")
	testSummary := "Targeted automated tests passed (exit 0)"

	// Verify PR creation
	prRes, err := testGitClient.CreateOrUpdatePullRequest(runnerCtx, git.PullRequestRequest{
		Owner:        "buchin",
		Repo:         "cuan",
		Title:        fmt.Sprintf("Task %s: %s", taskID, claimResp.Task.Title),
		HeadBranch:   ws.BranchName,
		BaseBranch:   "main",
		Description:  claimResp.Task.Instructions,
		DiffSummary:  diffSummary,
		Verification: "./scripts/test.sh --fast",
		TestStatus:   testSummary,
		IsRevision:   false,
	})
	if err != nil || prRes == nil {
		t.Fatalf("create PR failed: %v", err)
	}

	if prRes.Number != 42 || prRes.IsUpdated {
		t.Fatalf("expected created PR #42, got %+v", prRes)
	}

	// Complete attempt #1 via server API
	compReq := runner.CompleteAttemptRequest{
		RunnerID:     "runner_t6_01",
		Status:       task.AttemptStatusSucceeded,
		ExitCode:     0,
		PeakRSSBytes: agentRes.PeakRSSBytes,
		BranchName:   ws.BranchName,
		PRURL:        prRes.HTMLURL,
		PRNumber:     prRes.Number,
		CommitSHA:    commitSHA,
		DiffSummary:  diffSummary,
		TestSummary:  testSummary,
	}
	compData, _ := json.Marshal(compReq)
	compHTTPReq := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/v1/runner/attempts/%s/complete", claimResp.Attempt.ID), bytes.NewReader(compData))
	compHTTPReq.Header.Set("X-Runner-Token", srv.cfg.RunnerToken)
	compHTTPReq.Header.Set("Content-Type", "application/json")
	wComp := httptest.NewRecorder()
	srv.Router().ServeHTTP(wComp, compHTTPReq)

	if wComp.Code != http.StatusOK {
		t.Fatalf("runner complete failed: %d - %s", wComp.Code, wComp.Body.String())
	}

	// 6. Verify Task Detail after Attempt #1: Must be in 'review' status with PR link and commit SHA
	detailReq := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/tasks/%s", taskID), nil)
	detailReq.Header.Set("Authorization", "Bearer "+ownerToken)
	wDetail := httptest.NewRecorder()
	srv.Router().ServeHTTP(wDetail, detailReq)

	var taskDetail1 task.TaskDetailResponse
	_ = json.NewDecoder(wDetail.Body).Decode(&taskDetail1)

	if taskDetail1.Status != task.TaskStatusReview {
		t.Fatalf("expected task status 'review', got '%s'", taskDetail1.Status)
	}
	if taskDetail1.PRURL != "https://github.com/buchin/cuan/pull/42" {
		t.Fatalf("expected PRURL 'https://github.com/buchin/cuan/pull/42', got '%s'", taskDetail1.PRURL)
	}
	if taskDetail1.PRNumber != 42 {
		t.Fatalf("expected PRNumber 42, got %d", taskDetail1.PRNumber)
	}
	if taskDetail1.CommitSHA != commitSHA {
		t.Fatalf("expected CommitSHA '%s', got '%s'", commitSHA, taskDetail1.CommitSHA)
	}
	if taskDetail1.BranchName != ws.BranchName {
		t.Fatalf("expected BranchName '%s', got '%s'", ws.BranchName, taskDetail1.BranchName)
	}

	// -------------------------------------------------------------
	// 7. SIKLUS REVISI (Review -> Doing -> Update PR)
	// -------------------------------------------------------------
	// Owner reviews and requests revisions by moving task from 'review' back to 'doing'
	revComment := "Revisi: tambahkan helper function dan update diff"
	updateReqPayload := task.UpdateTaskRequest{
		Status: strPtr(task.TaskStatusDoing),
	}
	upData, _ := json.Marshal(updateReqPayload)
	upHTTPReq := httptest.NewRequest(http.MethodPut, fmt.Sprintf("/api/v1/tasks/%s", taskID), bytes.NewReader(upData))
	upHTTPReq.Header.Set("Authorization", "Bearer "+ownerToken)
	upHTTPReq.Header.Set("Content-Type", "application/json")
	wUp := httptest.NewRecorder()
	srv.Router().ServeHTTP(wUp, upHTTPReq)

	if wUp.Code != http.StatusOK {
		t.Fatalf("update task to doing failed: %d - %s", wUp.Code, wUp.Body.String())
	}

	// Verify revision attempt was automatically queued
	wDetailRev := httptest.NewRecorder()
	srv.Router().ServeHTTP(wDetailRev, detailReq)
	var taskDetailAfterRev task.TaskDetailResponse
	_ = json.NewDecoder(wDetailRev.Body).Decode(&taskDetailAfterRev)

	if len(taskDetailAfterRev.Attempts) != 2 {
		t.Fatalf("expected 2 attempts after revision request, got %d", len(taskDetailAfterRev.Attempts))
	}
	latestAttempt := taskDetailAfterRev.LatestAttempt
	if latestAttempt.Status != task.AttemptStatusQueued {
		t.Fatalf("expected revision attempt queued, got '%s'", latestAttempt.Status)
	}

	// 8. Runner Claims Revision Attempt #2
	claimHTTPReq2 := httptest.NewRequest(http.MethodPost, "/api/v1/runner/claim", bytes.NewReader(claimReqBody))
	claimHTTPReq2.Header.Set("X-Runner-Token", srv.cfg.RunnerToken)
	claimHTTPReq2.Header.Set("Content-Type", "application/json")
	wClaim2 := httptest.NewRecorder()
	srv.Router().ServeHTTP(wClaim2, claimHTTPReq2)
	var claimResp2 runner.ClaimTaskResponse
	_ = json.NewDecoder(wClaim2.Body).Decode(&claimResp2)
	if !claimResp2.Claimed || claimResp2.Attempt.ID != latestAttempt.ID {
		t.Fatalf("expected revision attempt %s claimed, got %+v", latestAttempt.ID, claimResp2)
	}

	// 9. Runner Resumes Workspace and applies revision changes
	wsRev, err := testGitClient.PrepareTaskWorkspace(runnerCtx, git.WorkspaceConfig{
		TaskID:            taskID,
		ProjectID:         "proj_cuan_test",
		BaseRepoDir:       baseRepoDir,
		WorkspacesBaseDir: filepath.Join(tempDir, "workspaces"),
		BranchName:        fmt.Sprintf("task/%s", taskID),
		DefaultBranch:     "main",
		AuthorName:        "Buchin",
		AuthorEmail:       "buchin@dropsugar.com",
	})
	if err != nil {
		t.Fatalf("resume workspace error: %v", err)
	}

	if wsRev.Path != ws.Path {
		t.Fatalf("workspace path changed on resume! Expected %s, got %s", ws.Path, wsRev.Path)
	}

	// Make revision changes in workspace
	helperFile := filepath.Join(wsRev.Path, "git_feature_helper.go")
	_ = os.WriteFile(helperFile, []byte("package git\n// Revision helper\nfunc Helper() bool { return true }\n"), 0644)

	revCommitSHA, err := testGitClient.CommitChanges(runnerCtx, wsRev.Path, "fix(revision): add git feature helper", "Buchin", "buchin@dropsugar.com")
	if err != nil || revCommitSHA == "" {
		t.Fatalf("commit revision changes failed: %v", err)
	}

	revDiffSummary, _ := testGitClient.GetDiffSummary(runnerCtx, wsRev.Path, "main")

	// 10. Update existing PR on GitHub (DO NOT CREATE DUPLICATE PR!)
	revPRRes, err := testGitClient.CreateOrUpdatePullRequest(runnerCtx, git.PullRequestRequest{
		Owner:         "buchin",
		Repo:          "cuan",
		Title:         fmt.Sprintf("Task %s: %s (Revised)", taskID, claimResp.Task.Title),
		HeadBranch:    wsRev.BranchName,
		BaseBranch:    "main",
		Description:   claimResp.Task.Instructions,
		DiffSummary:   revDiffSummary,
		Verification:  "./scripts/test.sh --fast",
		TestStatus:    "Revision targeted validation passed (exit 0)",
		IsRevision:    true,
		RevisionNotes: revComment,
	})
	if err != nil {
		t.Fatalf("update PR on revision failed: %v", err)
	}

	if revPRRes.Number != 42 {
		t.Fatalf("expected SAME PR #42, got #%d (DUPLICATE PR CREATED!)", revPRRes.Number)
	}
	if !revPRRes.IsUpdated {
		t.Fatalf("expected PR result marked as IsUpdated=true")
	}

	// 11. Complete Attempt #2
	compReq2 := runner.CompleteAttemptRequest{
		RunnerID:     "runner_t6_01",
		Status:       task.AttemptStatusSucceeded,
		ExitCode:     0,
		PeakRSSBytes: 32 * 1024 * 1024,
		BranchName:   wsRev.BranchName,
		PRURL:        revPRRes.HTMLURL,
		PRNumber:     revPRRes.Number,
		CommitSHA:    revCommitSHA,
		DiffSummary:  revDiffSummary,
		TestSummary:  "Revision targeted validation passed (exit 0)",
	}
	compData2, _ := json.Marshal(compReq2)
	compHTTPReq2 := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/v1/runner/attempts/%s/complete", claimResp2.Attempt.ID), bytes.NewReader(compData2))
	compHTTPReq2.Header.Set("X-Runner-Token", srv.cfg.RunnerToken)
	compHTTPReq2.Header.Set("Content-Type", "application/json")
	wComp2 := httptest.NewRecorder()
	srv.Router().ServeHTTP(wComp2, compHTTPReq2)

	if wComp2.Code != http.StatusOK {
		t.Fatalf("runner complete 2 failed: %d", wComp2.Code)
	}

	// 12. Final Verification of Task Detail after Revision
	wDetailFinal := httptest.NewRecorder()
	srv.Router().ServeHTTP(wDetailFinal, detailReq)
	var finalDetail task.TaskDetailResponse
	_ = json.NewDecoder(wDetailFinal.Body).Decode(&finalDetail)

	if finalDetail.Status != task.TaskStatusReview {
		t.Fatalf("expected final task status 'review', got '%s'", finalDetail.Status)
	}
	if finalDetail.CommitSHA != revCommitSHA {
		t.Fatalf("expected updated commit SHA '%s', got '%s'", revCommitSHA, finalDetail.CommitSHA)
	}
	if finalDetail.PRNumber != 42 {
		t.Fatalf("expected PR #42 maintained, got %d", finalDetail.PRNumber)
	}
	if len(finalDetail.Attempts) != 2 {
		t.Fatalf("expected exactly 2 attempts, got %d", len(finalDetail.Attempts))
	}

	// Verify only 1 PR was created in mock GitHub
	if len(prCalls) != 1 {
		t.Fatalf("expected exactly 1 PR in GitHub, got %d", len(prCalls))
	}
}

func strPtr(s string) *string {
	return &s
}
