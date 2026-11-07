package task

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestInspectionPreparationRecoveryRequiresUnstartedApprovedParent(t *testing.T) {
	for _, scenario := range []string{"valid", "valid scope repair", "malformed scope parent", "reserved failed attempt", "active attempt", "wrong error", "stale without verifier", "cancelled parent", "expired parent", "foreign owner", "changed scope", "changed source", "changed repository", "changed purpose", "foreign parent", "chained recovery", "earlier observation", "parent revalidation"} {
		t.Run(scenario, func(t *testing.T) {
			ctx := context.Background()
			store := NewMemoryStore()
			now := time.Now().UTC()
			item := &Task{ID: "task", ProjectID: "project", SourceRoomID: "room", Instructions: "Approved scope", Status: TaskStatusBlocked}
			if err := store.CreateTask(ctx, item); err != nil {
				t.Fatal(err)
			}
			old := LiveInspectionIntent{Purpose: CurrentOutcomeInspection, SourceAttemptID: "source", TargetCommit: strings.Repeat("a", 40), OwnerID: "owner", RoomID: "room", SourceRequestID: "approved", InstructionsHash: LaunchInstructionsHash(item.Instructions), Repository: "owner/repo", ObservedAt: now.Add(-time.Minute)}
			if scenario == "parent revalidation" {
				old.PreviousAttemptID = "earlier"
				old.PreviousCommit = strings.Repeat("b", 40)
				old.ReviewRequestID = "review"
				old.ReviewPaths = []string{"app.go"}
			}
			if scenario == "chained recovery" {
				old.PreparationLaunchID = "previous-parent"
			}
			raw, _ := json.Marshal(old)
			parent, err := store.SaveLaunch(ctx, TaskLaunch{TaskID: item.ID, ProjectID: item.ProjectID, RequestID: old.RequestID(item.ID), OwnerID: old.OwnerID, RoomID: old.RoomID, SourceRequestID: old.SourceRequestID, AgentID: "agent", InstructionsHash: old.InstructionsHash, Revision: LiveInspectionMarker + string(raw), CreatedAt: now.Add(-time.Minute), AvailableAt: now.Add(-time.Minute), ExpiresAt: now.Add(time.Hour)})
			if err != nil {
				t.Fatal(err)
			}
			saved := store.launches[parent.ID]
			saved.State = "stopped"
			saved.SafeError = InspectionPublicationPreparationError
			saved.UpdatedAt = now.Add(-time.Second)
			proof := old
			proof.PreparationLaunchID = parent.ID
			proof.TargetCommit = strings.Repeat("c", 40)
			proof.ObservedAt = now
			switch scenario {
			case "valid scope repair":
				saved.SafeError = InspectionScopePreparationError
			case "malformed scope parent":
				saved.SafeError = InspectionScopePreparationError
				saved.Revision = LiveInspectionMarker + "invalid"
			case "reserved failed attempt", "active attempt":
				status := AttemptStatusFailed
				if scenario == "active attempt" {
					status = AttemptStatusRunning
				}
				store.attempts[attemptRequestID(item.ID, parent.RequestID)] = &TaskAttempt{ID: attemptRequestID(item.ID, parent.RequestID), TaskID: item.ID, Status: status}
			case "stale without verifier":
				saved.SafeError = InspectionPredecessorPreparationError
			case "wrong error":
				saved.SafeError = "unknown preparation error"
			case "cancelled parent":
				saved.State = "cancelled"
			case "expired parent":
				saved.ExpiresAt = now.Add(-time.Second)
			case "foreign owner":
				proof.OwnerID = "other"
			case "changed scope":
				proof.InstructionsHash = strings.Repeat("d", 64)
			case "changed source":
				proof.SourceAttemptID = "other"
			case "changed repository":
				proof.Repository = "other/repo"
			case "changed purpose":
				proof.Purpose = ""
			case "foreign parent":
				proof.PreparationLaunchID = "missing"
			case "earlier observation":
				proof.ObservedAt = now.Add(-time.Hour)
			}
			err = ValidateInspectionPreparationRecovery(ctx, store, item, &proof)
			if (err == nil) != (scenario == "valid" || scenario == "valid scope repair") {
				t.Fatalf("invalid preparation recovery: %v", err)
			}
		})
	}
}
