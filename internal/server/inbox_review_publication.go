package server

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/buchin/cuan/internal/db"
	"github.com/buchin/cuan/internal/executionpolicy"
	"github.com/buchin/cuan/internal/project"
	"github.com/buchin/cuan/internal/task"
)

const inboxPublicationReviewPolicy = "Production publication review policy: v7"
const previousPublicationReviewPolicy = "Production publication review policy: v6"

// v7 adds only managed-source admission repair. A policy label change alone
// must not spend another model review on unrelated unchanged v6 evidence.
func publicationReviewNeedsRefresh(previous string, missingAdmission bool) bool {
	if strings.Contains(previous, inboxPublicationReviewPolicy) {
		return false
	}
	return !strings.Contains(previous, previousPublicationReviewPolicy) || missingAdmission
}

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
	effectiveDetail := d
	if source, proof := s.verifiedSupplementalSource(ctx, d); source != nil && proof != nil {
		effectiveDetail = source
	}
	if task.IsAccountOutcome(&effectiveDetail.Task) || (task.SupportsAccountOutcomeInspection(&effectiveDetail.Task) && s.accountOutcomeIndependentlyVerified(ctx, effectiveDetail)) {
		return &reviewPublication{Required: false, Commit: effectiveDetail.CommitSHA, Provenance: "original_account_scope_no_source_publication"}
	}
	if required := task.AttemptPublicationRequired(effectiveDetail); required != nil && !*required {
		return &reviewPublication{Required: false, Commit: effectiveDetail.CommitSHA, Provenance: "current_completion_no_publication_required"}
	}
	if task.StagesDeclared(effectiveDetail.RequiredStages) && !task.RequiresPublish(effectiveDetail.RequiredStages) {
		return &reviewPublication{Required: false, Commit: effectiveDetail.CommitSHA, Provenance: "declared_stages_no_publication_required"}
	}
	_, err := s.taskPublicationPolicy(ctx, effectiveDetail)
	if err != nil {
		return &reviewPublication{Required: true, Commit: effectiveDetail.CommitSHA}
	}
	base := executionpolicy.FromInstructions(effectiveDetail.Instructions)
	if base.DenyDeploy || base.ReadOnlyRepository {
		return nil
	}
	p, err := s.projectSvc.Get(ctx, effectiveDetail.ProjectID)
	if err != nil || p == nil {
		return &reviewPublication{Required: true, Commit: effectiveDetail.CommitSHA}
	}
	if !project.SettingsFromMetadata(p.Metadata).IsAutoDeployEnabled() {
		return nil
	}
	result := &reviewPublication{Required: true, Commit: effectiveDetail.CommitSHA, ConnectionState: s.publicationConnectionState(p)}
	if task.VerifiedDeployment(effectiveDetail.Activities, effectiveDetail.LatestAttempt, effectiveDetail.CommitSHA, time.Now().UTC()) {
		result.Verified, result.Provenance = true, "verified_attempt_deployment"
	} else if running := s.runningProjectRelease(ctx, effectiveDetail); running != nil && running.Commit == effectiveDetail.CommitSHA {
		result.Verified, result.Provenance = true, "current_project_process_exact_commit"
		if running.Provenance == "registered_ssh_release" {
			result.Provenance = "registered_ssh_release_exact_commit"
		}
	}
	if !result.Verified {
		if running := s.runningProjectRelease(ctx, effectiveDetail); running != nil {
			contained, err := s.projectSvc.PublishedCommitContained(ctx, effectiveDetail.ProjectID, effectiveDetail.CommitSHA, running.Commit)
			if err == nil {
				result.ObservedRelease = running.Commit
				result.ResultCommitContained = &contained
				if contained {
					result.Verified, result.Provenance = true, running.Provenance+"_contains_result_commit"
				}
			}
			if !result.Verified {
				if appliedSHA := extractAppliedCommitSHA(effectiveDetail.CompleteNote); appliedSHA != "" {
					containedApplied, err := s.projectSvc.PublishedCommitContained(ctx, effectiveDetail.ProjectID, appliedSHA, running.Commit)
					if err == nil && containedApplied {
						result.ObservedRelease = running.Commit
						result.ResultCommitContained = &containedApplied
						result.Verified, result.Provenance = true, running.Provenance+"_contains_applied_commit"
					}
				}
			}
			if !result.Verified {
				if inspected := s.liveInspectionReviewContext(ctx, d); inspected != nil && inspected["verified_result_commit"] == running.Commit {
					result.Verified, result.Provenance, result.Commit = true, "verified_current_project_snapshot", running.Commit
				}
			}
		}
	}
	if !result.Verified && s.projectDeploymentRepo != nil {
		if deps, err := s.projectDeploymentRepo.ListByProjectID(ctx, effectiveDetail.ProjectID, 20); err == nil {
			for _, dep := range deps {
				if dep == nil || dep.Status != db.DeploymentStatusSuccess || !releaseObjectID.MatchString(dep.CommitSHA) {
					continue
				}
				if dep.CommitSHA == effectiveDetail.CommitSHA {
					contained := true
					result.ObservedRelease = dep.CommitSHA
					result.ResultCommitContained = &contained
					result.Verified, result.Provenance = true, "project_deployment_success_exact_commit"
					break
				}
				if contained, err := s.projectSvc.PublishedCommitContained(ctx, effectiveDetail.ProjectID, effectiveDetail.CommitSHA, dep.CommitSHA); err == nil && contained {
					result.ObservedRelease = dep.CommitSHA
					result.ResultCommitContained = &contained
					result.Verified, result.Provenance = true, "project_deployment_success_contains_result_commit"
					break
				}
				if p.LocalPath != "" && commitAppliesTask(ctx, p.LocalPath, dep.CommitSHA, effectiveDetail.ID) {
					contained := true
					result.ObservedRelease = dep.CommitSHA
					result.ResultCommitContained = &contained
					result.Verified, result.Provenance = true, "project_deployment_success_applied_task_commit"
					break
				}
			}
		}
	}
	return result
}

