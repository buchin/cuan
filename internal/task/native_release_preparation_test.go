package task

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestNativeReleasePreparationPreservesSourceTargetAndRejectsForgedRecords(t *testing.T) {
	for _, scenario := range []string{"valid", "missing", "source revoked", "cancelled", "expired", "author", "type", "attempt", "source", "target", "repository", "branch", "manifest", "scope", "future", "before start", "unknown field", "trailing"} {
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
			started := time.Now().UTC()
			att.Status = AttemptStatusRunning
			att.RunnerID = "native"
			att.StartedAt = &started
			must(store.UpdateAttempt(ctx, att))
			receipt := NativeReleasePreparation{Version: 1, TaskID: item.ID, AttemptID: att.ID, SourceCommit: intent.SourceCommit, TargetCommit: strings.Repeat("c", 40), InstructionsHash: intent.InstructionsHash, Repository: intent.Repository, TargetBranch: intent.TargetBranch, ManifestHash: intent.ManifestHash}
			event := &TaskActivity{ID: "native-release-preparation:" + att.ID, TaskID: item.ID, AttemptID: &att.ID, Author: "cuan-native-release", ActivityType: ActivityNativeReleasePreparation, CreatedAt: time.Now().UTC()}
			switch scenario {
			case "source revoked":
				source.CancelRequested = true
				must(store.UpdateAttempt(ctx, source))
			case "cancelled":
				att.CancelRequested = true
				must(store.UpdateAttempt(ctx, att))
			case "expired":
				expired := time.Now().Add(-time.Second)
				att.LeaseExpiresAt = &expired
				must(store.UpdateAttempt(ctx, att))
			case "author":
				event.Author = att.RunnerID
			case "type":
				event.ActivityType = "agent_report"
			case "attempt":
				receipt.AttemptID = "other"
			case "source":
				receipt.SourceCommit = strings.Repeat("d", 40)
			case "target":
				receipt.TargetCommit = "HEAD"
			case "repository":
				receipt.Repository = "foreign/repo"
			case "branch":
				receipt.TargetBranch = "main"
			case "manifest":
				receipt.ManifestHash = strings.Repeat("d", 64)
			case "scope":
				receipt.InstructionsHash = "changed"
			case "future":
				event.CreatedAt = time.Now().Add(time.Hour)
			case "before start":
				event.CreatedAt = started.Add(-time.Second)
			}
			raw, _ := json.Marshal(receipt)
			event.Message = string(raw)
			if scenario == "unknown field" {
				event.Message = strings.TrimSuffix(event.Message, "}") + `,"extra":true}`
			}
			if scenario == "trailing" {
				event.Message += " {}"
			}
			if scenario != "missing" {
				must(store.CreateActivity(ctx, event))
			}
			result, err := NativeReleasePreparationForAttempt(ctx, store, item, att.ID)
			if scenario == "valid" {
				if err != nil || result == nil || *result != receipt || result.SourceCommit == result.TargetCommit {
					t.Fatal("source→target mapping lost", err)
				}
				activities, _ := store.ListActivities(ctx, item.ID)
				if VerifiedDeployment(activities, att, result.TargetCommit, time.Now()) {
					t.Fatal("preparation became live proof")
				}
			} else if err == nil || result != nil {
				t.Fatal("invalid preparation accepted")
			}
		})
	}
}
