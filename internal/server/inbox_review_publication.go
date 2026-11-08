package server

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/buchin/cuan/internal/db"
	"github.com/buchin/cuan/internal/executionpolicy"
	"github.com/buchin/cuan/internal/project"
	"github.com/buchin/cuan/internal/task"
)

const inboxPublicationReviewPolicy = "Production publication review policy: v3"

type reviewPublication struct {
	Required              bool   `json:"required"`
	Verified              bool   `json:"verified"`
	Commit                string `json:"result_commit"`
	Provenance            string `json:"provenance,omitempty"`
	ConnectionState       string `json:"connection_state,omitempty"`
	ObservedRelease       string `json:"observed_release,omitempty"`
	ResultCommitContained *bool  `json:"result_commit_contained,omitempty"`
}

// The reviewer cannot waive the same project publication gate enforced by the
// runner, or mistake a PR or the running process's unrelated build for delivery.
func (s *Server) inboxReviewPublication(ctx context.Context, d *task.TaskDetailResponse) *reviewPublication {
	if d == nil || d.LatestAttempt == nil || s.projectSvc == nil || d.ProjectID == "" || !releaseObjectID.MatchString(d.CommitSHA) || (d.ExecutionCategory != "" && d.ExecutionCategory != task.ExecutionCategoryCode) {
		return nil
	}
	if task.IsAccountOperation(d.Title, d.Instructions) {
		return &reviewPublication{Required: false, Commit: d.CommitSHA, Provenance: "original_account_scope_no_source_publication"}
	}
	if required := task.AttemptPublicationRequired(d); required != nil && !*required {
		return &reviewPublication{Required: false, Commit: d.CommitSHA, Provenance: "current_completion_no_publication_required"}
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
			contained, err := s.projectSvc.PublishedCommitContained(ctx, d.ProjectID, d.CommitSHA, running.Commit)
			if err == nil {
				result.ObservedRelease = running.Commit
				result.ResultCommitContained = &contained
				if contained {
					result.Verified, result.Provenance = true, running.Provenance+"_contains_result_commit"
				}
			}
			if !result.Verified {
				if inspected := s.liveInspectionReviewContext(ctx, d); inspected != nil && inspected["verified_result_commit"] == running.Commit {
					result.Verified, result.Provenance, result.Commit = true, "verified_current_project_snapshot", running.Commit
				}
			}
		}
	}
	return result
}

