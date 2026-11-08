package task

import (
	"context"
	"encoding/json"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestCompletedNativeDeliveryRequiresOriginalSourcePreparationAndLiveProof(t *testing.T) {
	for _, scenario := range []string{"valid", "accepted task", "missing delivery", "missing preparation", "missing live proof", "failed stage", "revoked source", "changed scope", "changed task source", "wrong author", "wrong target", "stale observation", "process before preparation", "unknown field"} {
		t.Run(scenario, func(t *testing.T) {
			ctx := context.Background()
			store, item, source, intent, launch := nativeReleaseFixture(t)
			svc := NewService(store, nil)
			must := func(err error) {
				t.Helper()
				if err != nil {
					t.Fatal(err)
				}
			}
			saved, err := svc.SaveLaunch(ctx, launch)
			must(err)
			_, err = svc.ClaimLaunch(ctx, time.Now(), "dispatcher", saved.ID)
			must(err)
			att, err := svc.CreateAttempt(ctx, item.ID, "owner", CreateAttemptRequest{RunnerID: "native", AgentName: launch.AgentID, RequestID: launch.RequestID, ExpectedPredecessorID: intent.SourceReviewAttemptID})
			must(err)
			time.Sleep(time.Millisecond)
			now := time.Now().UTC()
			started := att.CreatedAt
			tick := now.Sub(started) / 10
			finished := started.Add(8 * tick)
			zero := 0
			att.RunnerID = "native"
			att.Status = AttemptStatusSucceeded
			att.StartedAt = &started
			att.FinishedAt = &finished
			att.ExitCode = &zero
			if scenario == "failed stage" {
				att.Status = AttemptStatusFailed
			}
			must(store.UpdateAttempt(ctx, att))
			target := strings.Repeat("c", 40)
			prep := NativeReleasePreparation{Version: 1, TaskID: item.ID, AttemptID: att.ID, SourceCommit: intent.SourceCommit, TargetCommit: target, InstructionsHash: intent.InstructionsHash, Repository: intent.Repository, TargetBranch: intent.TargetBranch, ManifestHash: intent.ManifestHash}
			delivery := NativeReleaseDelivery{Version: 1, TaskID: item.ID, AttemptID: att.ID, SourceAttemptID: source.ID, SourceCommit: intent.SourceCommit, TargetCommit: target, InstructionsHash: intent.InstructionsHash, Repository: intent.Repository, TargetBranch: intent.TargetBranch, ManifestHash: intent.ManifestHash, ProcessStartedAt: started.Add(3 * tick), ProcessFinishedAt: started.Add(4 * tick)}
			rawPrep, _ := json.Marshal(prep)
			prepEvent := &TaskActivity{ID: "native-release-preparation:" + att.ID, TaskID: item.ID, AttemptID: &att.ID, Author: "cuan-native-release", ActivityType: ActivityNativeReleasePreparation, Message: string(rawPrep), CreatedAt: started.Add(2 * tick)}
			live := DeploymentReceipt{Version: 1, TaskID: item.ID, AttemptID: att.ID, CommitSHA: target, ReleaseSHA: target, Environment: "production", Health: "healthy"}
			rawLive, _ := json.Marshal(live)
			liveEvent := &TaskActivity{ID: DeploymentReceiptID(item.ID, att.ID, target), TaskID: item.ID, AttemptID: &att.ID, Author: "cuan-deploy", ActivityType: "deployment_verified", Message: string(rawLive), CreatedAt: started.Add(6 * tick)}
			event := &TaskActivity{ID: "native-release-delivery:" + att.ID, TaskID: item.ID, AttemptID: &att.ID, Author: att.RunnerID, ActivityType: "native_release_delivery", CreatedAt: finished}
			switch scenario {
			case "accepted task":
				item.Status = TaskStatusDone
				must(store.UpdateTask(ctx, item))
			case "revoked source":
				source.CancelRequested = true
				must(store.UpdateAttempt(ctx, source))
			case "changed scope":
				item.Instructions += " changed"
				must(store.UpdateTask(ctx, item))
			case "changed task source":
				item.CommitSHA = strings.Repeat("d", 40)
				must(store.UpdateTask(ctx, item))
			case "wrong author":
				event.Author = "agent_cuan"
			case "wrong target":
				delivery.TargetCommit = strings.Repeat("d", 40)
			case "stale observation":
				liveEvent.CreatedAt = started.Add(3 * tick)
			case "process before preparation":
				delivery.ProcessStartedAt = started
			}
			rawDelivery, _ := json.Marshal(delivery)
			event.Message = string(rawDelivery)
			if scenario == "unknown field" {
				event.Message = strings.TrimSuffix(event.Message, "}") + `,"extra":true}`
			}
			if scenario != "missing preparation" {
				must(store.CreateActivity(ctx, prepEvent))
			}
			if scenario != "missing live proof" {
				must(store.CreateActivity(ctx, liveEvent))
			}
			if scenario != "missing delivery" {
				must(store.CreateActivity(ctx, event))
			}
			result, err := NativeReleaseDeliveryForAttempt(ctx, store, item, att.ID)
			valid := scenario == "valid" || scenario == "accepted task"
			if valid {
				if err != nil || result == nil || *result != delivery || result.SourceCommit == result.TargetCommit {
					t.Fatal("completed source→release lineage lost", err)
				}
				if ValidateNativeReleaseSource(ctx, store, item, &intent) == nil {
					t.Fatal("historical delivery reader reopened a finished release stage")
				}
				if scenario == "valid" {
					proof := LiveInspectionIntent{Purpose: NativeReleaseInspection, SourceAttemptID: att.ID, SourceCommit: intent.SourceCommit, TargetCommit: target, Repository: intent.Repository, OwnerID: intent.OwnerID, RoomID: intent.RoomID, SourceRequestID: intent.SourceRequestID, InstructionsHash: intent.InstructionsHash, ObservedAt: now}
					must(ValidateLiveInspectionSource(ctx, store, item, &proof))
					for _, change := range []string{"target", "owner", "request", "source attempt", "recovery", "old observation"} {
						forged := proof
						switch change {
						case "target":
							forged.TargetCommit = intent.SourceCommit
						case "owner":
							forged.OwnerID = "other"
						case "request":
							forged.SourceRequestID = "other"
						case "source attempt":
							forged.SourceAttemptID = source.ID
						case "recovery":
							forged.PreviousAttemptID = "other"
						case "old observation":
							forged.ObservedAt = started
						}
						if ValidateLiveInspectionSource(ctx, store, item, &forged) == nil {
							t.Fatalf("native verifier accepted changed %s", change)
						}
					}
					raw, _ := json.Marshal(proof)
					j := TaskLaunch{TaskID: item.ID, ProjectID: item.ProjectID, RequestID: proof.RequestID(item.ID), OwnerID: intent.OwnerID, RoomID: intent.RoomID, SourceRequestID: intent.SourceRequestID, AgentID: item.AssignedAgent, InstructionsHash: intent.InstructionsHash, Revision: "Scope: read-only repository. Do not push. Do not deploy.\n" + LiveInspectionMarker + string(raw), CreatedAt: now, AvailableAt: now, ExpiresAt: now.Add(time.Hour)}
					parsed, err := LiveInspectionForLaunch(item, &j)
					must(err)
					if parsed == nil || parsed.SourceAttemptID != proof.SourceAttemptID || parsed.TargetCommit != proof.TargetCommit || parsed.Purpose != proof.Purpose {
						t.Fatal("delivered-source inspection launch lost binding")
					}
					saved, err := svc.SaveLaunch(ctx, j)
					must(err)
					_, err = svc.ClaimLaunch(ctx, time.Now(), "dispatcher", saved.ID)
					must(err)
					if _, err := svc.CreateAttempt(ctx, item.ID, intent.OwnerID, CreateAttemptRequest{RunnerID: "verifier", AgentName: item.AssignedAgent, RequestID: j.RequestID, ExpectedPredecessorID: intent.SourceReviewAttemptID}); err == nil {
						t.Fatal("verifier bypassed native delivery predecessor")
					}
					verifier, err := svc.CreateAttempt(ctx, item.ID, intent.OwnerID, CreateAttemptRequest{RunnerID: "verifier", AgentName: item.AssignedAgent, RequestID: j.RequestID, ExpectedPredecessorID: att.ID})
					must(err)
					bound, err := ValidateLiveInspection(ctx, store, item, verifier.ID)
					must(err)
					if bound == nil || bound.TargetCommit != target || bound.SourceCommit != intent.SourceCommit {
						t.Fatal("verifier source/target changed")
					}
					limits, err := AttemptPublicationPolicy(ctx, store, item, verifier.ID)
					must(err)
					if !limits.ReadOnlyRepository || !limits.DenyPush || !limits.DenyDeploy {
						t.Fatal("native verifier gained implementation permission")
					}
					testStaleNativeInspectionRecovery(t, store, item, verifier, proof, j)
				}
			} else if err == nil || result != nil {
				t.Fatal("invalid delivery lineage accepted")
			}
		})
	}
}

func testStaleNativeInspectionRecovery(t *testing.T, store *MemoryStore, item *Task, verifier *TaskAttempt, original LiveInspectionIntent, launch TaskLaunch) {
	t.Helper()
	ctx := context.Background()
	start, finish, exit := verifier.CreatedAt, time.Now().UTC(), 1
	verifier.Status, verifier.StartedAt, verifier.FinishedAt, verifier.ExitCode = AttemptStatusFailed, &start, &finish, &exit
	verifier.ErrorMessage = StaleInspectionReleaseMessage
	base := *verifier
	proof := original
	proof.VerifierRecoveryAttemptID = verifier.ID
	proof.TargetCommit, proof.ObservedAt = strings.Repeat("d", 40), finish
	for _, scenario := range []string{"valid", "wrong owner", "wrong source", "same target", "generic failure", "cancelled", "still running", "missing finish"} {
		candidate, stopped := proof, base
		switch scenario {
		case "wrong owner":
			candidate.OwnerID = "foreign"
		case "wrong source":
			candidate.SourceCommit = strings.Repeat("f", 40)
		case "same target":
			candidate.TargetCommit = original.TargetCommit
		case "generic failure":
			stopped.ErrorMessage = "Project tests failed"
		case "cancelled":
			stopped.CancelRequested = true
		case "still running":
			stopped.Status = AttemptStatusRunning
		case "missing finish":
			stopped.FinishedAt = nil
		}
		// Separate fixture state per case; never revive a terminal attempt.
		variant := NewMemoryStore()
		variant.tasks, variant.activities, variant.launches = store.tasks, store.activities, store.launches
		for id, attempt := range store.attempts {
			variant.attempts[id] = attempt
		}
		variant.attempts[verifier.ID] = &stopped
		err := ValidateLiveInspectionSource(ctx, variant, item, &candidate)
		if (err == nil) != (scenario == "valid") {
			t.Fatalf("stale native recovery %s: %v", scenario, err)
		}
	}
	if err := store.UpdateAttempt(ctx, &base); err != nil {
		t.Fatal(err)
	}
	if proof.RequestID(item.ID) == original.RequestID(item.ID) || proof.PredecessorAttemptID() != verifier.ID {
		t.Fatal("recovery reused stopped launch or original release predecessor")
	}
	// Parsing and admission retain the exact recovery binding and read-only scope.
	raw, _ := json.Marshal(proof)
	launch.RequestID, launch.ID = proof.RequestID(item.ID), ""
	launch.Revision = "Scope: read-only repository. Do not push. Do not deploy.\n" + LiveInspectionMarker + string(raw)
	launch.CreatedAt, launch.AvailableAt = finish, finish
	if parsed, err := LiveInspectionForLaunch(item, &launch); err != nil || parsed == nil || !reflect.DeepEqual(*parsed, proof) {
		t.Fatal("native recovery launch lost exact identity", err)
	}
	// A completed model report can never be reclassified as a pre-model stop.
	if err := store.CreateActivity(ctx, &TaskActivity{ID: "unexpected-report", TaskID: item.ID, AttemptID: &verifier.ID, ActivityType: "agent_report", Message: "Provider result exists", CreatedAt: finish}); err != nil {
		t.Fatal(err)
	}
	if ValidateLiveInspectionSource(ctx, store, item, &proof) == nil {
		t.Fatal("stale-target recovery discarded an existing model result")
	}
}
