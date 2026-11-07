package server

import (
	"context"
	"encoding/json"
	"github.com/buchin/cuan/internal/task"
	"os"
	"strings"
	"testing"
)

func TestStandaloneArtifactGuidanceProvidesCriteriaWithoutRepositoryProcedures(t *testing.T) {
	s, _, _, _ := inboxFixture(t)
	ctx := context.Background()
	scope := "Create a standalone file containing the exact requested line. Do not modify the repository. Do not deploy."
	criteria := []task.DoDItem{{ID: "original-file", Text: "Deliver result.txt containing the exact requested line", Completed: true, Evidence: "Historical claim must not become current evidence"}}
	created, err := s.taskSvc.CreateTask(ctx, "owner", task.CreateTaskRequest{Title: "Create result.txt", Instructions: scope, DoDChecklist: criteria})
	if err != nil {
		t.Fatal(err)
	}
	a, err := s.taskSvc.CreateAttempt(ctx, created.ID, "owner", task.CreateAttemptRequest{RunnerID: "local_terminal", AgentName: "agent_lifecycle"})
	if err != nil {
		t.Fatal(err)
	}
	doing := task.TaskStatusDoing
	if _, err = s.taskSvc.UpdateTask(ctx, created.ID, "owner", task.UpdateTaskRequest{Status: &doing}); err != nil {
		t.Fatal(err)
	}
	if _, err = s.taskSvc.UpdateAttemptStatus(ctx, a.ID, "fixture", task.UpdateAttemptRequest{Status: task.AttemptStatusRunning}); err != nil {
		t.Fatal(err)
	}
	env, prompt, token, cleanup, err := s.issueTaskAgent(ctx, created.ID, a.ID, "agent_lifecycle")
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	guideBytes := len(prompt)
	if _, path, ok := strings.Cut(prompt, "Read the session guidance at "); ok {
		path = strings.TrimSuffix(path, " before proceeding.")
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		guideBytes = len(raw)
	}
	t.Logf("artifact session guidance bytes=%d", guideBytes)
	if len(env) == 0 || !strings.Contains(prompt, "original-file") || !strings.Contains(prompt, criteria[0].Text) {
		t.Fatal("worker needs another read to recover original acceptance IDs/text")
	}
	for _, unrelated := range []string{"Read the session guidance at", "AGENTIC SCRUM", "MANDATORY REPOSITORY GUIDANCE", "Historical claim must not become current evidence"} {
		if strings.Contains(prompt, unrelated) {
			t.Fatal("unrelated or duplicate startup procedure", unrelated)
		}
	}
	if strings.Contains(prompt, token) || !strings.Contains(prompt, "--artifact") || !strings.Contains(prompt, "--report-file") {
		t.Fatal("file/report contract missing or credential exposed")
	}
	_, payload, ok := strings.Cut(prompt, "Original acceptance criteria:\n"+"```json\n")
	if !ok {
		t.Fatal("missing machine-readable original criteria")
	}
	payload, _, ok = strings.Cut(payload, "\n```")
	if !ok {
		t.Fatal("criterion snapshot is not delimited")
	}
	var reportItems []task.DoDItem
	if err = json.Unmarshal([]byte(payload), &reportItems); err != nil {
		t.Fatal(err)
	}
	for i := range reportItems {
		reportItems[i].Completed = true
		reportItems[i].Evidence = "Contract fixture preserved the original criterion identity; independent acceptance still required"
	}
	summary, note := "Criterion handoff fixture", "Only reporting contract exercised; no production task accepted"
	saved, err := s.taskSvc.ReportAgent(ctx, created.ID, a.ID, "fixture", task.AgentReport{State: "review", Summary: &summary, CompleteNote: &note, Checklist: &reportItems})
	if err != nil || saved.Status != task.TaskStatusDoing {
		t.Fatal("supplied snapshot could not report without another metadata read, or bypassed acceptance", err)
	}
}
