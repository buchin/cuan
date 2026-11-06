package task

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func inspectionIdentityFixture(t *testing.T) (*MemoryStore, *Task, *TaskAttempt, *LiveInspectionIntent) {
	t.Helper()
	now := time.Now().UTC()
	end := now.Add(-time.Hour)
	zero := 0
	store := NewMemoryStore()
	item := &Task{ID: "identity", ProjectID: "project", SourceRoomID: "room", Instructions: "Approved scope", Status: TaskStatusBlocked, CommitSHA: strings.Repeat("b", 40)}
	if err := store.CreateTask(context.Background(), item); err != nil {
		t.Fatal(err)
	}
	item, _ = store.GetTask(context.Background(), item.ID)
	source := &TaskAttempt{ID: "source", TaskID: item.ID, RunnerID: "runner", Status: AttemptStatusFailed, ExitCode: &zero, ErrorMessage: "Project verification failed", FinishedAt: &end, AttemptNumber: 1, CreatedAt: end.Add(-time.Minute)}
	store.attempts[source.ID] = source
	proof := &LiveInspectionIntent{SourceAttemptID: source.ID, SourceCommit: strings.Repeat("a", 40), TargetCommit: item.CommitSHA, Repository: "owner/repo", OwnerID: "owner", RoomID: item.SourceRoomID, SourceRequestID: "approved", InstructionsHash: LaunchInstructionsHash(item.Instructions), ObservedAt: end.Add(time.Minute)}
	request := proof.RequestID(item.ID)
	raw, _ := json.Marshal(proof)
	store.launches[LaunchID(item.ID, request)] = &TaskLaunch{TaskID: item.ID, ProjectID: item.ProjectID, RequestID: request, OwnerID: proof.OwnerID, RoomID: proof.RoomID, SourceRequestID: proof.SourceRequestID, InstructionsHash: proof.InstructionsHash, Revision: LiveInspectionMarker + string(raw), CreatedAt: proof.ObservedAt, State: "stopped"}
	attemptEnd := end.Add(10 * time.Minute)
	inspection := &TaskAttempt{ID: attemptRequestID(item.ID, request), TaskID: item.ID, Status: AttemptStatusInterrupted, FinishedAt: &attemptEnd, CreatedAt: proof.ObservedAt, AttemptNumber: 2, ErrorMessage: "Recorded interruption"}
	store.attempts[inspection.ID] = inspection
	for _, e := range []*TaskActivity{
		{ID: "native", ActivityType: "attempt_transition", Author: source.RunnerID, Message: "Attempt source selesai: failed. (commit: " + proof.SourceCommit + ")."},
		{ID: "report", ActivityType: "agent_report", Author: "agent", NewStatus: TaskStatusReview},
		{ID: "kind", ActivityType: "agent_result_kind", Author: "agent", NewStatus: "implementation"},
	} {
		e.TaskID = item.ID
		e.AttemptID = &source.ID
		e.CreatedAt = end
		store.activities[e.ID] = e
	}
	return store, item, inspection, proof
}

func TestInspectionIdentityRecovery(t *testing.T) {
	for _, scenario := range []string{"valid", "no native receipt", "agent receipt", "wrong source", "late receipt", "cancelled", "active", "accepted", "changed scope", "foreign room", "new attempt", "pending launch"} {
		t.Run(scenario, func(t *testing.T) {
			store, item, a, _ := inspectionIdentityFixture(t)
			switch scenario {
			case "no native receipt":
				delete(store.activities, "native")
			case "agent receipt":
				store.activities["native"].Author = "agent"
			case "wrong source":
				store.activities["native"].Message = "different commit"
			case "late receipt":
				store.activities["native"].CreatedAt = time.Now()
			case "cancelled":
				a.CancelRequested = true
			case "active":
				a.Status = AttemptStatusRunning
			case "accepted":
				store.tasks[item.ID].Status = TaskStatusDone
			case "changed scope":
				store.tasks[item.ID].Instructions = "Changed scope"
			case "foreign room":
				store.tasks[item.ID].SourceRoomID = "other"
			case "new attempt":
				store.attempts["new"] = &TaskAttempt{ID: "new", TaskID: item.ID, Status: AttemptStatusRunning, AttemptNumber: 3}
			case "pending launch":
				store.launches["pending"] = &TaskLaunch{TaskID: item.ID, State: "queued"}
			}
			changed, err := NewService(store, nil).ReconcileInspectionIdentity(context.Background(), item.ID)
			if scenario == "valid" {
				if err != nil || !changed {
					t.Fatalf("not restored: %v %v", changed, err)
				}
				saved, _ := store.GetTask(context.Background(), item.ID)
				if saved.CommitSHA != strings.Repeat("a", 40) || saved.Status != TaskStatusBlocked || a.Status != AttemptStatusInterrupted || a.ErrorMessage != "Recorded interruption" || len(store.attempts) != 2 {
					t.Fatal("restoration rewrote work history")
				}
				if changed, err = NewService(store, nil).ReconcileInspectionIdentity(context.Background(), item.ID); changed || err != nil {
					t.Fatal("not idempotent", err)
				}
			} else if changed {
				t.Fatal("unsafe identity restored")
			}
		})
	}
}

