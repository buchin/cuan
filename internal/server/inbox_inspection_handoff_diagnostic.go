package server

import (
	"context"

	"github.com/buchin/cuan/internal/db"
	"github.com/buchin/cuan/internal/task"
)

func (s *Server) inspectionHandoffDiagnostic(ctx context.Context, d *task.TaskDetailResponse) *task.InspectionHandoffDiagnostic {
	if d == nil || d.LatestAttempt == nil {
		return nil
	}
	intent, err := task.LiveInspectionForAttempt(ctx, s.taskSvc.Store(), &d.Task, d.LatestAttempt.ID)
	if err != nil || intent == nil || intent.Purpose != task.CurrentOutcomeInspection || intent.SourceCommit == "" {
		return nil
	}
	out := &task.InspectionHandoffDiagnostic{Code: "source_correction_evidence_incomplete"}
	out.ApprovedScope, _ = s.inboxApprovedTaskScope(ctx, d)
	out.SourceCorrectionAllowed = s.inspectedSourceCorrectionAllowed(ctx, db.InboxRequest{OwnerID: intent.OwnerID, RoomID: intent.RoomID}, d)
	if publication := s.inboxReviewPublication(ctx, d); publication != nil {
		out.PublicationRequired, out.SourceContained, out.RunningCommit = &publication.Required, publication.ResultCommitContained, publication.ObservedRelease
	}
	if out.SourceCorrectionAllowed {
		out.Code = "publication_observation_incomplete"
		if out.SourceContained != nil {
			out.Code = "source_already_contained"
			if !*out.SourceContained {
				out.Code = "separate_developer_reconciliation_required"
			}
		}
	}
	return out
}