// commitAppliesTask verifies that a deployed squash-merge commit on the project
// repository was created specifically by Cuan One-Click Apply for taskID.
func commitAppliesTask(ctx context.Context, repoPath, commitSHA, taskID string) bool {
	if repoPath == "" || commitSHA == "" || taskID == "" {
		return false
	}
	out, err := exec.CommandContext(ctx, "git", "-C", repoPath, "log", "-n", "1", "--format=%s", commitSHA).Output()
	if err != nil {
		return false
	}
	subject := strings.TrimSpace(string(out))
	return strings.HasPrefix(subject, fmt.Sprintf("Apply Task %s:", taskID)) ||
		subject == fmt.Sprintf("Apply Task %s", taskID)
}

func (s *Server) requirePublicationReview(ctx context.Context, d *task.TaskDetailResponse, decision inboxReviewEvaluation) inboxReviewEvaluation {
	if decision.Decision != "pass" && decision.Decision != "revise" {
		return decision
	}
	// A failed source check precedes publication. Missing release evidence must
	// not redirect an authenticated test failure into a read-only release audit.
	failure := nativeProjectCheckFailure(d)
	if failure != "" {
		inspection, inspectionErr := task.LiveInspectionForAttempt(ctx, s.taskSvc.Store(), &d.Task, d.LatestAttempt.ID)
		verification, verificationErr := task.VerificationIntentForAttempt(ctx, s.taskSvc.Store(), &d.Task, d.LatestAttempt.ID)
		if inspectionErr != nil || verificationErr != nil || inspection != nil || verification != nil {
			failure = ""
		}
	}
	if failure != "" && decision.RequestedOutcome != "needs_input" {
		decision.Decision, decision.RequestedOutcome, decision.RemainingWork = "revise", "unfinished", "implementation"
		decision.SourceReview = "not_checked"
		decision.ManagerAction, decision.ManagerReason = "replan", "Repair the recorded native project check failure before independent verification or publication."
		decision.Finding = "Pengujian otomatis gagal; Cuan perlu memperbaikinya sebelum melanjutkan."
		decision.Explanation = decision.Finding
		decision.Revision = "Native project checks failed on the current developer attempt. Read the full current runner_verification record with the bound task tools and inspect the retained source. Reproduce the failing check and correct its root cause within the original authorized scope. Preserve unrelated work and failed history. Do not waive checks, repeat investigation or deployment, or start an independent source verifier before the registered checks pass. Submit the corrected exact source for separate verification; original publication and acceptance requirements remain. Native diagnostic: " + nativeCheckDiagnostic(failure)
		return decision
	}
	// A bound independent source rejection precedes publication just like a
	// native test failure. Missing deployment evidence must not turn an actual
	// implementation correction into another inspection of production. This
	// preserves classification only; admission still enforces owner scope.
	if d != nil && d.LatestAttempt != nil && decision.Decision == "revise" && decision.SourceReview == "revise" &&
		decision.RemainingWork == "implementation" && decision.RequestedOutcome != "needs_input" && decision.Revision != "" {
		if source, proof := s.verifiedSupplementalSource(ctx, d); source != nil && proof != nil {
			return decision
		}
		if inspection, err := task.LiveInspectionForAttempt(ctx, s.taskSvc.Store(), &d.Task, d.LatestAttempt.ID); err == nil && inspection != nil && s.inspectedSourceCorrectionAllowed(ctx, db.InboxRequest{OwnerID: inspection.OwnerID, RoomID: inspection.RoomID}, d) {
			return decision
		}
	}
	// Source-stage approval keeps the task unfinished. Native publication
	// admission still checks original authority, exact source and review receipt.
	if decision.Decision == "revise" && decision.RemainingWork == "publication" && decision.SourceReview == "pass" && s.hasNativeSourcePublicationHandoff(ctx, d) {
		observation := s.inboxReviewPublication(ctx, d)
		if observation != nil && observation.Verified && observation.Provenance != "verified_current_project_snapshot" && (observation.ResultCommitContained == nil || *observation.ResultCommitContained) {
			decision.Decision = "pass"
			decision.RequestedOutcome = "completed"
			decision.RemainingWork = "none"
			decision.Finding = "Perubahan telah terverifikasi secara independen dan telah aktif pada rilis terbaru."
			decision.Explanation = decision.Finding
			decision.Revision = ""
			decision.ManagerAction = ""
			decision.ManagerReason = ""
			return decision
		}
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
	// A completed source-bound inspection must not be scheduled as another
	// inspection of itself when the PM identifies remaining publication. Its
	// historical receipt can admit separate reconciliation under original scope;
	// it does not grant writes to the snapshot or establish feature absence.
	if decision.Decision == "revise" && (decision.RemainingWork == "publication" || decision.RemainingWork == "implementation") && decision.RequestedOutcome != "needs_input" && strings.TrimSpace(decision.Revision) != "" &&
		(decision.ManagerAction == "" || decision.ManagerAction == "continue" || decision.ManagerAction == "replan") &&
		observation != nil && observation.Required && observation.ResultCommitContained != nil && !*observation.ResultCommitContained &&
		(!observation.Verified || observation.Provenance == "verified_current_project_snapshot") {
		inspection, err := task.LiveInspectionForAttempt(ctx, s.taskSvc.Store(), &d.Task, d.LatestAttempt.ID)
		approved, _ := s.inboxApprovedTaskScope(ctx, d)
		if err == nil && inspection != nil && approved != nil && !approved.ReadOnlyRepository && !approved.DenyDeploy &&
			s.inspectedSourceCorrectionAllowed(ctx, db.InboxRequest{OwnerID: inspection.OwnerID, RoomID: inspection.RoomID}, d) {
			decision.RemainingWork, decision.RequestedOutcome, decision.SourceReview = "implementation", "unfinished", "not_checked"
			if decision.ManagerAction == "" {
				decision.ManagerAction = "continue"
			}
			if decision.ManagerReason == "" {
				decision.ManagerReason = "The completed source-bound inspection and native non-inclusion observation leave original publication unfinished; reconcile saved effects within original authorization."
			}
			prefix := "Native publication reconciliation: the retained source " + d.CommitSHA + " has completed a source-bound inspection;"
			if !strings.HasPrefix(decision.Revision, prefix) {
				decision.Revision = prefix + " the current registered release does not contain that commit. This is not proof that the feature is absent or permission to replay effects. Reconcile the retained source, equivalent changes and saved publication effects in the private developer workspace. Complete only remaining original work and publication through the native release pipeline; preserve unrelated work, original scope and independent verification. Reviewer instruction: " + decision.Revision
			}
			return decision
		}
	}
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
			if decision.RemainingWork == "implementation" && strings.HasPrefix(decision.Revision, "Native publication reconciliation: native source integration finding:") {
				return decision
			}
			decision.Decision, decision.RequestedOutcome, decision.RemainingWork = "revise", "unfinished", "implementation"
			decision.SourceReview = "not_checked"
			decision.ManagerAction, decision.ManagerReason = "replan", "Native project checks failed on the unchanged running revision, while the independently checked retained source passed. Reconcile the concrete source integration gap instead of repeating that failed inspection."
			decision.Finding = "Perbaikan pengujian belum terintegrasi pada versi yang berjalan."
			decision.Explanation = decision.Finding
			decision.Revision = "Native publication reconciliation: native source integration finding: registered project checks failed on the observed running revision " + failedTarget + ", while the retained source " + d.CommitSHA + " passed separate source verification. Inspect the recorded native failure and actual source delta before modifying anything. Reconcile only the task-relevant correction onto the current project baseline in the developer workspace, preserving unrelated/inherited work and saved effects. Integrate the minimal task-relevant delta rather than the entire retained branch. Complete only remaining original work and publication allowed by the owner, preserve every original criterion, and submit the exact resulting source for independent verification. Do not repeat the unchanged failed current-outcome inspection or claim the local source was already published."
			return decision
		}
	}
	if missingPublicationConnection(observation.ConnectionState) {
		// Task capabilities cannot register an Owner-scoped connection. Wait
		// for that concrete prerequisite instead of looping implementation.
		// A changed saved connection triggers native snapshot re-evaluation.
		customerDraft := "Perubahan sudah selesai dan lulus seluruh pengujian. Penerapan otomatis ke website produksi masih menunggu akses server atau koneksi rilis agar tim dapat menayangkannya."
		if decision.CustomerReplyDraft != "" {
			customerDraft = decision.CustomerReplyDraft
		}
		return inboxReviewEvaluation{
			Decision: "blocked", RequestedOutcome: "unfinished", RemainingWork: "external",
			Finding:       "Perubahan sudah selesai dan lulus uji; koneksi rilis ke server produksi belum tersedia.",
			Explanation:   "Perubahan kode sudah selesai dan memenuhi seluruh kriteria pengujian. Agar Cuan dapat menyiapkan jalur rilis dan menayangkannya otomatis ke produksi, cukup kirimkan akses server/hosting atau token rilis di chat ini (atau konfirmasi jika rilis dikelola di luar Cuan).",
			ManagerAction: "wait", ManagerReason: missingPublicationConnectionGuidance,
			VisualInspection: decision.VisualInspection, CustomerReplyDraft: customerDraft,
		}
	}
	if missingManagedSourceCorrection(decision, observation) && s.missingManagedSourceAdmission(ctx, d) {
		decision.Decision, decision.RequestedOutcome, decision.RemainingWork = "revise", "unfinished", "implementation"
		decision.SourceReview = "not_checked"
		decision.ManagerAction, decision.ManagerReason = "replan", "The retained independently checked source lacks native managed publication admission; reconcile it in a separate developer session within original owner scope."
		decision.Finding = "Pengujian perubahan sudah lulus, tetapi proses rilisnya belum tuntas."
		decision.Explanation = decision.Finding
		const prefix = "Native managed source admission repair: "
		if !strings.HasPrefix(decision.Revision, prefix) {
			decision.Revision = managedSourceAdmissionRevision(d.CommitSHA)
		}
		return decision
	}
	// Unknown publication must be reconciled first. Successful native ancestry
	// inspection is different: when it has established that the result commit
	// is absent, do not convert a concrete implementation handoff back into
	// another read-only audit. This classifies work, never grants execution.
	isPriorVerification := s.isPriorVerificationAttempt(ctx, d)
	if (decision.Decision == "revise" || decision.Decision == "pass") &&
		(decision.RemainingWork == "implementation" || (isPriorVerification && decision.Decision == "pass")) &&
		observation.ResultCommitContained != nil && !*observation.ResultCommitContained &&
		releaseObjectID.MatchString(observation.ObservedRelease) {
		approved, _ := s.inboxApprovedTaskScope(ctx, d)
		if approved != nil && !approved.ReadOnlyRepository && !approved.DenyDeploy {
			const prefix = "Native publication reconciliation: "
			revisionText := decision.Revision
			if revisionText == "" && decision.Explanation != "" {
				revisionText = decision.Explanation
			}
			if !strings.HasPrefix(revisionText, prefix) {
				decision.Revision = prefix + "the observed registered release " + observation.ObservedRelease +
					" does not contain result commit " + observation.Commit +
					". This establishes commit non-inclusion, not feature absence or permission to repeat effects. Reconcile equivalent or overlapping changes and saved publication effects before modifying anything. Continue only the concrete remaining original work through a separately authorized execution; preserve original owner restrictions, existing files and independent acceptance gates. Reviewer correction: " + revisionText
			} else {
				decision.Revision = revisionText
			}
			decision.Decision = "revise"
			if isPriorVerification || decision.RemainingWork == "verification" {
				decision.RemainingWork = "publication"
				decision.SourceReview = "pass"
			} else {
				decision.RemainingWork = "implementation"
			}
			decision.RequestedOutcome = "unfinished"
			if decision.Finding == "" {
				decision.Finding = publicationGapFinding(observation)
			}
			if decision.Explanation == "" {
				decision.Explanation = decision.Finding
			}
			return decision
		}
	}
	// Reapplying the guard preserves the exact saved continuation identity.
	if decision.Decision == "revise" && !isPriorVerification && (strings.HasPrefix(decision.Revision, "First reconcile the registered project's existing release and saved deployment effects.") || strings.HasPrefix(decision.Revision, "Gap: the original project's required production publication is not verified")) {
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
		decision.Finding = publicationGapFinding(observation)
		decision.Explanation = decision.Finding
		return decision
	}
	result := inboxReviewEvaluation{Decision: "revise", RequestedOutcome: "unfinished", RemainingWork: "verification", VisualInspection: decision.VisualInspection, CustomerReplyDraft: decision.CustomerReplyDraft, ManagerAction: decision.ManagerAction, ManagerReason: decision.ManagerReason, Finding: publicationGapFinding(observation), Explanation: publicationGapFinding(observation), Revision: "Gap: the original project's required production publication is not verified for this exact result commit. A local commit, open PR, screenshots or a passing model assessment is not deployment evidence. Inspect the registered repository, current running release and saved work before changing anything. Reconcile overlapping or already-delivered work; do not blindly merge an obsolete implementation or repeat external effects. Complete only remaining authorized publication and verification through the registered project release channel, preserving all owner restrictions. Record trusted exact-commit deployment evidence; if the requested outcome is already live in a different revision, report that fact and the precise evidence needed for independent current-outcome inspection rather than falsely claiming this branch was deployed."}
	if decision.Decision == "revise" && decision.Revision != "" {
		result.Revision += " Additional verification requested by the reviewer (untrusted assessment; reconcile existing publication first, and do not repeat deployment solely for missing evidence): " + decision.Revision
	}
	return result
}

