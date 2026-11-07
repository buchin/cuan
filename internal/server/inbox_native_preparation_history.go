package server

import (
	"context"

	"github.com/buchin/cuan/internal/task"
)

// Review-only historical facts do not resurrect a failed native launch or grant
// publication. Current live publication and original acceptance stay separate.
func (s *Server) inboxNativePreparationHistory(ctx context.Context, d *task.TaskDetailResponse) any {
	prepared, status := s.validatedNativePreparationHistory(ctx, d)
	if prepared == nil {
		return nil
	}
	return map[string]any{"preparation": prepared, "attempt_status": status, "preparation_only": true, "delivery_not_established_by_preparation": true}
}

func (s *Server) validatedNativePreparationHistory(ctx context.Context, d *task.TaskDetailResponse) (*task.NativeReleasePreparation, string) {
	if d == nil || s.taskSvc == nil {
		return nil, ""
	}
	for _, attempt := range d.Attempts {
		if attempt.Status != task.AttemptStatusSucceeded && attempt.Status != task.AttemptStatusFailed {
			continue
		}
		found := false
		for _, event := range d.Activities {
			if event != nil && event.ID == "native-release-preparation:"+attempt.ID {
				found = true
				break
			}
		}
		if !found {
			continue
		}
		prepared, err := task.HistoricalNativeReleasePreparationForAttempt(ctx, s.taskSvc.Store(), &d.Task, attempt.ID)
		if err != nil || prepared == nil {
			continue
		}
		return prepared, attempt.Status
	}
	return nil, ""
}

// This only permits a read-only inspection of an already observed outcome. It
// does not publish local work or count preparation as delivery/acceptance.
func (s *Server) preparedSourceAlreadyPublished(ctx context.Context, d *task.TaskDetailResponse) bool {
	prepared, _ := s.validatedNativePreparationHistory(ctx, d)
	if d == nil || prepared == nil || prepared.SourceCommit != d.CommitSHA {
		return false
	}
	proof := &task.LiveInspectionIntent{Purpose: task.CurrentOutcomeInspection, PreparedPublicationAttemptID: prepared.AttemptID, SourceCommit: d.CommitSHA, Repository: prepared.Repository}
	if task.ValidatePreparedPublicationInspection(ctx, s.taskSvc.Store(), &d.Task, proof) != nil {
		return false
	}
	observed := s.inboxReviewPublication(ctx, d)
	return preparedSourceMatchesPublication(prepared, d.CommitSHA, observed)
}

func preparedSourceMatchesPublication(prepared *task.NativeReleasePreparation, source string, observed *reviewPublication) bool {
	return prepared != nil && prepared.SourceCommit == source && observed != nil && observed.Required && observed.Verified && observed.Commit == source && observed.ObservedRelease != "" && observed.ResultCommitContained != nil && *observed.ResultCommitContained
}
