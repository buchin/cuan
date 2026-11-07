package runner

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/buchin/cuan/internal/project"
	"github.com/buchin/cuan/internal/task"
)

func TestManagedReleaseAdmissionRequiresExplicitFrozenScope(t *testing.T) {
	for _, scenario := range []string{"valid", "unclaimed", "old runner", "disabled", "not exact opt in", "no auto deploy", "no push", "no deploy", "read only", "no inbox", "ops", "verifier", "inspection", "release", "foreign runner", "cancelled", "missing target"} {
		t.Run(scenario, func(t *testing.T) {
			store := task.NewMemoryStore()
			service := NewService(nil, store)
			now := time.Now().UTC()
			item := &task.Task{ID: "task", ProjectID: "project", SourceRoomID: "room", ExecutionCategory: task.ExecutionCategoryCode, Instructions: "Implement and deploy the approved change"}
			att := &task.TaskAttempt{ID: "attempt", TaskID: item.ID, RunnerID: "runner", Status: task.AttemptStatusRunning, StartedAt: &now}
			claim := &ClaimTaskResponse{Claimed: true, Task: item, Attempt: att, AutoDeploy: true, Environment: "CUAN_MANAGED_RELEASE=1\nPRIVATE_SECRET=do-not-record", RepoOwner: "owner", RepoName: "repo", DeployBranch: "dev"}
			req := ClaimTaskRequest{RunnerID: "runner", SupportsNativeRelease: true}
			invalid := false
			switch scenario {
			case "unclaimed":
				claim.Claimed = false
			case "old runner":
				req.SupportsNativeRelease = false
			case "disabled":
				claim.Environment = ""
			case "not exact opt in":
				claim.Environment = "CUAN_MANAGED_RELEASE=true"
			case "no auto deploy":
				claim.AutoDeploy = false
			case "no push":
				claim.PublicationLimits.DenyPush = true
			case "no deploy":
				claim.PublicationLimits.DenyDeploy = true
			case "read only":
				claim.PublicationLimits.ReadOnlyRepository = true
			case "no inbox":
				item.SourceRoomID = ""
			case "ops":
				item.ExecutionCategory = task.ExecutionCategoryOps
			case "verifier":
				claim.Verification = &task.VerificationIntent{}
			case "inspection":
				claim.LiveInspection = &task.LiveInspectionIntent{}
			case "release":
				claim.NativeRelease = &task.NativeReleaseIntent{}
			case "foreign runner":
				att.RunnerID = "foreign"
				invalid = true
			case "cancelled":
				att.CancelRequested = true
				invalid = true
			case "missing target":
				claim.DeployBranch = ""
				invalid = true
			}
			err := service.bindManagedReleaseClaim(context.Background(), req, claim)
			if (err != nil) != invalid {
				t.Fatalf("unexpected admission error: %v", err)
			}
			activities, err := store.ListActivities(context.Background(), item.ID)
			if err != nil {
				t.Fatal(err)
			}
			if scenario != "valid" {
				if claim.ManagedRelease != nil || len(activities) != 0 {
					t.Fatal("unqualified attempt received publication ownership")
				}
				return
			}
			if claim.ManagedRelease == nil || len(activities) != 1 {
				t.Fatal("missing durable binding")
			}
			var saved ManagedReleaseClaim
			event := activities[0]
			if json.Unmarshal([]byte(event.Message), &saved) != nil || saved != *claim.ManagedRelease || saved.InstructionsHash != task.LaunchInstructionsHash(item.Instructions) || saved.Repository != "owner/repo" || saved.TargetBranch != "dev" || event.ID != "managed-release-claim:"+att.ID || event.Author != req.RunnerID || event.AttemptID == nil || *event.AttemptID != att.ID {
				t.Fatal("claim ownership was not pinned")
			}
			if strings.Contains(event.Message, "PRIVATE_SECRET") || strings.Contains(event.Message, "do-not-record") {
				t.Fatal("environment leaked into receipt")
			}
			claim.DeployBranch = "changed"
			item.Instructions = "changed"
			if saved.TargetBranch != "dev" || saved.InstructionsHash == task.LaunchInstructionsHash(item.Instructions) {
				t.Fatal("binding followed mutable configuration")
			}
		})
	}
}