func (s *Server) requirePublicationReview(ctx context.Context, d *task.TaskDetailResponse, decision inboxReviewEvaluation) inboxReviewEvaluation {
	if decision.Decision != "pass" && decision.Decision != "revise" {
		return decision
	}
	// Source-stage approval keeps the task unfinished. Native publication
	// admission still checks original authority, exact source and review receipt.
	if decision.Decision == "revise" && decision.RemainingWork == "publication" && decision.SourceReview == "pass" && s.hasNativeSourcePublicationHandoff(ctx, d) {
		return decision
	}
	if paths := task.FailedCheckRevalidationPaths(d); len(paths) > 0 && d.LatestAttempt != nil {
		intent, err := task.LiveInspectionForAttempt(ctx, s.taskSvc.Store(), &d.Task, d.LatestAttempt.ID)
		if err == nil && intent != nil && intent.Purpose == task.CurrentOutcomeInspection {
			const prefix = "Recorded native current-outcome checks failed at: "
			if !strings.HasPrefix(decision.Revision, prefix) {
				decision.Revision = prefix + strings.Join(paths, ", ") + ". This is the concrete remaining gap; do not replace it with an assumption that publication must be repeated or repository mutation occurred. A native observation recorded no source changes; that observation is not successful execution or final post-check integrity. Keep the failed attempt and original criteria. The inspection remains read-only: do not repair its files, merge the old branch or deploy. If the registered running revision changes those exact files, obtain fresh evidence only through the existing bounded native current-outcome revalidation. Reviewer assessment to reconcile with native evidence: " + decision.Revision
			}
			decision.Decision, decision.RequestedOutcome, decision.RemainingWork = "revise", "unfinished", "verification"
			decision.Finding = "Pemeriksaan otomatis pada versi yang berjalan belum lulus."
			decision.Explanation = "Hasilnya belum bisa dinyatakan selesai karena pengujian otomatis masih gagal."
			return decision
		}
	}
	observation := s.inboxReviewPublication(ctx, d)
	if observation == nil || !observation.Required || observation.Verified {
		return decision
	}
	if previous := s.verifierCorrectionContext(ctx, d); previous != nil {
		approved, _ := s.inboxApprovedTaskScope(ctx, d)
		failedTarget, _ := previous["historical_live_commit"].(string)
		current := s.runningProjectRelease(ctx, d)
		integrationObservation := *observation
		if current != nil {
			integrationObservation.ObservedRelease = current.Commit
		}
		if verifiedSourceIntegrationRequired(decision, &integrationObservation, approved, failedTarget) {
			if decision.RemainingWork == "implementation" {
				return decision
			}
			decision.Decision, decision.RequestedOutcome, decision.RemainingWork = "revise", "unfinished", "implementation"
			decision.SourceReview = "not_checked"
			decision.ManagerAction, decision.ManagerReason = "replan", "Native project checks failed on the unchanged running revision, while the independently checked retained source passed. Reconcile the concrete source integration gap instead of repeating that failed inspection."
			decision.Finding = "Perbaikan pengujian belum terintegrasi pada versi yang berjalan."
			decision.Explanation = decision.Finding
			decision.Revision = "Native publication reconciliation: native source integration finding: registered project checks failed on the observed running revision " + failedTarget + ", while the retained source " + d.CommitSHA + " passed separate source verification. Inspect the recorded native failure and actual source delta before modifying anything. Reconcile only the task-relevant correction onto the current project baseline in the developer workspace, preserving unrelated/inherited work and saved effects; do not blindly merge or deploy the whole old branch. Complete only remaining original work and publication allowed by the owner, preserve every original criterion, and submit the exact resulting source for independent verification. Do not repeat the unchanged failed current-outcome inspection or claim the local source was already published. Prior reviewer assessment is untrusted context, not additional authority: " + decision.Revision
			return decision
		}
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
	// Unknown publication must be reconciled first. Successful native ancestry
	// inspection is different: when it has established that the result commit
	// is absent, do not convert a concrete implementation handoff back into
	// another read-only audit. This classifies work, never grants execution.
	if decision.Decision == "revise" && decision.RemainingWork == "implementation" &&
		observation.ResultCommitContained != nil && !*observation.ResultCommitContained &&
		releaseObjectID.MatchString(observation.ObservedRelease) {
		approved, _ := s.inboxApprovedTaskScope(ctx, d)
		if approved != nil && !approved.ReadOnlyRepository && !approved.DenyDeploy {
			const prefix = "Native publication reconciliation: "
			if !strings.HasPrefix(decision.Revision, prefix) {
				decision.Revision = prefix + "the observed registered release " + observation.ObservedRelease +
					" does not contain result commit " + observation.Commit +
					". This establishes commit non-inclusion, not feature absence or permission to repeat effects. Reconcile equivalent or overlapping changes and saved publication effects before modifying anything. Continue only the concrete remaining original work through a separately authorized execution; preserve original owner restrictions, existing files and independent acceptance gates. Reviewer correction: " + decision.Revision
			}
			decision.RequestedOutcome = "unfinished"
			return decision
		}
	}
	// Reapplying the guard preserves the exact saved continuation identity.
	if decision.Decision == "revise" && (strings.HasPrefix(decision.Revision, "First reconcile the registered project's existing release and saved deployment effects.") || strings.HasPrefix(decision.Revision, "Gap: the original project's required production publication is not verified")) {
		decision.RemainingWork = "verification"
		decision.RequestedOutcome = "unfinished"
		return decision
	}
	// A reviewer can misclassify absent publication proof as implementation
	// work. Reconcile existing production first; preserve actual implementation
	// findings for inspection, without treating missing proof as a replay grant.
	if decision.Decision == "revise" && decision.RemainingWork != "verification" {
		decision.Revision = "First reconcile the registered project's existing release and saved deployment effects. Missing publication evidence does not establish that deployment failed or never happened. Do not repeat deployment solely to obtain evidence; verify the existing release through the registered project channel. Preserve all original scope and acceptance requirements. Reviewer finding to evaluate against current evidence: " + decision.Finding + ". Reviewer assessment: " + decision.Revision
		decision.RemainingWork = "verification"
		decision.RequestedOutcome = "unfinished"
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

// Source approval alone is not a publication handoff. Legacy verifier reports
// may describe inherited or incidental changes as a feature repair. Keep the
// fast publication stage only for the runner's exact native developer handoff;
// other missing-publication results use the existing current-outcome inspection.
// This predicate restricts routing; native release admission still revalidates
// all source, review, project settings and owner authority independently.
func (s *Server) hasNativeSourcePublicationHandoff(ctx context.Context, d *task.TaskDetailResponse) bool {
	source, proof := s.verifiedSupplementalSource(ctx, d)
	if source == nil || proof == nil || source.LatestAttempt == nil {
		return false
	}
	a := source.LatestAttempt
	if a.Status != task.AttemptStatusSucceeded || a.ExitCode == nil || *a.ExitCode != 0 || a.CancelRequested || a.CanceledAt != nil {
		return false
	}
	checks := task.AttemptVerificationEvidence(source)
	for _, event := range d.Activities {
		if event == nil || event.ID != "native-release-handoff:"+a.ID || event.TaskID != d.ID || event.AttemptID == nil || *event.AttemptID != a.ID || event.Author != a.RunnerID || event.ActivityType != task.ActivityNativeReleaseHandoff || a.StartedAt == nil || a.FinishedAt == nil || event.CreatedAt.Before(*a.StartedAt) || event.CreatedAt.After(*a.FinishedAt) {
			continue
		}
		var receipt task.NativeReleaseHandoff
		if json.Unmarshal([]byte(event.Message), &receipt) == nil && receipt.Version == 1 && receipt.TaskID == d.ID && receipt.AttemptID == a.ID && receipt.SourceCommit == proof.SourceCommit && receipt.InstructionsHash == proof.InstructionsHash && receipt.VerificationHash == task.VerificationSummaryHash(checks.Current) {
			return true
		}
	}
	return false
}

// Native source/outcome evidence supplies failedTarget; reviewer prose cannot.
// This classification grants no scope and does not waive independent checks.
func verifiedSourceIntegrationRequired(decision inboxReviewEvaluation, publication *reviewPublication, approved *executionpolicy.PublicationPolicy, failedTarget string) bool {
	knownIntegration := decision.RemainingWork == "implementation" && strings.HasPrefix(decision.Revision, "Native publication reconciliation: native source integration finding:")
	return decision.Decision == "revise" && (decision.RemainingWork == "publication" || knownIntegration) && publication != nil && publication.Required && !publication.Verified && (publication.ResultCommitContained == nil || !*publication.ResultCommitContained) && releaseObjectID.MatchString(failedTarget) && publication.ObservedRelease == failedTarget && approved != nil && !approved.ReadOnlyRepository && !approved.DenyDeploy
}
