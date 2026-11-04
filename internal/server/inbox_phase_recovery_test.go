package server

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/buchin/cuan/internal/db"
	"github.com/buchin/cuan/internal/task"
)

type phaseLaunchFailureStore struct {
	task.Store
	task.LaunchStore
	task.PendingCoordinatorStore
	fail bool
}

func (s *phaseLaunchFailureStore) SaveLaunch(ctx context.Context, j task.TaskLaunch) (*task.TaskLaunch, error) {
	if s.fail {
		return nil, errors.New("temporary launch storage outage")
	}
	return s.LaunchStore.SaveLaunch(ctx, j)
}

func phaseHandoffRecoveryContract(t *testing.T, databases ...*sql.DB) {
	t.Helper()
	s, q, parent := phaseLineageSetup(t, databases...)
	ctx := context.Background()
	underlying := s.taskSvc.Store()
	failing := &phaseLaunchFailureStore{Store: underlying, LaunchStore: underlying.(task.LaunchStore), PendingCoordinatorStore: underlying.(task.PendingCoordinatorStore), fail: true}
	s.taskSvc = task.NewService(failing, s.projectSvc)
	decision := inboxReviewEvaluation{Decision: "pass", Explanation: "Phase accepted"}
	if err := s.savePhaseHandoffIntent(ctx, q, parent, parent, decision); err != nil {
		t.Fatal(err)
	}
	if _, err := s.attemptInboxAutoDelegation(ctx, q, parent, decision.Explanation); err == nil {
		t.Fatal("launch persistence failure was hidden by reply text")
	}
	childID := "msg_" + delegationMessageID("auto-phase:"+parent.ID+":2:input")
	requestID := delegationMessageID("auto-phase:" + parent.ID + ":2")
	if _, err := s.taskSvc.GetLaunch(ctx, task.LaunchID(childID, requestID)); err == nil {
		t.Fatal("fixture unexpectedly persisted launch")
	}
	// A fresh coordinator sees durable intent and records backoff on storage error.
	restarted := &Server{taskSvc: s.taskSvc, projectStudioRepo: s.projectStudioRepo, projectSvc: s.projectSvc}
	restarted.recoverPhaseHandoffs(ctx)
	pending, err := failing.PendingPhaseHandoffs(ctx, time.Now().UTC(), 20)
	if err != nil || len(pending) != 0 {
		t.Fatalf("recovery ignored cooldown: %d %v", len(pending), err)
	}
	failing.fail = false
	pending, err = failing.PendingPhaseHandoffs(ctx, time.Now().UTC().Add(6*time.Minute), 20)
	if err != nil || len(pending) != 1 {
		t.Fatalf("durable intent lost: %d %v", len(pending), err)
	}
	restarted.recoverPhaseHandoff(ctx, pending[0], time.Now().UTC().Add(6*time.Minute))
	launch, err := s.taskSvc.GetLaunch(ctx, task.LaunchID(childID, requestID))
	if err != nil || launch.State != "queued" || launch.SourceRequestID != q.RequestID {
		t.Fatalf("recovery did not preserve approved launch: %v %v", launch, err)
	}
	restarted.recoverPhaseHandoffs(ctx)
	rows, err := s.taskSvc.ListTasks(ctx, task.TaskFilter{SourceRoomID: q.RoomID})
	if err != nil || len(rows) != 2 {
		t.Fatalf("duplicate phase: %d %v", len(rows), err)
	}
	child, err := s.taskSvc.GetTaskDetail(ctx, childID)
	if err != nil || len(child.Attempts) != 0 || !strings.Contains(child.Instructions, "Do not deploy.") {
		t.Fatal("recovery executed work or relaxed scope", err)
	}
	pending, err = failing.PendingPhaseHandoffs(ctx, time.Now().UTC().Add(10*time.Minute), 20)
	if err != nil || len(pending) != 0 {
		t.Fatal("finished intent remained pending", err)
	}
}
func TestApprovedPhaseHandoffRecoveryAfterLaunchStorageOutage(t *testing.T) {
	phaseHandoffRecoveryContract(t)
}

