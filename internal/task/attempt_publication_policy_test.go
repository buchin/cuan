package task

import (
	"context"
	"testing"
	"time"
)

func TestAttemptPublicationPolicyMemory(t *testing.T) {
	attemptPublicationPolicyContract(t, NewMemoryStore(), "launch-project")
}
func attemptPublicationPolicyContract(t *testing.T, store Store, projectID string) {
	t.Helper()
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Microsecond)
	svc := NewService(store, nil)
	created, err := svc.CreateTask(ctx, "owner", CreateTaskRequest{ProjectID: projectID, Title: "Preserve report", Instructions: "Do not push. Inspect saved work."})
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
