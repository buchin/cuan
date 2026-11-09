package task

import (
	"context"
	"testing"
	"time"
)

func TestReadOnlyCategoryBindsPublicationWithoutPromptKeywords(t *testing.T) {
	item := &Task{ExecutionCategory: ExecutionCategoryReadOnly, Instructions: "Read current account state and deliver an attached report."}
	policy := ScopePublicationPolicy(item)
	if !policy.ReadOnlyRepository || !policy.DenyPush || !policy.DenyDeploy {
		t.Fatalf("saved readonly category not enforced: %+v", policy)
	}
	continued := launchPublicationPolicy(item, &TaskLaunch{Revision: "Continue and publish the report"}, false)
	if continued != policy {
		t.Fatalf("continuation relaxed readonly scope: %+v", continued)
	}
}

func TestAttemptPublicationPolicyMemory(t *testing.T) {
	attemptPublicationPolicyContract(t, NewMemoryStore(), "launch-project")
}
func attemptPublicationPolicyContract(t *testing.T, store Store, projectID string) {
	t.Helper()
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Microsecond)
	svc := NewService(store, nil)
	created, err := svc.CreateTask(ctx, "owner", CreateTaskRequest{ProjectID: projectID, Title: "Preserve report", Instructions: "Do not push. Inspect saved work.", ExecutionCategory: ExecutionCategoryCode})
	if err != nil {
		t.Fatal(err)
	}
	launches := store.(LaunchStore)
	old := TaskLaunch{TaskID: created.ID, ProjectID: projectID, RequestID: "limited-attempt", SourceRequestID: "owner", OwnerID: "owner", RoomID: "91000000-0000-4000-8000-000000000001", AgentID: "agent", InstructionsHash: LaunchInstructionsHash(created.Instructions), Revision: "Scope: read-only repository. Do not deploy. Verify local evidence only.", CreatedAt: now, AvailableAt: now, ExpiresAt: now.Add(time.Hour)}
	if _, err = launches.SaveLaunch(ctx, old); err != nil {
		t.Fatal(err)
	}
	next := old
	next.RequestID = "later-attempt"
	next.Revision = "Continue original approved work"
	next.CreatedAt = now.Add(time.Second)
	next.AvailableAt = next.CreatedAt
	if _, err = launches.SaveLaunch(ctx, next); err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct {
		id     string
		deploy bool
	}{{attemptRequestID(created.ID, old.RequestID), true}, {attemptRequestID(created.ID, next.RequestID), false}, {"unrelated-attempt", false}} {
		policy, e := AttemptPublicationPolicy(ctx, store, created, c.id)
		if e != nil || !policy.DenyPush || policy.DenyDeploy != c.deploy || policy.ReadOnlyRepository != c.deploy {
			t.Fatalf("attempt %s: %+v %v", c.id, policy, e)
		}
	}
}

func TestEvidenceCautionCannotRelaxOriginalPublicationBan(t *testing.T) {
	revision := &TaskLaunch{Revision: "Do not repeat deployment solely to obtain evidence. Deploy the verified correction to production."}
	for _, instructions := range []string{"Do not deploy.", "Scope: read-only repository. Deploy production."} {
		item := &Task{ExecutionCategory: ExecutionCategoryCode, Instructions: instructions}
		if got := launchPublicationPolicy(item, revision, false); !got.DenyDeploy {
			t.Fatalf("correction relaxed original restriction: %+v", got)
		}
	}
	allowed := &Task{ExecutionCategory: ExecutionCategoryCode, Instructions: "Implement and deploy the verified correction to production."}
	if got := launchPublicationPolicy(allowed, revision, false); got.DenyDeploy {
		t.Fatalf("evidence caution stranded approved publication: %+v", got)
	}
	if got := launchPublicationPolicy(allowed, revision, true); !got.ReadOnlyRepository || !got.DenyDeploy || !got.DenyPush {
		t.Fatalf("verifier boundary relaxed: %+v", got)
	}
}

func TestIndonesianRepeatEvidenceCautionRetainsAttemptBoundaries(t *testing.T) {
	item := &Task{ExecutionCategory: ExecutionCategoryCode, Instructions: "Native deploy dan exact health check pada rilis baru. Jangan mengulang broad suite atau deploy hanya demi bukti bila bukti native yang sah sudah ada."}
	launch := &TaskLaunch{Revision: "Continue original approved work."}
	if got := launchPublicationPolicy(item, launch, false); got.ReadOnlyRepository || got.DenyPush || got.DenyDeploy {
		t.Fatalf("repeat-effect caution blocked approved release: %+v", got)
	}
	if got := launchPublicationPolicy(item, launch, true); !got.ReadOnlyRepository || !got.DenyPush || !got.DenyDeploy {
		t.Fatalf("source verifier lost independent read-only scope: %+v", got)
	}
	launch.Revision = "Do not deploy. Inspect the existing result only."
	if got := launchPublicationPolicy(item, launch, false); !got.DenyDeploy {
		t.Fatalf("original authorization overrode this attempt's explicit ban: %+v", got)
	}
}
