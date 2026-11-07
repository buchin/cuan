package task

import (
	"context"
	"encoding/json"
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
			att, err := svc.CreateAttempt(ctx, item.ID, "owner", CreateAttemptRequest{RunnerID: "native", AgentName: launch.AgentID, RequestID: launch.RequestID, ExpectedPredecessorID: source.ID})
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
			} else if err == nil || result != nil {
				t.Fatal("invalid delivery lineage accepted")
			}
		})
	}
}
