package server

import (
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

	"github.com/go-chi/chi/v5"

	"github.com/buchin/cuan/internal/project"
)

func TestParseCommitTask(t *testing.T) {
	cases := []struct {
		msg     string
		wantID  string
		wantRef string
	}{
		{
			msg:     "Apply Task msg_41dad175-e793-4312-952b-e3f6f518b659: Pindahkan Detail Task ke SlideDrawer & Sederhanakan Menjadi Timeline",
			wantID:  "msg_41dad175-e793-4312-952b-e3f6f518b659",
			wantRef: "#41DAD1",
		},
		{
			msg:     "Apply Task task_ac6c5bdac31c: DevNull Daily Ship Log & Story Drafter",
			wantID:  "task_ac6c5bdac31c",
			wantRef: "#AC6C5B",
		},
		{
			msg:     "fix(terminal): prevent chat permission deny rules",
			wantID:  "",
			wantRef: "",
		},
		{
			msg:     "feat: normal commit without apply task prefix",
			wantID:  "",
			wantRef: "",
		},
	}

	for _, tc := range cases {
		gotID, gotRef := parseCommitTask(tc.msg)
		if gotID != tc.wantID || gotRef != tc.wantRef {
			t.Errorf("parseCommitTask(%q) = (%q, %q), want (%q, %q)", tc.msg, gotID, gotRef, tc.wantID, tc.wantRef)
		}
	}
}

func setupGitLogTestRepo(t *testing.T) string {
	dir := t.TempDir()
	runCmd := func(args ...string) {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=Buchin",
			"GIT_AUTHOR_EMAIL=buchin@example.com",
			"GIT_COMMITTER_NAME=Buchin",
			"GIT_COMMITTER_EMAIL=buchin@example.com",
		)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git command failed: %s: %v", string(out), err)
		}
	}

	runCmd("init", "-b", "main")
	_ = os.WriteFile(filepath.Join(dir, "README.md"), []byte("# Test Project\n"), 0644)
	runCmd("add", "README.md")
	runCmd("commit", "-m", "feat: initial commit")

	_ = os.WriteFile(filepath.Join(dir, "app.go"), []byte("package main\n\nfunc main() {}\n"), 0644)
	runCmd("add", "app.go")
	runCmd("commit", "-m", "Apply Task msg_41dad175-e793-4312-952b-e3f6f518b659: Pindahkan Detail Task ke SlideDrawer")

	return dir
}

func TestProjectGitLogAndCommitDetailEndpoints(t *testing.T) {
	repoDir := setupGitLogTestRepo(t)
	projID := "proj_test_git_activity_123"

	// Mock project service with memory store
	store := project.NewMemoryStore()
	proj := &project.Project{
		ID:            projID,
		Name:          "Test Git Project",
		LocalPath:     repoDir,
		DefaultBranch: "main",
		CreatedAt:     time.Now().UTC(),
		UpdatedAt:     time.Now().UTC(),
	}
	if err := store.CreateProject(context.Background(), proj); err != nil {
		t.Fatal(err)
	}
	projSvc := project.NewService(store, nil, repoDir, "")

	s := &Server{
		projectSvc: projSvc,
	}

	r := chi.NewRouter()
	r.Get("/api/v1/projects/{id}/git-log", s.handleGetProjectGitLog)
	r.Get("/api/v1/projects/{id}/commits/{sha}", s.handleGetProjectCommitDetail)

	// 1. Test GET /api/v1/projects/{id}/git-log
	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/projects/%s/git-log", projID), nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 on git-log, got %d: %s", rec.Code, rec.Body.String())
	}

	var logResp ProjectGitLogResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &logResp); err != nil {
		t.Fatalf("failed to decode git log response: %v", err)
	}

	if logResp.Total != 2 {
		t.Fatalf("expected 2 commits, got %d", logResp.Total)
	}

	latest := logResp.Commits[0]
	if latest.TaskID != "msg_41dad175-e793-4312-952b-e3f6f518b659" {
		t.Errorf("expected TaskID 'msg_41dad175-e793-4312-952b-e3f6f518b659', got %q", latest.TaskID)
	}
	if latest.TaskRef != "#41DAD1" {
		t.Errorf("expected TaskRef '#41DAD1', got %q", latest.TaskRef)
	}

	// 2. Test GET /api/v1/projects/{id}/commits/{sha}
	shaReq := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/projects/%s/commits/%s", projID, latest.SHA), nil)
	shaRec := httptest.NewRecorder()
	r.ServeHTTP(shaRec, shaReq)

	if shaRec.Code != http.StatusOK {
		t.Fatalf("expected 200 on commit detail, got %d: %s", shaRec.Code, shaRec.Body.String())
	}

	var detailResp ProjectCommitDetailResponse
	if err := json.Unmarshal(shaRec.Body.Bytes(), &detailResp); err != nil {
		t.Fatalf("failed to decode commit detail response: %v", err)
	}

	if detailResp.SHA != latest.SHA {
		t.Errorf("expected SHA %q, got %q", latest.SHA, detailResp.SHA)
	}
	if len(detailResp.Files) != 1 || detailResp.Files[0].Path != "app.go" {
		t.Errorf("expected 1 file 'app.go', got: %+v", detailResp.Files)
	}

	// 3. Test Invalid SHA validation
	badReq := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/projects/%s/commits/%s", projID, "invalid!sha"), nil)
	badRec := httptest.NewRecorder()
	r.ServeHTTP(badRec, badReq)
	if badRec.Code != http.StatusBadRequest {
		t.Errorf("expected 400 on invalid sha, got %d", badRec.Code)
	}
}

func TestProjectGitLogEmptyRepo(t *testing.T) {
	emptyDir := t.TempDir()
	initCmd := exec.Command("git", "init", "-b", "main")
	initCmd.Dir = emptyDir
	if err := initCmd.Run(); err != nil {
		t.Fatal(err)
	}

	projID := "proj_empty_repo"
	store := project.NewMemoryStore()
	proj := &project.Project{
		ID:            projID,
		Name:          "Empty Project",
		LocalPath:     emptyDir,
		DefaultBranch: "main",
		CreatedAt:     time.Now().UTC(),
		UpdatedAt:     time.Now().UTC(),
	}
	_ = store.CreateProject(context.Background(), proj)
	projSvc := project.NewService(store, nil, emptyDir, "")

	s := &Server{
		projectSvc: projSvc,
	}

	r := chi.NewRouter()
	r.Get("/api/v1/projects/{id}/git-log", s.handleGetProjectGitLog)

	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/projects/%s/git-log", projID), nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 on empty git repo, got %d: %s", rec.Code, rec.Body.String())
	}

	var logResp ProjectGitLogResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &logResp); err != nil {
		t.Fatal(err)
	}
	if logResp.Total != 0 || len(logResp.Commits) != 0 {
		t.Errorf("expected 0 commits for empty repo, got %d", logResp.Total)
	}
}
