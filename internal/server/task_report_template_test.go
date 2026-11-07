package server

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	cuanPrompt "github.com/buchin/cuan/internal/prompt"
	"github.com/buchin/cuan/internal/task"
	"github.com/buchin/cuan/internal/taskcli"
)

func TestReportTemplateRoundTripPreservesPendingIndependentEvidence(t *testing.T) {
	s, _, _, _ := inboxFixture(t)
	api := httptest.NewServer(s.Router())
	defer api.Close()
	s.taskAgentURL = api.URL
	ctx := context.Background()
	criteria := []task.DoDItem{{ID: "original-second", Text: "Observe exact \"ready\" 文", Completed: true, Evidence: "Old evidence"}, {ID: "original-first", Text: "Independently verify the result"}}
	created, err := s.taskSvc.CreateTask(ctx, "owner", task.CreateTaskRequest{Title: "Observe fixture", Instructions: "Observe the fixture result", DoDChecklist: criteria})
	if err != nil {
		t.Fatal(err)
	}
	att, err := s.taskSvc.CreateAttempt(ctx, created.ID, "owner", task.CreateAttemptRequest{RunnerID: "local_terminal", AgentName: "agent_lifecycle"})
	if err != nil {
		t.Fatal(err)
	}
	doing := task.TaskStatusDoing
	if _, err = s.taskSvc.UpdateTask(ctx, created.ID, "owner", task.UpdateTaskRequest{Status: &doing}); err != nil {
		t.Fatal(err)
	}
	if _, err = s.taskSvc.UpdateAttemptStatus(ctx, att.ID, "fixture", task.UpdateAttemptRequest{Status: task.AttemptStatusRunning}); err != nil {
		t.Fatal(err)
	}
	env, extra, _, cleanup, err := s.issueTaskAgent(ctx, created.ID, att.ID, "agent_lifecycle")
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	_, guidePath, ok := strings.Cut(extra, "Read the session guidance at ")
	if !ok {
		t.Fatal("missing private capability guidance")
	}
	guide, err := os.ReadFile(strings.TrimSuffix(guidePath, " before proceeding."))
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "AGENTS.md"), []byte("TARGET_REPOSITORY_RULE"), 0600); err != nil {
		t.Fatal(err)
	}
	combined := cuanPrompt.BuildRunnerPromptWithDoD(created.Title, created.Instructions, extra, []string{criteria[0].Text, criteria[1].Text}, root) + "\n" + string(guide)
	if strings.Count(combined, "=== MANDATORY REPOSITORY GUIDANCE (AGENTS.md) ===") != 1 || !strings.Contains(combined, "TARGET_REPOSITORY_RULE") || !strings.Contains(string(guide), "task report-template") || !strings.Contains(combined, criteria[1].Text) {
		t.Fatal("duplicate/missing repository guidance or report context")
	}
	preparedPath := ""
	for _, entry := range env {
		key, value, ok := strings.Cut(entry, "=")
		if ok {
			t.Setenv(key, value)
			if key == "CUAN_TASK_REPORT_FILE" {
				preparedPath = value
			}
		}
	}
	prepared, err := os.ReadFile(preparedPath)
	if err != nil || !strings.Contains(string(guide), "CUAN_TASK_REPORT_FILE") {
		t.Fatal("launch did not supply prepared report", err)
	}
	info, err := os.Stat(preparedPath)
	if err != nil || info.Mode().Perm() != 0600 || strings.Contains(string(prepared), "Old evidence") {
		t.Fatal("prepared report permissions or inherited claims", err)
	}
	var out, errs bytes.Buffer
	if code := taskcli.Run([]string{"report-template"}, &out, &errs); code != 0 {
		t.Fatal(code, errs.String())
	}
	if !bytes.Equal(prepared, out.Bytes()) && strings.TrimSpace(string(prepared)) != strings.TrimSpace(out.String()) {
		t.Fatal("launch and scoped CLI report schemas differ")
	}
	path := preparedPath
	if code := taskcli.Run([]string{"report", "--report-file", path}, &bytes.Buffer{}, &bytes.Buffer{}); code == 0 {
		t.Fatal("empty template accepted as a factual handoff")
	}
	ready, err := s.taskSvc.AgentReviewReady(ctx, created.ID, att.ID)
	if err != nil || ready {
		t.Fatal("template finished session without observations", err)
	}
	var report struct {
		State    string         `json:"state"`
		Summary  string         `json:"summary"`
		Note     string         `json:"complete_note"`
		Criteria []task.DoDItem `json:"dod_checklist"`
	}
	if err := json.Unmarshal(out.Bytes(), &report); err != nil {
		t.Fatal(err)
	}
	if len(report.Criteria) != 2 || report.Criteria[0].Completed || report.Criteria[0].Evidence != "" {
		t.Fatal("template inherited old completion")
	}
	report.Summary = "Fixture result observed"
	report.Note = "Observed exact fixture value ready; independent verification remains pending"
	report.Criteria[0].Completed = true
	report.Criteria[0].Evidence = "Observed in-fixture literal ready"
	report.Criteria[1].Evidence = "Waiting for the separate verifier"
	raw, err := json.Marshal(report)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
	if code := taskcli.Run([]string{"report", "--report-file", path}, &bytes.Buffer{}, &errs); code != 0 {
		t.Fatal(code, errs.String())
	}
	saved, err := s.taskSvc.GetTaskDetail(ctx, created.ID)
	if err != nil || saved.Status != task.TaskStatusDoing || len(saved.DoDChecklist) != 2 {
		t.Fatal("report bypassed independent acceptance", err)
	}
	for i, item := range saved.DoDChecklist {
		if item.ID != criteria[i].ID || item.Text != criteria[i].Text {
			t.Fatal("original criterion identity/order changed")
		}
	}
	if saved.DoDChecklist[1].Completed || saved.DoDChecklist[1].Evidence != "Waiting for the separate verifier" {
		t.Fatal("pending criterion was checked off or lost")
	}
}
