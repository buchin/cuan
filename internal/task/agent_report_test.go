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
			placeholderSummary, placeholderNote := "<concise summary of work done>", "<detailed DoD verification evidence>"
			for _, bad := range []AgentReport{
				{Summary: &summary, CompleteNote: &note},
				{State: "review", Summary: &placeholderSummary, CompleteNote: &note},
				{State: "review", Summary: &summary, CompleteNote: &placeholderNote},
				{State: "progress", ResultKind: "answer", Progress: "not final"},
				{State: "review", ResultKind: "done", Summary: &summary, CompleteNote: &note},
				{State: "blocked", ResultKind: "answer", Summary: &summary, CompleteNote: &note},
			} {
				if _, err := svc.ReportAgent(ctx, created.ID, a.ID, "fixture", bad); err == nil {
					t.Fatal("invalid result classification accepted")
				}
				if _, err := svc.StageAgentReport(ctx, created.ID, a.ID, "fixture", bad); err == nil {
					t.Fatal("invalid result classification accepted for pending delivery")
				}
			}
			final := AgentReport{State: "review", ResultKind: kind, Summary: &summary, CompleteNote: &note}
			if _, err = svc.StageAgentReport(ctx, created.ID, a.ID, "fixture", final); err != nil {
				t.Fatal(err)
			}
			if ready, err := svc.AgentReviewReady(ctx, created.ID, a.ID); err != nil || ready {
				t.Fatal("staged result became ready before delivery", ready, err)
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

func TestAgentExplicitCompletion(t *testing.T) {
	for _, state := range []string{"done", "review"} {
		for _, kind := range []string{"implementation", "answer"} {
			t.Run(state+"_"+kind, func(t *testing.T) {
				ctx := context.Background()
				svc := NewService(NewMemoryStore(), nil)
				created, err := svc.CreateTask(ctx, "owner", CreateTaskRequest{Title: "Work", Instructions: "Implement request"})
				if err != nil {
					t.Fatal(err)
				}
				a, err := svc.CreateAttempt(ctx, created.ID, "owner", CreateAttemptRequest{RunnerID: "local_terminal", AgentName: "fixture"})
				if err != nil {
					t.Fatal(err)
				}
				doing := TaskStatusDoing
				if _, err = svc.UpdateTask(ctx, created.ID, "owner", UpdateTaskRequest{Status: &doing}); err != nil {
					t.Fatal(err)
				}
				if _, err = svc.UpdateAttemptStatus(ctx, a.ID, "runner", UpdateAttemptRequest{Status: AttemptStatusRunning}); err != nil {
					t.Fatal(err)
				}
				if err = svc.RequireAgentReport(ctx, created.ID, a.ID, "fixture"); err != nil {
					t.Fatal(err)
				}
				summary, note := "Implemented request", "Targeted checks passed"
				if _, err = svc.ReportAgent(ctx, created.ID, a.ID, "fixture", AgentReport{State: "done"}); err == nil {
					t.Fatal("missing evidence accepted")
				}
				if _, err = svc.ReportAgent(ctx, created.ID, a.ID, "fixture", AgentReport{State: state, ResultKind: kind, Summary: &summary, CompleteNote: &note}); err != nil {
					t.Fatal(err)
				}
				if _, err = svc.FinishReportedAttempt(ctx, a.ID, 143); err != nil {
					t.Fatal(err)
				}
				if err = svc.ReconcileLocalTask(ctx, created.ID, a.ID); err != nil {
					t.Fatal(err)
				}
				d, err := svc.GetTaskDetail(ctx, created.ID)
				if err != nil {
					t.Fatal(err)
				}
				want := TaskStatusReview
				if state == "done" && kind == "implementation" {
					want = TaskStatusDone
				}
				if d.Status != want {
					t.Fatalf("got %s want %s", d.Status, want)
				}
			})
		}
	}
}

func TestAgentReportLatestProgressOverwrites(t *testing.T) {
	ctx := context.Background()
	svc := NewService(NewMemoryStore(), nil)
	created, err := svc.CreateTask(ctx, "owner", CreateTaskRequest{Title: "Feature Task", Instructions: "Build a button"})
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

	// 1. Initial progress checkpoint
	if _, err = svc.ReportAgent(ctx, created.ID, a.ID, "fixture", AgentReport{State: "progress", Progress: "Analyzing codebase layout"}); err != nil {
		t.Fatal(err)
	}

	detail, err := svc.GetTaskDetail(ctx, created.ID)
	if err != nil || detail.LatestProgress != "Analyzing codebase layout" {
		t.Fatalf("expected progress 'Analyzing codebase layout', got %q", detail.LatestProgress)
	}
	list, err := svc.ListTasks(ctx, TaskFilter{})
	if err != nil || len(list) != 1 || list[0].LatestProgress != "Analyzing codebase layout" {
		t.Fatalf("expected list progress 'Analyzing codebase layout', got %+v", list)
	}

	// 2. Overwrite with subsequent progress update
	if _, err = svc.ReportAgent(ctx, created.ID, a.ID, "fixture", AgentReport{State: "progress", Progress: "Implementing scroll to bottom button component"}); err != nil {
		t.Fatal(err)
	}

	detail, err = svc.GetTaskDetail(ctx, created.ID)
	if err != nil || detail.LatestProgress != "Implementing scroll to bottom button component" {
		t.Fatalf("expected overwritten progress 'Implementing scroll to bottom button component', got %q", detail.LatestProgress)
	}
	list, err = svc.ListTasks(ctx, TaskFilter{})
	if err != nil || len(list) != 1 || list[0].LatestProgress != "Implementing scroll to bottom button component" {
		t.Fatalf("expected overwritten list progress, got %+v", list)
	}
}

func TestDefaultProjectKeepsLegacyCreationReplay(t *testing.T) {
	ctx := context.Background()
	svc := NewService(NewMemoryStore(), nil)
	req := CreateTaskRequest{RequestID: "226345aa-e8c7-4e78-8bf4-56fa39bd9945", Title: "Saved before default routing", Instructions: "Original scope"}
	old, err := svc.CreateTask(ctx, "owner", req)
	if err != nil {
		t.Fatal(err)
	}
	req.ProjectID = "default"
	req.ProjectIDs = []string{"default"}
	replay, err := svc.CreateTask(ctx, "owner", req)
	if err != nil || replay.ID != old.ID || replay.ProjectID != "" {
		t.Fatal("legacy task changed or duplicated", replay, err)
	}
	req.Instructions = "Changed scope"
	if _, err = svc.CreateTask(ctx, "owner", req); err == nil {
		t.Fatal("changed request accepted")
	}
}

func TestRunnerLogsDoNotBecomePublishedProgress(t *testing.T) {
	ctx := context.Background()
	svc := NewService(NewMemoryStore(), nil)
	created, err := svc.CreateTask(ctx, "owner", CreateTaskRequest{Title: "Live Task", Instructions: "Build a feature"})
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

	// Even natural-sounding terminal text, reasoning and keyboard hints stay in diagnostics.
	logs := []LogRecord{
		{AttemptID: a.ID, Stream: "stdout", Line: "Press ctrl+r to review pending artifacts."},
		{AttemptID: a.ID, Stream: "stdout", Line: "Thought for 37.8s (private reasoning fragment)"},
		{AttemptID: a.ID, Stream: "stdout", Line: "• Scheduled 1 timer (ctrl+o to expand)"},
		{AttemptID: a.ID, Stream: "stdout", Line: "Paket internal/server sedang dikompilasi... Menunggu proses linking biner akhir."},
		{AttemptID: a.ID, Stream: "stdout", Line: "\x1B[16D\x1B[?25h\x1B[?25l\r\x1B[3A⢿  \x1B[90mRunning command..\x1B[m"},
	}
	if err := svc.AppendAttemptLogs(ctx, a.ID, logs, 100); err != nil {
		t.Fatal(err)
	}

	detail, err := svc.GetTaskDetail(ctx, created.ID)
	if err != nil {
		t.Fatal(err)
	}
	if detail.LatestProgress != "" {
		t.Fatalf("unexpected terminal text in published progress: %q", detail.LatestProgress)
	}

	list, err := svc.ListTasks(ctx, TaskFilter{})
	if err != nil || len(list) != 1 {
		t.Fatalf("expected 1 task in list, got %+v", list)
	}
	if list[0].LatestProgress != "" {
		t.Fatalf("unexpected terminal text in list progress: %q", list[0].LatestProgress)
	}
}

func TestAgentReportCannotCompleteOrReplaceUnverifiedCriteria(t *testing.T) {
	ctx := context.Background()
	svc := NewService(NewMemoryStore(), nil)
	created, err := svc.CreateTask(ctx, "owner", CreateTaskRequest{Title: "Evidence", Instructions: "Deliver approved scope", DoDChecklist: []DoDItem{{ID: "approved", Text: "Original requirement"}}})
	if err != nil {
		t.Fatal(err)
	}
	a, err := svc.CreateAttempt(ctx, created.ID, "owner", CreateAttemptRequest{RunnerID: "local_terminal", AgentName: "fixture"})
	if err != nil {
		t.Fatal(err)
	}
	doing := TaskStatusDoing
	svc.UpdateTask(ctx, created.ID, "owner", UpdateTaskRequest{Status: &doing})
	svc.UpdateAttemptStatus(ctx, a.ID, "fixture", UpdateAttemptRequest{Status: AttemptStatusRunning})
	summary, note := "Implemented requirement", "Checked fixture output"
	if _, err = svc.ReportAgent(ctx, created.ID, a.ID, "fixture", AgentReport{State: "done", Summary: &summary, CompleteNote: &note}); err == nil {
		t.Fatal("unchecked criteria were implicitly completed")
	}
	if _, err = svc.StageAgentReport(ctx, created.ID, a.ID, "fixture", AgentReport{State: "done", Summary: &summary, CompleteNote: &note}); err == nil {
		t.Fatal("staging waived unverified acceptance criteria")
	}
	changed := []DoDItem{{ID: "approved", Text: "Different requirement", Completed: true, Evidence: "Claimed"}}
	if _, err = svc.ReportAgent(ctx, created.ID, a.ID, "fixture", AgentReport{State: "done", Summary: &summary, CompleteNote: &note, Checklist: &changed}); err == nil {
		t.Fatal("agent changed approved scope")
	}
	if _, err = svc.StageAgentReport(ctx, created.ID, a.ID, "fixture", AgentReport{State: "done", Summary: &summary, CompleteNote: &note, Checklist: &changed}); err == nil {
		t.Fatal("staging changed approved scope")
	}
	verified := []DoDItem{{ID: "approved", Text: "Original requirement", Completed: true, Evidence: "Fixture output matched original expected value"}}
	if _, err = svc.ReportAgent(ctx, created.ID, a.ID, "fixture", AgentReport{State: "done", Summary: &summary, CompleteNote: &note, Checklist: &verified}); err != nil {
		t.Fatal(err)
	}
	saved, _ := svc.GetTaskDetail(ctx, created.ID)
	if saved.Status != TaskStatusDoing || len(saved.DoDChecklist) != 1 || saved.DoDChecklist[0].Evidence == "" {
		t.Fatal("report bypassed execution completion or lost evidence")
	}
}
