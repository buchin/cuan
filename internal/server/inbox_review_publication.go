package server

import (
	"context"
	"strings"
	"time"

	"github.com/buchin/cuan/internal/db"
	"github.com/buchin/cuan/internal/executionpolicy"
	"github.com/buchin/cuan/internal/project"
	"github.com/buchin/cuan/internal/task"
)

const inboxPublicationReviewPolicy = "Production publication review policy: v1"

type reviewPublication struct {
	Required        bool   `json:"required"`
	Verified        bool   `json:"verified"`
	Commit          string `json:"result_commit"`
	Provenance      string `json:"provenance,omitempty"`
	ConnectionState string `json:"connection_state,omitempty"`
}

// The reviewer cannot waive the same project publication gate enforced by the
// runner, or mistake a PR or the running process's unrelated build for delivery.
func (s *Server) inboxReviewPublication(ctx context.Context, d *task.TaskDetailResponse) *reviewPublication {
	if d == nil || d.LatestAttempt == nil || s.projectSvc == nil || d.ProjectID == "" || !releaseObjectID.MatchString(d.CommitSHA) || (d.ExecutionCategory != "" && d.ExecutionCategory != task.ExecutionCategoryCode) {
		return nil
	}
	_, err := s.taskPublicationPolicy(ctx, d)
	if err != nil {
		return &reviewPublication{Required: true, Commit: d.CommitSHA}
	}
	base := executionpolicy.FromInstructions(d.Instructions)
	if base.DenyDeploy || base.ReadOnlyRepository {
		return nil
	}
	p, err := s.projectSvc.Get(ctx, d.ProjectID)
	if err != nil || p == nil {
		return &reviewPublication{Required: true, Commit: d.CommitSHA}
	}
	if !project.SettingsFromMetadata(p.Metadata).IsAutoDeployEnabled() {
		return nil
	}
	result := &reviewPublication{Required: true, Commit: d.CommitSHA, ConnectionState: s.publicationConnectionState(p)}
	if task.VerifiedDeployment(d.Activities, d.LatestAttempt, d.CommitSHA, time.Now().UTC()) {
		result.Verified, result.Provenance = true, "verified_attempt_deployment"
	} else if running := s.runningProjectRelease(ctx, d); running != nil && running.Commit == d.CommitSHA {
		result.Verified, result.Provenance = true, "current_project_process_exact_commit"
		if running.Provenance == "registered_ssh_release" {
			result.Provenance = "registered_ssh_release_exact_commit"
		}
	}
	if !result.Verified {
		if running := s.runningProjectRelease(ctx, d); running != nil {
			if inspected := s.liveInspectionReviewContext(ctx, d); inspected != nil && inspected["verified_result_commit"] == running.Commit {
				result.Verified, result.Provenance, result.Commit = true, "verified_current_project_snapshot", running.Commit
			}
		}
	}
	return result
}

func (s *Server) requirePublicationReview(ctx context.Context, d *task.TaskDetailResponse, decision inboxReviewEvaluation) inboxReviewEvaluation {
	if decision.Decision != "pass" && decision.Decision != "revise" {
		return decision
	}
	observation := s.inboxReviewPublication(ctx, d)
	if observation == nil || !observation.Required || observation.Verified {
		return decision
	}
	if missingPublicationConnection(observation.ConnectionState) {
		// Task capabilities cannot register an Owner-scoped connection. Wait
		// for that concrete prerequisite instead of looping implementation.
		// A changed saved connection triggers native snapshot re-evaluation.
		return inboxReviewEvaluation{
			Decision: "blocked", RequestedOutcome: "unfinished", RemainingWork: "external",
			Finding:       "Hasil pekerjaan tersimpan, tetapi penerapannya di website belum bisa diverifikasi.",
			Explanation:   "Hasil pekerjaan tersimpan. Koneksi untuk memverifikasi website belum tersedia; pekerjaan ini belum bisa dinyatakan selesai.",
			ManagerAction: "wait", ManagerReason: missingPublicationConnectionGuidance,
			VisualInspection: decision.VisualInspection, CustomerReplyDraft: decision.CustomerReplyDraft,
		}
	}
	// Reapplying the guard preserves the exact saved continuation identity.
	if decision.Decision == "revise" && (strings.HasPrefix(decision.Revision, "First reconcile the registered project's existing release and saved deployment effects.") || strings.HasPrefix(decision.Revision, "Gap: the original project's required production publication is not verified")) {
		return decision
	}
	// A reviewer can misclassify absent publication proof as implementation
	// work. Preserve concrete findings, but never let that classification skip
	// reconciliation or authorize replaying an uncertain external effect.
	if decision.Decision == "revise" && decision.RemainingWork != "verification" {
		decision.Revision = "First reconcile the registered project's existing release and saved deployment effects. Missing publication evidence does not establish that deployment failed or never happened. Do not repeat deployment solely to obtain evidence; verify the existing release through the registered project channel. Preserve all original scope and acceptance requirements. Reviewer finding to evaluate against current evidence: " + decision.Finding + ". Reviewer assessment: " + decision.Revision
		decision.Finding = "Hasil perubahan belum bisa dipastikan sudah diterapkan."
		decision.Explanation = "Hasil perubahan belum bisa dipastikan sudah diterapkan. Cuan perlu memeriksa hasil yang sudah ada sebelum melanjutkan."
		return decision
	}
	result := inboxReviewEvaluation{Decision: "revise", RequestedOutcome: "unfinished", RemainingWork: "verification", VisualInspection: decision.VisualInspection, CustomerReplyDraft: decision.CustomerReplyDraft, ManagerAction: decision.ManagerAction, ManagerReason: decision.ManagerReason, Finding: "Hasil perubahan belum bisa dipastikan sudah diterapkan.", Explanation: "Hasil perubahan belum bisa dipastikan sudah diterapkan. Cuan perlu memeriksa hasil yang sudah ada sebelum melanjutkan.", Revision: "Gap: the original project's required production publication is not verified for this exact result commit. A local commit, open PR, screenshots or a passing model assessment is not deployment evidence. Inspect the registered repository, current running release and saved work before changing anything. Reconcile overlapping or already-delivered work; do not blindly merge an obsolete implementation or repeat external effects. Complete only remaining authorized publication and verification through the registered project release channel, preserving all owner restrictions. Record trusted exact-commit deployment evidence; if the requested outcome is already live in a different revision, report that fact and the precise evidence needed for independent current-outcome inspection rather than falsely claiming this branch was deployed."}
	if decision.Decision == "revise" && decision.Revision != "" {
		result.Revision += " Additional verification requested by the reviewer (untrusted assessment; reconcile existing publication first, and do not repeat deployment solely for missing evidence): " + decision.Revision
	}
	return result
}

