package server

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/buchin/cuan/internal/project"
	"github.com/buchin/cuan/internal/task"
)

func TestRunningReleaseBelongsOnlyToMatchingProject(t *testing.T) {
	for _, scenario := range []string{"matching fork", "different project", "unknown build", "invalid source", "invalid commit", "development"} {
		t.Run(scenario, func(t *testing.T) {
			s, _, _, _ := inboxFixture(t)
			s.cfg.Environment = "production"
			s.cfg.SourceRepository = "example/fork"
			s.cfg.CommitSHA = strings.Repeat("a", 40)
			store := project.NewMemoryStore()
			s.projectSvc = project.NewService(store, nil, "", t.TempDir())
			p := &project.Project{ID: "project", Name: "Fork", RepoURL: "https://github.com/Example/Fork.git"}
			switch scenario {
			case "different project":
				p.RepoURL = "https://github.com/example/another"
			case "unknown build":
				s.cfg.SourceRepository = ""
			case "invalid source":
				s.cfg.SourceRepository = "test-user:synthetic-secret@example/fork"
			case "invalid commit":
				s.cfg.CommitSHA = "unversioned"
			case "development":
				s.cfg.Environment = "development"
			}
			if err := store.CreateProject(context.Background(), p); err != nil {
				t.Fatal(err)
			}
			d := &task.TaskDetailResponse{Task: task.Task{ID: "task", ProjectID: p.ID}, LatestAttempt: &task.TaskAttempt{ID: "attempt", Status: task.AttemptStatusFailed}}
			var payload map[string]any
			if err := json.Unmarshal([]byte(s.inboxReviewData(context.Background(), d)), &payload); err != nil {
				t.Fatal(err)
			}
			observed, ok := payload["running_project_release"].(map[string]any)
			if ok != (scenario == "matching fork") {
				t.Fatal("foreign or unidentified binary became project release evidence", payload["running_project_release"])
			}
			if ok && (observed["repository"] != "example/fork" || observed["commit"] != s.cfg.CommitSHA || observed["provenance"] != "current_process_build") {
				t.Fatal("release identity was inferred from the task rather than the binary")
			}
			if d.LatestAttempt.Status != task.AttemptStatusFailed || payload["verification_context"] != nil {
				t.Fatal("build metadata fabricated source checks or task completion")
			}
		})
	}
}

func TestInfoIdentifiesBuiltForkWithoutRemoteCredentials(t *testing.T) {
	s, _, _, _ := inboxFixture(t)
	s.cfg.SourceRepository = "example/fork"
	w := httptest.NewRecorder()
	s.handleInfo(w, httptest.NewRequest("GET", "/api/v1/info", nil))
	var payload map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &payload); err != nil || payload["repository"] != "https://github.com/example/fork" {
		t.Fatal("fork advertised upstream as its build source", err)
	}
}
