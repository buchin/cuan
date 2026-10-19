package task

import (
	"context"
	"testing"
)

func TestAgentReportResultKindDoesNotApproveWork(t *testing.T) {
	for _, kind := range []string{"answer", ""} {
		t.Run("kind_"+kind, func(t *testing.T) {
			ctx := context.Background()
			svc := NewService(NewMemoryStore(), nil)
			created, err := svc.CreateTask(ctx, "owner", CreateTaskRequest{Title: "Hello", Instructions: "Say hello"})
			if err != nil {
				t.Fatal(err)
			}
			a, err := svc.CreateAttempt(ctx, created.ID, "owner", CreateAttemptRequest{RunnerID: "local_terminal", AgentName: "fixture"})
			if err != nil {
				t.Fatal(err)
			}
			doing := TaskStatusDoing
			svc.UpdateTask(ctx, created.ID, "owner", UpdateTaskRequest{Status: &doing})
			svc.UpdateAttemptStatus(ctx, a.ID, "runner", UpdateAttemptRequest{Status: AttemptStatusRunning})
			svc.RequireAgentReport(ctx, created.ID, a.ID, "fixture")
			summary, note := "Hello!", "Answered directly; no file changes."
			for _, bad := range []AgentReport{
				{State: "progress", ResultKind: "answer", Progress: "not final"},
				{State: "review", ResultKind: "done", Summary: &summary, CompleteNote: &note},
				{State: "blocked", ResultKind: "answer", Summary: &summary, CompleteNote: &note},
			} {
				if _, err := svc.ReportAgent(ctx, created.ID, a.ID, "fixture", bad); err == nil {
					t.Fatal("invalid result classification accepted")
				}
			}
			if _, err = svc.ReportAgent(ctx, created.ID, a.ID, "fixture", AgentReport{State: "review", ResultKind: kind, Summary: &summary, CompleteNote: &note}); err != nil {
				t.Fatal(err)
			}
			d, _ := svc.GetTaskDetail(ctx, created.ID)
			if d.Status != TaskStatusDoing {
				t.Fatal("classification approved active work")
			}
			expected := kind
			if expected == "" {
				expected = "implementation"
			}
			var report, classification *TaskActivity
			for _, activity := range d.Activities {
				if activity.ActivityType == "agent_report" {
					report = activity
				}
				if activity.ActivityType == "agent_result_kind" {
					classification = activity
				}
			}
			if report == nil || classification == nil || classification.NewStatus != expected || !report.CreatedAt.Equal(classification.CreatedAt) {
				t.Fatal("classification not bound to exact final report")
			}
			zero := 0
			svc.UpdateAttemptStatus(ctx, a.ID, "runner", UpdateAttemptRequest{Status: AttemptStatusSucceeded, ExitCode: &zero})
			svc.ReconcileLocalTask(ctx, created.ID, a.ID)
			d, _ = svc.GetTaskDetail(ctx, created.ID)
			if d.Status != TaskStatusReview {
				t.Fatal("result type bypassed human Done approval", d.Status)
			}
		})
	}
}
