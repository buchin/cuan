package task

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

type executionReviewCountingStore struct {
	*MemoryStore
	single, batch int
}

func (s *executionReviewCountingStore) LaunchForAttempt(ctx context.Context, id string) (*TaskLaunch, error) {
	s.single++
	return s.MemoryStore.LaunchForAttempt(ctx, id)
}

func (s *executionReviewCountingStore) LaunchesForAttempts(ctx context.Context, ids []string) (map[string]*TaskLaunch, error) {
	s.batch++
	return s.MemoryStore.LaunchesForAttempts(ctx, ids)
}

func TestExecutionReviewStateUsesExactAttemptWithoutPerRowReads(t *testing.T) {
	for _, scenario := range []string{"running", "queued", "unstarted", "live inspection", "native inspection", "native release", "developer", "malformed", "cancelled", "finished", "done", "pending developer"} {
		t.Run(scenario, func(t *testing.T) {
			ctx := context.Background()
			store := &executionReviewCountingStore{MemoryStore: NewMemoryStore()}
			svc := NewService(store, nil)
			now := time.Now().UTC()
			item := &Task{ID: "task", ProjectID: "project", SourceRoomID: "91000000-0000-4000-8000-000000000001", Status: TaskStatusDoing, Instructions: "Approved work", CreatedAt: now, UpdatedAt: now}
			proof := VerificationIntent{SourceAttemptID: "source", SourceCommit: strings.Repeat("a", 40), InstructionsHash: LaunchInstructionsHash(item.Instructions), OwnerID: "owner", RoomID: item.SourceRoomID, SourceRequestID: "approval", AssessmentRequestID: "assessment", SourceVerificationHash: strings.Repeat("b", 64)}
			raw, _ := json.Marshal(proof)
			launch := TaskLaunch{TaskID: item.ID, ProjectID: item.ProjectID, RequestID: VerificationRequestID(item.ID, proof.SourceCommit, proof.SourceAttemptID), OwnerID: proof.OwnerID, RoomID: proof.RoomID, SourceRequestID: proof.SourceRequestID, AgentID: "agent_cuan", InstructionsHash: proof.InstructionsHash, Revision: VerificationIntentMarker + string(raw), CreatedAt: now, AvailableAt: now, ExpiresAt: now.Add(time.Hour)}
			attempt := &TaskAttempt{ID: attemptRequestID(item.ID, launch.RequestID), TaskID: item.ID, AgentName: "agent_cuan", RunnerID: "runner", Status: AttemptStatusRunning, CreatedAt: now, StartedAt: &now}
			want := "running"
			switch scenario {
			case "queued":
				attempt.Status, want = AttemptStatusQueued, "queued"
				attempt.StartedAt = nil
			case "unstarted":
				attempt.StartedAt, want = nil, ""
			case "live inspection", "native inspection":
				purpose := CurrentOutcomeInspection
				if scenario == "native inspection" {
					purpose = NativeReleaseInspection
				}
				live := LiveInspectionIntent{Purpose: purpose, SourceAttemptID: "source", SourceCommit: proof.SourceCommit, TargetCommit: strings.Repeat("c", 40), Repository: "owner/repo", OwnerID: proof.OwnerID, RoomID: proof.RoomID, SourceRequestID: proof.SourceRequestID, InstructionsHash: proof.InstructionsHash, ObservedAt: now}
				raw, _ := json.Marshal(live)
				launch.RequestID, launch.Revision = live.RequestID(item.ID), LiveInspectionMarker+string(raw)
			case "native release":
				native := NativeReleaseIntent{SourceAttemptID: "source", SourceCommit: proof.SourceCommit, SourceVerificationHash: proof.SourceVerificationHash, InstructionsHash: proof.InstructionsHash, OwnerID: proof.OwnerID, RoomID: proof.RoomID, SourceRequestID: proof.SourceRequestID, Repository: "owner/repo", TargetBranch: "dev", ManifestHash: strings.Repeat("d", 64)}
				raw, _ := json.Marshal(native)
				launch.RequestID, launch.Revision, want = NativeReleaseRequestID(item.ID, proof.SourceCommit), NativeReleaseIntentMarker+string(raw), ""
			case "developer":
				launch.Revision, want = "Implement the approved feature and verify it", ""
			case "malformed":
				launch.Revision, want = VerificationIntentMarker+"{}", ""
			case "cancelled":
				attempt.CancelRequested, want = true, ""
			case "finished":
				attempt.Status, want = AttemptStatusSucceeded, ""
			case "done":
				want = ""
			}
			attempt.ID = attemptRequestID(item.ID, launch.RequestID)
			if err := store.CreateTask(ctx, item); err != nil {
				t.Fatal(err)
			}
			if _, err := store.SaveLaunch(ctx, launch); err != nil {
				t.Fatal(err)
			}
			if _, err := store.ClaimLaunch(ctx, now, "fixture", LaunchID(item.ID, launch.RequestID)); err != nil {
				t.Fatal(err)
			}
			if err := store.CreateAttempt(ctx, attempt); err != nil {
				t.Fatal(err)
			}
			if scenario == "done" {
				item.Status = TaskStatusDone
				if err := store.UpdateTask(ctx, item); err != nil {
					t.Fatal(err)
				}
			}
			if scenario == "pending developer" {
				future := launch
				future.RequestID, future.Revision, future.CreatedAt = "future-dev", "Implement another correction", now.Add(time.Second)
				future.AvailableAt = future.CreatedAt
				if _, err := store.SaveLaunch(ctx, future); err != nil {
					t.Fatal(err)
				}
			}
			items, err := svc.ListTasks(ctx, TaskFilter{})
			if err != nil || len(items) != 1 || items[0].ReviewState != want {
				t.Fatalf("list state: %v %v", items, err)
			}
			if store.single != 0 || store.batch != 1 {
				t.Fatal("task list added per-row intent reads", store.single, store.batch)
			}
			detail, err := svc.GetTaskDetail(ctx, item.ID)
			if err != nil || detail.ReviewState != want || detail.ExecutionReviewState != want {
				t.Fatalf("detail state: %v %v", detail, err)
			}
			if store.single != 1 {
				t.Fatal("detail did not inspect exactly one bound launch")
			}
			encoded, _ := json.Marshal(detail)
			if strings.Contains(string(encoded), "ExecutionReviewState") {
				t.Fatal("private projection leaked into API")
			}
		})
	}
}
