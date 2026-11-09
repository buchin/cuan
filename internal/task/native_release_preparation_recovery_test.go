package task

import (
	"context"
	"encoding/json"
	"testing"
	"time"
)

func TestNativePreparationRecoveryRetainsSourceAndAllowsOnlyOneContinuation(t *testing.T) {
	for _, scenario := range []string{"preparation failed", "process failed", "unknown exit", "cancelled", "wrong source", "wrong owner"} {
		t.Run(scenario, func(t *testing.T) {
			ctx := context.Background()
			store, item, _, proof, launch := nativeReleaseFixture(t)
			now := time.Now().UTC()
			launch.CreatedAt, launch.AvailableAt = now.Add(-3*time.Second), now.Add(-3*time.Second)
			prior, err := store.SaveLaunch(ctx, launch)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := store.ClaimLaunch(ctx, now, "release-fixture", prior.ID); err != nil {
				t.Fatal(err)
			}
			start, finish := now.Add(-time.Second), now.Add(-time.Millisecond)
			exit := 1
			old := &TaskAttempt{ID: attemptRequestID(item.ID, launch.RequestID), TaskID: item.ID, RunnerID: "native-runner", AgentName: item.AssignedAgent, Status: AttemptStatusFailed, ExitCode: &exit, CreatedAt: now.Add(-2 * time.Second), StartedAt: &start, FinishedAt: &finish, ErrorMessage: NativeReleasePreparationUnavailableMessage}
			switch scenario {
			case "process failed":
				old.ErrorMessage = "Native release process did not finish successfully; current release must be reconciled before retrying"
			case "unknown exit":
				old.ExitCode = nil
			case "cancelled":
				old.CancelRequested = true
			}
			if err := store.CreateAttempt(ctx, old); err != nil {
				t.Fatal(err)
			}
			item.Status = TaskStatusBlocked
			if err := store.UpdateTask(ctx, item); err != nil {
				t.Fatal(err)
			}
			recovery, err := NativePreparationRecoveryIntent(ctx, store, item)
			if scenario == "process failed" || scenario == "unknown exit" || scenario == "cancelled" {
				if err == nil || recovery != nil {
					t.Fatal("uncertain process/cancellation became a publication retry")
				}
				return
			}
			if err != nil || recovery == nil || recovery.PreparationRecoveryAttemptID != old.ID || recovery.SourceCommit != proof.SourceCommit || recovery.SourceReviewAttemptID != proof.SourceReviewAttemptID {
				t.Fatal("lost approved source or failed predecessor", err)
			}
			if scenario == "wrong source" {
				recovery.SourceCommit = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
			}
			if scenario == "wrong owner" {
				recovery.OwnerID = "foreign"
			}
			if scenario != "preparation failed" {
				if ValidateNativeReleaseSource(ctx, store, item, recovery) == nil {
					t.Fatal("recovery changed original publication authority")
				}
				return
			}
			raw, _ := json.Marshal(recovery)
			child := launch
			child.RequestID = NativeReleasePreparationRecoveryRequestID(item.ID, proof.SourceCommit)
			child.Revision = NativeReleaseIntentMarker + string(raw)
			if parsed, err := NativeReleaseIntentForLaunch(item, &child); err != nil || parsed == nil || *parsed != *recovery {
				t.Fatal("recovery has no typed native launch", err)
			}
			savedChild, err := store.SaveLaunch(ctx, child)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := store.ClaimLaunch(ctx, now, "recovery-fixture", savedChild.ID); err != nil {
				t.Fatal(err)
			}
			created, started := now.Add(time.Millisecond), now.Add(2*time.Millisecond)
			attempt := &TaskAttempt{ID: attemptRequestID(item.ID, child.RequestID), TaskID: item.ID, RunnerID: "native-runner", AgentName: item.AssignedAgent, Status: AttemptStatusRunning, CreatedAt: created, StartedAt: &started}
			if err := store.CreateAttempt(ctx, attempt); err != nil {
				t.Fatal(err)
			}
			if err := ValidateNativeReleaseSource(ctx, store, item, recovery); err != nil {
				t.Fatal("valid active recovery lost its source", err)
			}
			attempt.Status, attempt.ExitCode, attempt.ErrorMessage, attempt.FinishedAt = AttemptStatusFailed, &exit, NativeReleasePreparationUnavailableMessage, &finish
			if err := store.UpdateAttempt(ctx, attempt); err != nil {
				t.Fatal(err)
			}
			if again, err := NativePreparationRecoveryIntent(ctx, store, item); err == nil || again != nil {
				t.Fatal("recovery created an unbounded retry chain")
			}
			if saved, _ := store.GetAttempt(ctx, old.ID); saved.Status != AttemptStatusFailed {
				t.Fatal("recovery rewrote failed history")
			}
		})
	}
}