func TestApprovedPhaseHandoffRecoveryRejectsChangedParent(t *testing.T) {
	for _, scenario := range []string{"scope", "attempt cancelled", "evidence", "expired"} {
		t.Run(scenario, func(t *testing.T) {
			s, q, parent := phaseLineageSetup(t)
			ctx := context.Background()
			if err := s.savePhaseHandoffIntent(ctx, q, parent, parent, inboxReviewEvaluation{Decision: "pass", Explanation: "Accepted"}); err != nil {
				t.Fatal(err)
			}
			pending, err := s.taskSvc.Store().(task.PendingCoordinatorStore).PendingPhaseHandoffs(ctx, time.Now().UTC(), 20)
			if err != nil || len(pending) != 1 {
				t.Fatal("missing intent", err)
			}
			row, _ := s.taskSvc.Store().GetTask(ctx, parent.ID)
			now := time.Now().UTC()
			switch scenario {
			case "scope":
				row.Instructions += "\nChanged scope"
			case "evidence":
				row.CompleteNote = "Replaced report"
			case "attempt cancelled":
				a, _ := s.taskSvc.Store().GetLatestAttempt(ctx, parent.ID)
				a.CancelRequested = true
				s.taskSvc.Store().UpdateAttempt(ctx, a)
			case "expired":
				now = now.Add(25 * time.Hour)
			}
			if err := s.taskSvc.Store().UpdateTask(ctx, row); err != nil {
				t.Fatal(err)
			}
			s.recoverPhaseHandoff(ctx, pending[0], now)
			rows, _ := s.taskSvc.ListTasks(ctx, task.TaskFilter{SourceRoomID: q.RoomID})
			if len(rows) != 1 {
				t.Fatal("changed parent started a new phase")
			}
			left, err := s.taskSvc.Store().(task.PendingCoordinatorStore).PendingPhaseHandoffs(ctx, now, 20)
			if err != nil || len(left) != 0 {
				t.Fatal("terminal intent still retrying", err)
			}
		})
	}
}

var _ db.ProjectStudioRepository = phaseLookupFailureRepository{}

type phaseAcceptanceFailureStore struct {
	task.Store
	task.LaunchStore
	task.PendingCoordinatorStore
}

func (s phaseAcceptanceFailureStore) UpdateTask(ctx context.Context, t *task.Task) error {
	if t.Status == task.TaskStatusDone {
		return errors.New("temporary acceptance storage failure")
	}
	return s.Store.UpdateTask(ctx, t)
}
func TestApprovedPhaseRecoveryCompletesPersistedReviewAfterAcceptanceOutage(t *testing.T) {
	s, origin, parent := phaseLineageSetup(t)
	ctx := context.Background()
	underlying := s.taskSvc.Store()
	row, _ := underlying.GetTask(ctx, parent.ID)
	row.Status = task.TaskStatusReview
	row.Summary = "Phase checks passed"
	row.CompleteNote = "Verified original criteria"
	if err := underlying.UpdateTask(ctx, row); err != nil {
		t.Fatal(err)
	}
	key := "task-review:" + parent.LatestAttempt.ID
	q := origin
	q.RequestID = delegationMessageID(key)
	q.MessageID = delegationMessageID(key + ":input")
	q.ReplyID = delegationMessageID(key + ":reply")
	if err := s.projectStudioRepo.CreateMessage(ctx, &db.ProjectMessage{ID: q.MessageID, RoomID: q.RoomID, AuthorType: "system", AuthorID: "cuan_work_review", Content: "Review /tasks/" + parent.ID, CreatedAt: time.Now().UTC()}); err != nil {
		t.Fatal(err)
	}
	failing := phaseAcceptanceFailureStore{Store: underlying, LaunchStore: underlying.(task.LaunchStore), PendingCoordinatorStore: underlying.(task.PendingCoordinatorStore)}
	s.taskSvc = task.NewService(failing, s.projectSvc)
	s.applyInboxReviewEvaluation(ctx, q, `{"decision":"pass","explanation":"Phase accepted","revision":""}`)
	saved, _ := underlying.GetTask(ctx, parent.ID)
	if saved.Status == task.TaskStatusDone {
		t.Fatal("acceptance failure fixture failed")
	}
	pending, err := failing.PendingPhaseHandoffs(ctx, time.Now().UTC(), 20)
	if err != nil || len(pending) != 1 {
		t.Fatal("review decision was not persisted before acceptance", err)
	}
	s.taskSvc = task.NewService(underlying, s.projectSvc)
	restarted := &Server{taskSvc: s.taskSvc, projectStudioRepo: s.projectStudioRepo, projectSvc: s.projectSvc}
	restarted.recoverPhaseHandoffs(ctx)
	saved, _ = underlying.GetTask(ctx, parent.ID)
	if saved.Status != task.TaskStatusDone {
		t.Fatal("verified parent not accepted after storage recovered", saved.Status)
	}
	childID := "msg_" + delegationMessageID("auto-phase:"+parent.ID+":2:input")
	child, err := s.taskSvc.GetTaskDetail(ctx, childID)
	if err != nil || child.LatestAttempt != nil {
		t.Fatal("child missing or executed instead of queued", err)
	}
	pending, err = failing.PendingPhaseHandoffs(ctx, time.Now().UTC().Add(6*time.Minute), 20)
	if err != nil || len(pending) != 0 {
		t.Fatal("accepted handoff did not finish", len(pending), err)
	}
}
