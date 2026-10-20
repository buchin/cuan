package task

import (
	"context"
	"time"
)

// RecordGitHubPrerequisite persists a system event, separate from human instructions.
func (s *Service) RecordGitHubPrerequisite(ctx context.Context, taskID, attemptID string, resolved bool) error {
	current, err := s.store.GetTask(ctx, taskID)
	if err != nil {
		return err
	}
	kind := "github_connection_required"
	message := "Connect GitHub to save this task branch. Your local work is retained.\nContext fingerprint: " + LaunchInstructionsHash(current.Instructions)
	if resolved {
		kind = "github_connection_resolved"
		message = "The task branch was saved to GitHub."
	}
	activities, err := s.store.ListActivities(ctx, taskID)
	if err != nil {
		return err
	}
	var latest *TaskActivity
	for _, a := range activities {
		if (a.ActivityType == "github_connection_required" || a.ActivityType == "github_connection_resolved") && (latest == nil || a.CreatedAt.After(latest.CreatedAt)) {
			latest = a
		}
	}
	if latest != nil && latest.ActivityType == kind {
		return nil
	}
	return s.store.CreateActivity(ctx, &TaskActivity{ID: generateID("act"), TaskID: taskID, AttemptID: &attemptID, ActivityType: kind, Author: "Cuan", Message: message, CreatedAt: time.Now().UTC()})
}
