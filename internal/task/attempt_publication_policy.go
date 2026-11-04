package task

import (
	"context"
	"github.com/buchin/cuan/internal/executionpolicy"
)

// Revision restrictions belong to the exact execution intent, not whichever
// launch happens to be newest by the time a runner reports completion.
type AttemptLaunchStore interface {
	LaunchForAttempt(context.Context, string) (*TaskLaunch, error)
}

func AttemptPublicationPolicy(ctx context.Context, store Store, t *Task, attemptID string) (executionpolicy.PublicationPolicy, error) {
	policy := executionpolicy.FromInstructions(t.Instructions)
	if launches, ok := store.(AttemptLaunchStore); ok {
		launch, err := launches.LaunchForAttempt(ctx, attemptID)
		if err != nil {
			return policy, err
		}
		if launch != nil && launch.TaskID == t.ID {
			revision := executionpolicy.FromInstructions(launch.Revision)
			policy.DenyPush = policy.DenyPush || revision.DenyPush
			policy.DenyDeploy = policy.DenyDeploy || revision.DenyDeploy
		}
	}
	return policy, nil
}
func (s *Service) AttemptPublicationPolicy(ctx context.Context, taskID, attemptID string) (executionpolicy.PublicationPolicy, error) {
	t, err := s.store.GetTask(ctx, taskID)
	if err != nil {
		return executionpolicy.PublicationPolicy{}, err
	}
	return AttemptPublicationPolicy(ctx, s.store, t, attemptID)
}
func (s *DBStore) LaunchForAttempt(ctx context.Context, attemptID string) (*TaskLaunch, error) {
	return scanLaunch(s.db.QueryRowContext(ctx, `SELECT `+launchColumns+` FROM task_launches WHERE attempt_id=$1`, attemptID))
}
func (m *MemoryStore) LaunchForAttempt(_ context.Context, attemptID string) (*TaskLaunch, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, j := range m.launches {
		if attemptRequestID(j.TaskID, j.RequestID) == attemptID {
			return copyLaunch(j), nil
		}
	}
	return nil, nil
}
