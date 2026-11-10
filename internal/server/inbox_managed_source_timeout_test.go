package server

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/buchin/cuan/internal/db"
	"github.com/buchin/cuan/internal/task"
)

func TestManagedSourceTimeoutPlansCompletionInsteadOfProductionAudit(t *testing.T) {
	now, exit := time.Now().UTC(), 124
	start := now.Add(-time.Minute)
	item := task.Task{ID: "work", ProjectID: "project", SourceRoomID: "room", ExecutionCategory: task.ExecutionCategoryCode, Status: task.TaskStatusBlocked, Instructions: "Implement and publish the approved work"}
	attempt := &task.TaskAttempt{ID: "dev", TaskID: item.ID, RunnerID: "runner", Status: task.AttemptStatusFailed, StartedAt: &start, FinishedAt: &now, ExitCode: &exit, ErrorMessage: "Execution timed out after adaptive timeout (25m0s)"}
	claim := task.ManagedReleaseClaim{Version: 1, TaskID: item.ID, ProjectID: item.ProjectID, RoomID: item.SourceRoomID, AttemptID: attempt.ID, RunnerID: attempt.RunnerID, InstructionsHash: task.LaunchInstructionsHash(item.Instructions), Repository: "owner/repo", TargetBranch: "dev"}
	raw, _ := json.Marshal(claim)
	event := &task.TaskActivity{ID: "managed-release-claim:" + attempt.ID, TaskID: item.ID, AttemptID: &attempt.ID, Author: attempt.RunnerID, ActivityType: "managed_release_claim", Message: string(raw), CreatedAt: start.Add(time.Second)}
	detail := &task.TaskDetailResponse{Task: item, LatestAttempt: attempt, Activities: []*task.TaskActivity{event}}
	// No production observer is supplied: incomplete source must be routed
	// before attempting to observe an unrelated live release.
	s := &Server{}
	proof, code := s.planLiveResultInspection(context.Background(), detail, db.InboxRequest{}, attempt)
	if proof != nil || code != "managed_source_timeout_requires_completion" {
		t.Fatalf("wrong recovery target: %v %s", proof, code)
	}
}