func TestManagedReleaseCompletionRejectsChangedBinding(t *testing.T) {
	for _, scenario := range []string{"valid", "absent", "scope", "project", "room", "runner", "author", "receipt attempt", "type", "unknown field", "trailing data", "future", "before start", "cancelled", "repo", "branch", "disabled", "missing context"} {
		t.Run(scenario, func(t *testing.T) {
			ctx := context.Background()
			store := task.NewMemoryStore()
			now := time.Now().Add(-time.Second).UTC()
			item := &task.Task{ID: "task", ProjectID: "project", SourceRoomID: "room", ExecutionCategory: task.ExecutionCategoryCode, Instructions: "Implement and deploy"}
			att := &task.TaskAttempt{ID: "attempt", TaskID: item.ID, RunnerID: "runner", Status: task.AttemptStatusRunning, StartedAt: &now}
			p := &project.ProjectResponse{ID: "project", RepoOwner: "owner", RepoName: "repo", DefaultBranch: "dev", Metadata: json.RawMessage(`{"cuan_settings":{"auto_deploy":true,"environment":"CUAN_MANAGED_RELEASE=1"}}`)}
			finder := &mockProjectFinder{projects: map[string]*project.ProjectResponse{"project": p}}
			svc := NewService(nil, store, finder)
			binding := ManagedReleaseClaim{Version: 1, TaskID: item.ID, ProjectID: item.ProjectID, RoomID: item.SourceRoomID, AttemptID: att.ID, RunnerID: att.RunnerID, InstructionsHash: task.LaunchInstructionsHash(item.Instructions), Repository: "owner/repo", TargetBranch: "dev"}
			raw, _ := json.Marshal(binding)
			event := &task.TaskActivity{ID: "managed-release-claim:" + att.ID, TaskID: item.ID, AttemptID: &att.ID, Author: att.RunnerID, ActivityType: managedReleaseClaimActivity, Message: string(raw), CreatedAt: now.Add(time.Millisecond)}
			switch scenario {
			case "project":
				item.ProjectID = "changed"
			case "room":
				item.SourceRoomID = "other"
			case "scope":
				item.Instructions += " changed"
			case "runner":
				att.RunnerID = "other"
			case "author":
				event.Author = "model"
			case "receipt attempt":
				wrong := "other"
				event.AttemptID = &wrong
			case "type":
				event.ActivityType = "comment"
			case "unknown field":
				event.Message = strings.TrimSuffix(event.Message, "}") + `,"extra":true}`
			case "trailing data":
				event.Message += " {}"
			case "future":
				event.CreatedAt = time.Now().Add(time.Hour)
			case "before start":
				event.CreatedAt = now.Add(-time.Second)
			case "cancelled":
				att.CancelRequested = true
			case "repo":
				p.RepoName = "foreign"
			case "branch":
				p.DefaultBranch = "main"
			case "disabled":
				p.Metadata = json.RawMessage(`{"cuan_settings":{"auto_deploy":false}}`)
			case "missing context":
				delete(finder.projects, "project")
			}
			if scenario != "absent" {
				if err := store.CreateActivity(ctx, event); err != nil {
					t.Fatal(err)
				}
			}
			result, err := svc.managedReleaseClaimForCompletion(ctx, item, att)
			if scenario == "absent" {
				if err != nil || result != nil {
					t.Fatal("setting retroactively created ownership")
				}
				return
			}
			if scenario == "valid" {
				if err != nil || result == nil || *result != binding {
					t.Fatalf("valid claim rejected: %v", err)
				}
				return
			}
			if err == nil || result != nil {
				t.Fatal("invalid admission accepted")
			}
		})
	}
}