func TestInspectionIdentityCompareAndSwap(t *testing.T) {
	for _, scenario := range []string{"changed timestamp", "changed commit", "cancelled", "new attempt", "active launch"} {
		t.Run(scenario, func(t *testing.T) {
			store, item, a, proof := inspectionIdentityFixture(t)
			switch scenario {
			case "changed timestamp":
				store.tasks[item.ID].UpdatedAt = time.Now().Add(time.Minute)
			case "changed commit":
				store.tasks[item.ID].CommitSHA = strings.Repeat("c", 40)
			case "cancelled":
				store.attempts[a.ID].CancelRequested = true
			case "new attempt":
				store.attempts["new"] = &TaskAttempt{ID: "new", TaskID: item.ID, AttemptNumber: 3}
			case "active launch":
				store.launches["new"] = &TaskLaunch{TaskID: item.ID, State: "claimed"}
			}
			changed, err := store.RestoreInspectionIdentity(context.Background(), item, a, proof, &TaskActivity{ID: "restore"})
			if err != nil || changed {
				t.Fatal("stale snapshot restored", err)
			}
			if _, ok := store.activities["restore"]; ok {
				t.Fatal("failed CAS saved receipt")
			}
		})
	}
}

func TestInspectionIdentityCurrentOutcome(t *testing.T) {
	store, item, a, proof := inspectionIdentityFixture(t)
	source := store.attempts[proof.SourceAttemptID]
	source.Status = AttemptStatusSucceeded
	source.ErrorMessage = ""
	start := source.CreatedAt.Add(time.Second)
	source.StartedAt = &start
	item.Instructions = "Cuan managed delegation v1.\nApproved task"
	store.tasks[item.ID].Instructions = item.Instructions
	proof.Purpose = CurrentOutcomeInspection
	proof.InstructionsHash = LaunchInstructionsHash(item.Instructions)
	raw, _ := json.Marshal(map[string]any{"request_id": "review", "instructions_hash": proof.InstructionsHash, "evidence_digest": strings.Repeat("c", 64), "assessment": map[string]string{"decision": "revise", "remaining_work": "verification"}})
	store.activities["review"] = &TaskActivity{ID: "review", TaskID: item.ID, AttemptID: &source.ID, Author: "coordinator", ActivityType: "coordinator_review_assessment", Message: string(raw), CreatedAt: source.FinishedAt.Add(time.Second)}
	var launch *TaskLaunch
	for id, l := range store.launches {
		launch = l
		delete(store.launches, id)
	}
	launch.RequestID = proof.RequestID(item.ID)
	launch.InstructionsHash = proof.InstructionsHash
	raw, _ = json.Marshal(proof)
	launch.Revision = LiveInspectionMarker + string(raw)
	store.launches[LaunchID(item.ID, launch.RequestID)] = launch
	delete(store.attempts, a.ID)
	a.ID = attemptRequestID(item.ID, launch.RequestID)
	store.attempts[a.ID] = a
	changed, err := NewService(store, nil).ReconcileInspectionIdentity(context.Background(), item.ID)
	if err != nil || !changed {
		t.Fatal("current outcome identity not restored", err)
	}
	if store.attempts[a.ID].Status != AttemptStatusInterrupted {
		t.Fatal("interruption was erased")
	}
}
