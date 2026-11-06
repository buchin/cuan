package task

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestExecutionInfoUsesExactAttemptWithoutMutatingPendingLaunch(t *testing.T) {
	ctx := context.Background()
	store := NewMemoryStore()
	svc := NewService(store, nil)
	created, err := svc.CreateTask(ctx, "owner", CreateTaskRequest{ProjectID: "project", Title: "PRIVATE_TITLE", Instructions: "Approved work PRIVATE_CONTENT"})
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	old := TaskLaunch{TaskID: created.ID, ProjectID: "project", RequestID: "old-launch", SourceRequestID: "owner", OwnerID: "owner", RoomID: "91000000-0000-4000-8000-000000000001", AgentID: "agent", InstructionsHash: LaunchInstructionsHash(created.Instructions), Revision: "Scope: read-only repository. Do not push. Do not deploy. PRIVATE_REVISION", CreatedAt: now, AvailableAt: now, ExpiresAt: now.Add(time.Hour)}
	savedLaunch, err := store.SaveLaunch(ctx, old)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.ClaimLaunch(ctx, now.Add(time.Millisecond), "worker", savedLaunch.ID); err != nil {
		t.Fatal(err)
	}
	a := &TaskAttempt{ID: attemptRequestID(created.ID, old.RequestID), TaskID: created.ID, AgentName: "agent", Status: AttemptStatusFailed, CreatedAt: now}
	if err := store.CreateAttempt(ctx, a); err != nil {
		t.Fatal(err)
	}
	next := old
	next.RequestID = "new-launch"
	next.Revision = "Implement the original approved change"
	next.CreatedAt = now.Add(time.Second)
	next.AvailableAt = next.CreatedAt
	if _, err := store.SaveLaunch(ctx, next); err != nil {
		t.Fatal(err)
	}
	before, _ := store.GetTask(ctx, created.ID)
	info, err := svc.ExecutionInfo(ctx, created.ID)
	if err != nil || info.AttemptID != a.ID || info.AttemptStatus != AttemptStatusFailed || !info.IntentParsed || info.IntentKind != "execution" || !info.Scope.ReadOnlyRepository || !info.Scope.DenyPush || !info.Scope.DenyDeploy {
		t.Fatal("pending launch replaced exact execution intent", err, info)
	}
	if info.LatestLaunch == nil || info.LatestLaunch.ID != LaunchID(created.ID, next.RequestID) || info.LatestLaunch.State != "queued" || info.LatestLaunch.IntentKind != "execution" {
		t.Fatal("latest admission not separately identified", info.LatestLaunch)
	}
	raw, _ := json.Marshal(info)
	if strings.Contains(string(raw), "PRIVATE_") {
		t.Fatal("private launch content leaked")
	}
	latest, _ := store.LatestLaunch(ctx, created.ID)
	attempts, _ := store.ListAttempts(ctx, created.ID)
	saved, _ := store.GetTask(ctx, created.ID)
	if latest.RequestID != next.RequestID || latest.State != "queued" || len(attempts) != 1 || !saved.UpdatedAt.Equal(before.UpdatedAt) {
		t.Fatal("inspection changed authoritative state")
	}
}

func TestExecutionInfoDiagnosesRejectedAdmissionWithoutAttempt(t *testing.T) {
	ctx := context.Background()
	store := NewMemoryStore()
	svc := NewService(store, nil)
	created, err := svc.CreateTask(ctx, "owner", CreateTaskRequest{ProjectID: "project", Title: "PRIVATE_TITLE", Instructions: "PRIVATE_INSTRUCTIONS"})
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	j, err := store.SaveLaunch(ctx, TaskLaunch{TaskID: created.ID, ProjectID: created.ProjectID, RequestID: "pending", SourceRequestID: "owner", OwnerID: "PRIVATE_OWNER", RoomID: "room", AgentID: "agent", InstructionsHash: LaunchInstructionsHash(created.Instructions), Revision: LiveInspectionMarker + "PRIVATE_MALFORMED_JSON", CreatedAt: now, AvailableAt: now, ExpiresAt: now.Add(time.Hour)})
	if err != nil {
		t.Fatal(err)
	}
	_, parseErr := LiveInspectionForLaunch(created, j)
	if !errors.Is(parseErr, ErrInvalidTaskInput) {
		t.Fatal("validation error lost input classification", parseErr)
	}
	info, err := svc.ExecutionInfo(ctx, created.ID)
	if err != nil || info.IntentKind != "no_attempt" || info.LatestLaunch == nil || info.LatestLaunch.ValidationCode != "live_inspection_malformed_intent" {
		t.Fatal("missing admission diagnosis", info, err)
	}
	raw, _ := json.Marshal(info)
	if strings.Contains(string(raw), "PRIVATE_") {
		t.Fatal("diagnosis leaked private input")
	}
	attempts, _ := store.ListAttempts(ctx, created.ID)
	if len(attempts) != 0 {
		t.Fatal("diagnosis started work")
	}
}
