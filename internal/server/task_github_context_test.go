package server

import (
	"context"
	"database/sql"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/buchin/cuan/internal/git"
	"github.com/buchin/cuan/internal/task"
)

func TestTaskGitHubContextUsesOriginalOwnerAndActiveGrant(t *testing.T) {
	taskGitHubContextContract(t, nil)
}
func TestTaskGitHubContextUsesOriginalOwnerAndActiveGrantPostgres(t *testing.T) {
	withPhasePostgres(t, func(database *sql.DB) { taskGitHubContextContract(t, database) })
}

func taskGitHubContextContract(t *testing.T, database *sql.DB) {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	var s *Server
	var d *task.TaskDetailResponse
	var owner string
	var agentID string
	if database == nil {
		server, origin, detail := phaseLineageSetup(t)
		s, d, owner, agentID = server, detail, origin.OwnerID, origin.AgentID
	} else {
		server, origin, detail := phaseLineageSetup(t, database)
		s, d, owner, agentID = server, detail, origin.OwnerID, origin.AgentID
	}
	ctx := context.Background()
	d.Status = task.TaskStatusDoing
	if err := s.taskSvc.Store().UpdateTask(ctx, &d.Task); err != nil {
		t.Fatal(err)
	}
	att, err := s.taskSvc.CreateAttempt(ctx, d.ID, owner, task.CreateAttemptRequest{RunnerID: "fixture", AgentName: agentID})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.taskSvc.UpdateAttemptStatus(ctx, att.ID, "fixture", task.UpdateAttemptRequest{Status: task.AttemptStatusRunning}); err != nil {
		t.Fatal(err)
	}
	path, err := githubConnectionPath(owner)
	if err != nil {
		t.Fatal(err)
	}
	if err := git.SaveConnection(path, s.cfg.AuthSecret, "original-owner-secret", git.Account{Login: "original-owner"}); err != nil {
		t.Fatal(err)
	}
	foreign, _ := githubConnectionPath("foreign-owner")
	if err := git.SaveConnection(foreign, s.cfg.AuthSecret, "foreign-owner-secret", git.Account{Login: "foreign-owner"}); err != nil {
		t.Fatal(err)
	}
	_, _, credential, cleanup, err := s.issueTaskAgent(ctx, d.ID, att.ID, agentID)
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	get := func(id string) *httptest.ResponseRecorder {
		r := httptest.NewRequest("GET", "/api/v1/task-agent/"+id, nil)
		r.Header.Set("Authorization", "Bearer "+credential)
		w := httptest.NewRecorder()
		s.Router().ServeHTTP(w, r)
		return w
	}
	response := get(d.ID)
	if response.Code != 200 || !strings.Contains(response.Body.String(), `"state":"connected"`) || !strings.Contains(response.Body.String(), `"login":"original-owner"`) || strings.Contains(response.Body.String(), "original-owner-secret") || strings.Contains(response.Body.String(), "foreign-owner") {
		t.Fatal("task grant exposed credentials or another owner", response.Code)
	}
	if response := get("another-task"); response.Code != 403 {
		t.Fatal("task grant escaped its task")
	}
	if _, err := s.taskSvc.UpdateAttemptStatus(ctx, att.ID, "fixture", task.UpdateAttemptRequest{Status: task.AttemptStatusSucceeded}); err != nil {
		t.Fatal(err)
	}
	if response := get(d.ID); response.Code != 409 {
		t.Fatal("ended attempt retained connection access")
	}
}
