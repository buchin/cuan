package server

import (
	"context"
	"strings"
	"time"

	"github.com/buchin/cuan/internal/task"
)

func isReconciliationOperation(d *task.TaskDetailResponse) bool {
	return d != nil && d.ExecutionCategory == task.ExecutionCategoryOps && strings.Contains(d.Instructions, task.ReconciliationExecutionMarker)
}
func requireOperationalOutcome(d *task.TaskDetailResponse, decision inboxReviewEvaluation) inboxReviewEvaluation {
	if d == nil || (d.ExecutionCategory != task.ExecutionCategoryOps && !task.IsAccountOutcome(&d.Task)) {
		return decision
	}
	if decision.RequestedOutcome == "unfinished" && decision.Decision == "pass" {
		decision.Decision = "revise"
		decision.RemainingWork = "implementation"
		decision.Revision = "Complete only the remaining originally authorized operation. First inspect current state, preserve successful effects and verify the original acceptance criteria. Do not repeat an uncertain effect or expand scope."
		decision.Explanation = "Operasi yang diminta belum selesai. Cuan melanjutkan sisa pekerjaan dalam izin awal setelah memeriksa keadaan terkini."
		return decision
	}
	if decision.RequestedOutcome == "needs_input" || (decision.Decision == "pass" && decision.RequestedOutcome != "completed") {
		decision.Decision = "blocked"
		decision.RemainingWork = "external"
		decision.Revision = ""
		if decision.RequestedOutcome == "" {
			decision.Finding = "Hasil operasi yang diminta belum terverifikasi."
			decision.Explanation = "Pemeriksaan telah dilaporkan, tetapi penyelesaian operasi yang diminta belum terverifikasi. Pekerjaan belum diterima sebagai selesai."
		}
	}
	return decision
}

// Repair only accepted reconciliation operations reviewed before the requested-
// outcome policy. This re-evaluates saved evidence, never executes business work,
// resets attempts, changes scope or reopens canceled/ordinary accepted tasks.
func (s *Server) reconcileOperationalAcceptances(ctx context.Context) {
	reader, ok := s.taskSvc.Store().(task.OperationalAcceptanceAuditStore)
	if !ok {
		return
	}
	ids, err := reader.OperationalAcceptanceTaskIDs(ctx)
	if err != nil {
		return
	}
	for _, id := range ids {
		d, err := s.taskSvc.GetTaskDetail(ctx, id)
		if err != nil || !isReconciliationOperation(d) || d.Status != task.TaskStatusDone || d.LatestAttempt == nil {
			continue
		}
		a := d.LatestAttempt
		if a.Status != task.AttemptStatusSucceeded || a.CancelRequested || a.CanceledAt != nil || (d.Launch != nil && (d.Launch.State == "queued" || d.Launch.State == "claimed" || d.Launch.State == "cancelled")) {
			continue
		}
		assessment, found := reviewAssessment(d, "")
		if !found || assessment.Decision != "pass" {
			continue
		}
		state := "verified"
		if assessment.RequestedOutcome != "completed" {
			if changed, err := reader.RequeueOperationalAcceptance(ctx, id, a.ID, d.UpdatedAt); err != nil || !changed {
				continue
			}
			state = "review_required"
		}
		_ = s.taskSvc.Store().CreateActivity(ctx, &task.TaskActivity{ID: "operational-audit:" + a.ID, TaskID: id, AttemptID: &a.ID, ActivityType: task.OperationalAcceptanceAudit, Author: "coordinator", NewStatus: state, Message: "Re-evaluate the original requested operation from saved evidence; no execution was started.", CreatedAt: time.Now().UTC()})
	}
}
