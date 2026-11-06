package server

import (
	"context"
	"database/sql"
	"strings"
	"testing"
	"time"

	"github.com/buchin/cuan/internal/task"
)

func TestApprovedPhaseSurvivesAcceptedPredecessorCorrection(t *testing.T) {
	for _, scenario := range []string{"accepted correction", "parent review", "new failed", "new cancelled", "historical cancelled", "parent scope changed", "approval withdrawn", "phase scope changed", "foreign owner", "parent lane changed"} {
		t.Run(scenario, func(t *testing.T) { correctedPhaseContract(t, scenario) })
	}
}
func TestCorrectedPhaseLineagePostgres(t *testing.T) {
	for _, scenario := range []string{"accepted correction", "new failed", "parent scope changed", "approval withdrawn"} {
		t.Run(scenario, func(t *testing.T) {
			withPhasePostgres(t, func(database *sql.DB) { correctedPhaseContract(t, scenario, database) })
		})
	}
}
func correctedPhaseContract(t *testing.T, scenario string, databases ...*sql.DB) {
	t.Helper()
	s, q, root, parent, _ := phaseLineageFixture(t, databases...)
	ctx := context.Background()
	first := &task.TaskAttempt{ID: "phase-original", TaskID: parent.ID, RunnerID: "local_terminal", Status: task.AttemptStatusSucceeded, AttemptNumber: 1, CreatedAt: time.Now().UTC()}
	if err := s.taskSvc.Store().CreateAttempt(ctx, first); err != nil {
		t.Fatal(err)
	}
	parent.Status = task.TaskStatusDone
	if err := s.taskSvc.Store().UpdateTask(ctx, &parent.Task); err != nil {
		t.Fatal(err)
	}
	if err := s.taskSvc.FinishTaskLaunches(ctx, parent.ID, "stopped"); err != nil {
		t.Fatal(err)
	}
	parent, _ = s.taskSvc.GetTaskDetail(ctx, parent.ID)
	text := s.enqueueInboxAutoDelegation(ctx, q, parent, "Accepted phase")
	if !strings.Contains(text, "Phase 3: UI") {
		t.Fatal(text)
	}
	id := "msg_" + delegationMessageID("auto-phase:"+root.ID+":3:input")
	child, err := s.taskSvc.GetTaskDetail(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	var original string
	for _, event := range child.Activities {
		if event.ID == phaseReceiptID(child.ID) {
			original = event.Message
		}
	}
	if original == "" {
		t.Fatal("missing immutable receipt")
	}
	launch, err := s.taskSvc.GetLaunch(ctx, task.LaunchID(child.ID, delegationMessageID("auto-phase:"+root.ID+":3")))
	if err != nil {
		t.Fatal(err)
	}
	corrected := &task.TaskAttempt{ID: "phase-correction", TaskID: parent.ID, RunnerID: "local_terminal", Status: task.AttemptStatusSucceeded, AttemptNumber: 2, CreatedAt: first.CreatedAt.Add(time.Second)}
	switch scenario {
	case "parent review":
		parent.Status = task.TaskStatusReview
	case "new failed":
		corrected.Status = task.AttemptStatusFailed
	case "new cancelled":
		corrected.CancelRequested = true
	case "historical cancelled":
		first.CancelRequested = true
		if err := s.taskSvc.Store().UpdateAttempt(ctx, first); err != nil {
			t.Fatal(err)
		}
	case "parent scope changed":
		parent.Instructions += "\nDifferent scope"
	case "parent lane changed":
		parent.ExecutionCategory = task.ExecutionCategoryReadOnly
	case "phase scope changed":
		child.Instructions += "\nDifferent scope"
	case "foreign owner":
		launch.OwnerID = "other"
	case "approval withdrawn":
		m, _ := s.projectStudioRepo.GetMessageByID(ctx, q.MessageID)
		m.Content = "Plan withdrawn"
		if err := s.projectStudioRepo.UpdateMessage(ctx, m); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.taskSvc.Store().CreateAttempt(ctx, corrected); err != nil {
		t.Fatal(err)
	}
	if err := s.taskSvc.Store().UpdateTask(ctx, &parent.Task); err != nil {
		t.Fatal(err)
	}
	_, authorized, err := s.authorizeSavedTaskLaunch(ctx, *launch, child)
	if err != nil {
		t.Fatal(err)
	}
	if authorized != (scenario == "accepted correction") {
		t.Fatalf("authorization=%v for %s", authorized, scenario)
	}
	if scenario == "accepted correction" {
		parent, _ = s.taskSvc.GetTaskDetail(ctx, parent.ID)
		approval, err := s.phaseApproval(ctx, parent, q.OwnerID, q.RoomID)
		if err != nil {
			t.Fatal(err)
		}
		if err := s.savePhaseLineage(ctx, child, parent, approval, q.OwnerID, q.RoomID); err != nil {
			t.Fatal("immutable receipt replay failed", err)
		}
		// Missing history or an attempt rewind must never acquire approval.
		copy := *parent
		copy.Attempts = nil
		if phasePredecessorAttemptValid(&copy, first.ID) {
			t.Fatal("missing historical evidence accepted")
		}
		copy.Attempts = []*task.TaskAttempt{{ID: first.ID, TaskID: parent.ID, Status: task.AttemptStatusSucceeded, AttemptNumber: 3}}
		if phasePredecessorAttemptValid(&copy, first.ID) {
			t.Fatal("attempt rewind accepted")
		}
	}
	after, _ := s.taskSvc.GetTaskDetail(ctx, child.ID)
	receipts := 0
	for _, event := range after.Activities {
		if event.ID == phaseReceiptID(child.ID) {
			receipts++
			if event.Message != original {
				t.Fatal("historical receipt rewritten")
			}
		}
	}
	if receipts != 1 || len(after.Attempts) != 0 {
		t.Fatal("authorization replay created execution or extra lineage", receipts, len(after.Attempts))
	}
	// Original attempt remains the original successful execution, except the
	// explicit negative-fixture cancellation. No execution or provider ran.
	historical, err := s.taskSvc.Store().GetAttempt(ctx, first.ID)
	if err != nil || historical.Status != task.AttemptStatusSucceeded {
		t.Fatal(historical, err)
	}
}