func managedSourceAdmissionRevision(commit string) string {
	return "Native managed source admission repair: retain the implemented source " + commit + " and its independent verification as history. The source completion disabled publication and has no native managed-source admission. Preserve the absence of historical admission, handoff and release receipts; never fabricate them or repeat implementation. In the separate private developer workspace, reconcile only this original task's retained changes with the current registered project baseline, preserving unrelated work and uncertain existing effects. Prepare a clean checked source checkpoint for the native managed release handoff and separate source verification. Native publication and final outcome checks remain mandatory under original owner restrictions."
}

func publicationGapFinding(observed *reviewPublication) string {
	if observed != nil && observed.ResultCommitContained != nil && !*observed.ResultCommitContained && releaseObjectID.MatchString(observed.ObservedRelease) {
		return "Rilis hasil yang diperiksa belum tuntas."
	}
	return "Bukti rilis hasil pekerjaan belum lengkap."
}

func missingManagedSourceCorrection(decision inboxReviewEvaluation, observed *reviewPublication) bool {
	return decision.Decision == "revise" && decision.RequestedOutcome != "needs_input" &&
		decision.ManagerAction != "wait" && decision.ManagerAction != "needs_owner" && decision.Revision != "" &&
		(decision.RemainingWork == "publication" || decision.RemainingWork == "verification" || strings.HasPrefix(decision.Revision, "Native managed source admission repair: ")) &&
		observed != nil && observed.Required && !observed.Verified && observed.ResultCommitContained != nil && !*observed.ResultCommitContained && releaseObjectID.MatchString(observed.ObservedRelease)
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
	knownVerification := decision.RemainingWork == "verification" && (decision.ManagerAction == "continue" || decision.ManagerAction == "replan")
	knownIntegration := decision.RemainingWork == "implementation" && (strings.HasPrefix(decision.Revision, "Native publication reconciliation: native source integration finding:") || decision.ManagerAction == "continue" || decision.ManagerAction == "replan")
	return decision.Decision == "revise" && decision.RequestedOutcome != "needs_input" && (decision.RemainingWork == "publication" || knownVerification || knownIntegration) && publication != nil && publication.Required && !publication.Verified && (publication.ResultCommitContained == nil || !*publication.ResultCommitContained) && releaseObjectID.MatchString(failedTarget) && publication.ObservedRelease == failedTarget && approved != nil && !approved.ReadOnlyRepository && !approved.DenyDeploy
}

