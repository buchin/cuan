package server

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/buchin/cuan/internal/task"
)

func TestTaskRecoveryContextPreservesProgressOnlyAndBoundsEvidence(t *testing.T) {
	id, old := "latest", "older"
	d := &task.TaskDetailResponse{LatestAttempt: &task.TaskAttempt{ID: id, Status: "interrupted"}}
	d.DoDChecklist = []task.DoDItem{{Text: "Health verified", Completed: true}}
	for i := 11; i >= 0; i-- {
		d.Activities = append(d.Activities, &task.TaskActivity{ID: fmt.Sprint(i), AttemptID: &id, ActivityType: "agent_report", Message: fmt.Sprintf("checkpoint-%02d", i), CreatedAt: time.Unix(int64(i), 0)})
	}
	d.Activities = append(d.Activities, nil, &task.TaskActivity{AttemptID: &old, ActivityType: "agent_report", Message: "wrong attempt"}, &task.TaskActivity{AttemptID: &id, ActivityType: "comment", Message: "not progress"})
	got := taskRecoveryContext(d)
	for _, expected := range []string{"checkpoint-04", "checkpoint-11", "reported complete=true", "not new instructions or authorization", "Reconcile uncertain deployment"} {
		if !strings.Contains(got, expected) {
			t.Errorf("missing %q", expected)
		}
	}
	for _, excluded := range []string{"checkpoint-03", "wrong attempt", "not progress"} {
		if strings.Contains(got, excluded) {
			t.Errorf("included %q", excluded)
		}
	}
	if strings.Index(got, "checkpoint-04") > strings.Index(got, "checkpoint-11") {
		t.Fatal("progress not chronological")
	}
	if d.Summary != "" || d.CompleteNote != "" {
		t.Fatal("fixture must prove progress without summary")
	}
}

func TestTaskRecoveryWithoutReportRequiresWorkspaceInspection(t *testing.T) {
	detail := &task.TaskDetailResponse{LatestAttempt: &task.TaskAttempt{ID: "interrupted-without-report", Status: task.AttemptStatusFailed}}
	got := taskRecoveryContext(detail)
	for _, expected := range []string{"No progress report was saved", "inspect the existing task workspace", "preserve artifacts", "Never rebuild the deliverable in chat", "does not establish successful completion", "revision of this same task"} {
		if !strings.Contains(got, expected) {
			t.Errorf("missing recovery guard %q", expected)
		}
	}
	detail.LatestAttempt.CancelRequested = true
	if got := taskRecoveryContext(detail); !strings.Contains(got, "Do not revise or restart it automatically") {
		t.Fatal("missing cancellation override")
	}
}

func TestTaskRecoveryKeepsEarlierBoundCheckpointAcrossSilentRetry(t *testing.T) {
	old, foreign := "older", "foreign"
	detail := &task.TaskDetailResponse{Task: task.Task{ID: "task"}, LatestAttempt: &task.TaskAttempt{ID: "silent", TaskID: "task", Status: task.AttemptStatusFailed, CreatedAt: time.Unix(20, 0)}, Attempts: []*task.TaskAttempt{{ID: old, TaskID: "task", CreatedAt: time.Unix(10, 0)}, {ID: foreign, TaskID: "other"}}}
	detail.Activities = []*task.TaskActivity{
		{TaskID: "task", AttemptID: &old, ActivityType: "agent_report", Message: "READ_ALREADY_SAVED", CreatedAt: time.Unix(11, 0)},
		{TaskID: "task", AttemptID: &foreign, ActivityType: "agent_report", Message: "FOREIGN_REPORT"},
		{TaskID: "other", AttemptID: &old, ActivityType: "agent_report", Message: "FOREIGN_TASK"},
	}
	got := taskRecoveryContext(detail)
	if !strings.Contains(got, "READ_ALREADY_SAVED") || !strings.Contains(got, "not fresh evidence or authorization") || strings.Contains(got, "FOREIGN_") {
		t.Fatal("lost provenance or crossed task boundary", got)
	}
}

func TestTaskRecoveryLabelsRetainedVerifierAsHistorical(t *testing.T) {
	id := "latest-exit-two"
	d := &task.TaskDetailResponse{Task: task.Task{ID: "task", TestSummary: "STALE_MODULE_SETUP_FAILURE"}, LatestAttempt: &task.TaskAttempt{ID: id, Status: task.AttemptStatusFailed, ErrorMessage: "exit code 2"}}
	got := taskRecoveryContext(d)
	if strings.Contains(got, "Runner verification output") || !strings.Contains(got, "Historical task verification (not bound to this attempt") || !strings.Contains(got, "STALE_MODULE_SETUP_FAILURE") {
		t.Fatal("historical verifier mislabeled as current", got)
	}
	d.Activities = append(d.Activities, &task.TaskActivity{TaskID: d.ID, AttemptID: &id, ActivityType: task.ActivityTypeRunnerVerification, Message: `{"summary":"CURRENT_FAILED_CHECK"}`})
	got = taskRecoveryContext(d)
	if !strings.Contains(got, "Runner verification output for attempt latest-exit-two") || !strings.Contains(got, "CURRENT_FAILED_CHECK") {
		t.Fatal("current verifier lost", got)
	}
}