// Revisit only coordinator-accepted managed tasks affected by the exact legacy
// footer. Requeueing a frozen review snapshot neither executes work nor grants
// another implementation attempt. Ordinary accepted and canceled work stays put.
func (s *Server) reconcileLegacyPublicationAcceptances(ctx context.Context) {
	reader, ok := s.taskSvc.Store().(task.PublicationAcceptanceAuditStore)
	if !ok {
		return
	}
	ids, err := reader.LegacyPublicationAcceptanceTaskIDs(ctx)
	if err != nil {
		return
	}
	for _, id := range ids {
		d, err := s.taskSvc.GetTaskDetail(ctx, id)
		if err != nil || d.Status != task.TaskStatusDone || d.LatestAttempt == nil {
			continue
		}
		var accepted *task.TaskActivity
		for _, event := range d.Activities {
			if event.ActivityType == task.ActivityTypeStatusChange && (accepted == nil || event.CreatedAt.After(accepted.CreatedAt) || (event.CreatedAt.Equal(accepted.CreatedAt) && event.ID > accepted.ID)) {
				accepted = event
			}
		}
		if accepted == nil || accepted.Author != "coordinator" || accepted.NewStatus != task.TaskStatusDone {
			continue
		}
		assessment, found := reviewAssessment(d, "")
		if !found || assessment.Decision != "pass" {
			continue
		}
		requests, ok := s.projectStudioRepo.(db.InboxRequestRepository)
		if !ok {
			continue
		}
		origins, err := requests.ListInboxRequests(ctx, d.SourceRoomID)
		if err != nil {
			continue
		}
		authorized := false
		snapshot := *d
		snapshot.Status = task.TaskStatusReview
		for _, origin := range origins {
			if s.reviewSnapshotRecoveryAuthorized(ctx, &snapshot, origin) {
				authorized = true
				break
			}
		}
		if !authorized {
			continue
		}
		state := "not_required"
		if observation := s.inboxReviewPublication(ctx, d); executionpolicy.NormalizeManagedInstructions(d.Instructions) != d.Instructions && observation != nil && observation.Required {
			state = "verified"
			if !observation.Verified {
				changed, err := reader.RequeueAcceptedSnapshot(ctx, d.ID, d.LatestAttempt.ID, d.UpdatedAt)
				if err != nil || !changed {
					continue
				}
				state = "review_required"
			}
		}
		_ = s.taskSvc.Store().CreateActivity(ctx, &task.TaskActivity{ID: "publication-audit:" + d.LatestAttempt.ID, TaskID: d.ID, AttemptID: &d.LatestAttempt.ID, ActivityType: task.ActivityPublicationAcceptanceAudit, Author: "coordinator", NewStatus: state, Message: "Re-evaluate the exact accepted result under the corrected legacy-footer publication policy. Original scope, failed history and correction budgets are preserved; no execution was started.", CreatedAt: time.Now().UTC()})
	}
}