func (s *Server) isPriorVerificationAttempt(ctx context.Context, d *task.TaskDetailResponse) bool {
	if d == nil || d.LatestAttempt == nil || s.taskSvc == nil {
		return false
	}
	limits, err := s.taskSvc.AttemptPublicationPolicy(ctx, d.ID, d.LatestAttempt.ID)
	if err == nil && limits.ReadOnlyRepository {
		return true
	}
	if reader, ok := s.taskSvc.Store().(task.AttemptLaunchStore); ok {
		if launch, err := reader.LaunchForAttempt(ctx, d.LatestAttempt.ID); err == nil && launch != nil {
			if strings.HasPrefix(launch.Revision, "Scope: read-only repository") ||
				strings.Contains(launch.Revision, "First reconcile the registered project's existing release") ||
				strings.Contains(launch.Revision, "Gap: the original project's required production publication") {
				return true
			}
		}
	}
	return false
}

func extractAppliedCommitSHA(note string) string {
	idx := strings.Index(note, "Perubahan telah berhasil diterapkan (Commit ")
	if idx < 0 {
		return ""
	}
	sub := note[idx+len("Perubahan telah berhasil diterapkan (Commit "):]
	end := strings.Index(sub, ")")
	if end < 0 {
		return ""
	}
	sha := strings.TrimSpace(sub[:end])
	if releaseObjectID.MatchString(sha) {
		return sha
	}
	return ""
}
