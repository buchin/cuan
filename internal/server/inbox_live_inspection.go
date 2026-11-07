package server

import (
	"context"
	"encoding/json"
	"errors"
	"regexp"
	"strings"
	"time"

	"github.com/buchin/cuan/internal/db"
	"github.com/buchin/cuan/internal/task"
)

var liveInspectionCheckProhibition = regexp.MustCompile(`(?i)\b(?:(?:do not|don't|never|jangan|dilarang|tanpa|without)[^\n.!?]{0,40}\b(?:run|running|execute|jalankan|menjalankan|eksekusi)\b[^\n.!?]{0,40}\b(?:tests?|tes|pengujian)\b|(?:no|tanpa)\s+(?:tests?|tes|pengujian)\b)`)

func (s *Server) recoverLiveResultInspection(ctx context.Context, d *task.TaskDetailResponse, origin db.InboxRequest) bool {
	if d == nil || (d.Status != task.TaskStatusBlocked && d.Status != task.TaskStatusReview) || d.LatestAttempt == nil || origin.MessageID != d.SourceMessageID || origin.RoomID != d.SourceRoomID || !strings.HasPrefix(d.Instructions, "Cuan managed delegation v1.\n") || !s.sameRuntimeAgent(ctx, origin.AgentID, d.AssignedAgent) || !s.reviewSnapshotRecoveryAuthorized(ctx, d, origin) {
		return false
	}
	// A verifier is read-only and cannot become the implementation source.
	// Keep the current review/approval, resolving only its native-bound original
	// source for the existing single current-outcome inspection allowance.
	source := d.LatestAttempt
	if original, proof := s.verifiedSupplementalSource(ctx, d); original != nil {
		if proof.OwnerID != origin.OwnerID || proof.RoomID != origin.RoomID || proof.SourceRequestID != origin.RequestID {
			return false
		}
		source = original.LatestAttempt
	}
	if s.recoverUnstartedLiveInspection(ctx, d, origin, source.ID) {
		return true
	}
	if d.Status == task.TaskStatusReview {
		assessment, found := reviewAssessment(d, "")
		if !found || assessment.Decision != "revise" || assessment.RemainingWork != "verification" || (!task.IsUnpublishedReviewedOutcome(&d.Task, source, d.Activities) && !task.IsPublicationReconciliationOutcome(&d.Task, source, d.Activities) && len(task.FailedCheckRevalidationPaths(d)) == 0) {
			return false
		}
	}
	if restored, err := s.taskSvc.ReconcileInspectionIdentity(ctx, d.ID); err == nil && restored {
		s.publishInbox(d.SourceRoomID)
		return true
	}
	if s.recoverInspectionVerifier(ctx, d, origin) {
		return true
	}
	if s.recoverChangedOutcomeInspection(ctx, d, origin) {
		return true
	}
	a := source
	unpublished := task.IsUnpublishedLegacyOutcome(&d.Task, a, d.Activities)
	if !unpublished && (task.IsUnpublishedReviewedOutcome(&d.Task, a, d.Activities) || task.IsPublicationReconciliationOutcome(&d.Task, a, d.Activities)) {
		// The task layer validates the saved record shape; the coordinator also
		// requires its exact current evidence digest before granting inspection.
		assessment, found := reviewAssessment(d, "")
		unpublished = found && assessment.Decision == "revise" && assessment.RemainingWork == "verification"
	}
	currentOutcome := unpublished || (d.CommitSHA == "" && task.IsCurrentOutcomeFailure(a) && (d.ExecutionCategory == "" || d.ExecutionCategory == task.ExecutionCategoryCode))
	if (a.Status != task.AttemptStatusFailed && !unpublished) || a.CancelRequested || a.CanceledAt != nil || (a.FinishedAt == nil && !task.IsUnrecordedLocalFailure(a)) {
		return false
	}
	if !currentOutcome && (!releaseObjectID.MatchString(d.CommitSHA) || len(task.AcceptanceGaps(&d.Task)) > 0 || a.ExitCode == nil || *a.ExitCode != 0 || inboxReportEvidence(d).Provenance != "current_attempt_report") {
		return false
	}
	previous, err := task.LiveInspectionForAttempt(ctx, s.taskSvc.Store(), &d.Task, a.ID)
	if err != nil {
		return false
	}
	sourceAttempt := a.ID
	if currentOutcome {
		if previous != nil {
			return false
		}
	} else if previous != nil {
		if previous.Purpose != "" {
			return false
		}
		if previous.PreviousAttemptID != "" || a.ErrorMessage != "Live result inspection project checks did not pass" {
			return false
		}
		sourceAttempt = previous.SourceAttemptID
	} else if a.ErrorMessage != "Project verification failed" || d.ResultKind != "implementation" {
		return false
	}
	limits, err := s.taskSvc.AttemptPublicationPolicy(ctx, d.ID, sourceAttempt)
	if err != nil || limits.ReadOnlyRepository {
		return false
	}
	if !s.liveInspectionReadScopeAllowed(ctx, d, sourceAttempt) {
		return false
	}
	observed := s.runningProjectRelease(ctx, d)
	if observed == nil || observed.Commit == d.CommitSHA {
		return false
	}
	if previous != nil && observed.Commit == previous.TargetCommit {
		return false
	}
	_, authorized, err := s.authorizeSavedTaskLaunch(ctx, task.TaskLaunch{OwnerID: origin.OwnerID, RoomID: origin.RoomID, SourceRequestID: origin.RequestID, CreatedAt: time.Now().UTC()}, d)
	if err != nil || !authorized || !s.taskContinuationAfterCancel(ctx, origin, d) {
		return false
	}
	proof := &task.LiveInspectionIntent{SourceAttemptID: sourceAttempt, SourceCommit: d.CommitSHA, TargetCommit: observed.Commit, Repository: observed.Repository, OwnerID: origin.OwnerID, RoomID: origin.RoomID, SourceRequestID: origin.RequestID, InstructionsHash: task.LaunchInstructionsHash(d.Instructions), ObservedAt: observed.ObservedAt}
	if currentOutcome {
		proof.Purpose = task.CurrentOutcomeInspection
	}
	if previous != nil {
		proof.PreviousAttemptID, proof.PreviousCommit = a.ID, previous.TargetCommit
	}
	if task.ValidateLiveInspectionSource(ctx, s.taskSvc.Store(), &d.Task, proof) != nil {
		return false
	}
	return s.startLiveResultInspection(ctx, d, origin, proof, currentOutcome)
}

// A current review may grant a new correction before the correction budget is
// exhausted. Reconcile an existing live outcome first even on that fast path;
// otherwise an ordinary worker loses the native exact-revision inspection intent.
func (s *Server) startReviewedLiveInspection(ctx context.Context, q db.InboxRequest, d *task.TaskDetailResponse, assessment inboxReviewEvaluation) bool {
	if d == nil || assessment.Decision != "revise" || assessment.RemainingWork != "verification" || q.RoomID != d.SourceRoomID {
		return false
	}
	saved, found := reviewAssessment(d, q.RequestID)
	if !found || saved != assessment {
		return false
	}
	repository, ok := s.projectStudioRepo.(db.InboxRequestRepository)
	if !ok {
		return false
	}
	requests, err := repository.ListInboxRequests(ctx, d.SourceRoomID)
	if err != nil {
		return false
	}
	for _, origin := range requests {
		if origin.OwnerID == q.OwnerID && origin.MessageID == d.SourceMessageID {
			return s.recoverLiveResultInspection(ctx, d, origin)
		}
	}
	return false
}

func (s *Server) startLiveResultInspection(ctx context.Context, d *task.TaskDetailResponse, origin db.InboxRequest, proof *task.LiveInspectionIntent, currentOutcome bool) bool {
	id := proof.RequestID(d.ID)
	saved, err := s.taskSvc.GetLaunch(ctx, task.LaunchID(d.ID, id))
	if err != nil && !errors.Is(err, task.ErrTaskNotFound) {
		return false
	}
	if saved != nil {
		if proof.PreparationLaunchID == "" && task.IsInspectionPreparationFailure(saved) {
			replacement := *proof
			replacement.PreparationLaunchID = saved.ID
			if task.ValidateLiveInspectionSource(ctx, s.taskSvc.Store(), &d.Task, &replacement) == nil {
				return s.startLiveResultInspection(ctx, d, origin, &replacement, currentOutcome)
			}
		}
		return saved.State == "queued" || saved.State == "claimed" || saved.State == "started"
	}
	raw, _ := json.Marshal(proof)
	revision := "Scope: read-only repository. Do not push. Do not deploy.\n" + task.LiveInspectionMarker + string(raw) + "\nInspect the already running result in the separate exact-commit snapshot. Do not resume or modify the old implementation workspace. Prior reports and failed tests are history, not proof of this result. Inspect the actual current implementation against every original requirement and run targeted read-only checks. The runner will independently run the fixed project verifier after your final report and verify unchanged state. Do not modify code, configuration, tests, documentation, git index or refs, install dependencies into the snapshot, publish, or repeat external effects. Use session artifacts for any required screenshots/reports. Report actual findings, original acceptance evidence and precise remaining gaps; never repair or waive missing implementation/publication. Reserve time for runner checks within this verification deadline."
	if currentOutcome {
		revision = "Scope: read-only repository. Do not push. Do not deploy.\n" + task.LiveInspectionMarker + string(raw) + "\nInspect the exact currently running revision to determine whether the original requested outcome is already satisfied. Do not credit the historical attempt with this result or its publication, resume its process, alter its workspace, repeat implementation, or waive any original requirement. Reproduce the original issue and test the current behavior; inspect actual UI screenshots at original required viewports for UI work. Check every original acceptance criterion and report precise gaps. You may prepare locked dependencies and ignored verification outputs inside this private snapshot, without changing tracked files, lockfiles, index or git refs; never use shared build output symlinks. Deliver screenshots/reports using session artifacts. The runner independently runs fixed project checks and verifies unchanged source after your final report. This is current-outcome evidence, not ancestry of unknown old work or a deployment performed by this task. Do not fix code or perform external effects in this inspection."
	}
	if currentOutcome && proof.SourceCommit != "" {
		revision += "\nThe historical task retained a committed branch, but publication of that branch was not established. Preserve its recorded execution state, including any failure. Its result commit is retained as history, not an ancestor requirement or proof of deployment. Inspect only the already-running target revision and original outcome. Do not merge the old PR, claim that its branch was deployed, or change any historical execution/commit metadata."
	}
	revision += "\nFor visual evidence, prefer the repository's existing real-component/browser verification scripts before writing a new harness or guessing exports from minified release bundles. Never substitute manually authored HTML/DOM, recreated component markup or direct library-only rendering for screenshots of the actual application components. Report any failed capture honestly and leave the corresponding criteria incomplete. Reserve time within the same execution deadline for the runner's independent project checks."
	revision += "\nVerification ownership: inspect behavior and original acceptance criteria; the native runner owns the registered project check suite after your final report. Do not duplicate that suite or repeatedly poll unrelated processes. Run an additional targeted check only when it supplies acceptance evidence the registered suite does not cover, and explain that gap. Report runner-owned checks as pending, never as passed before their receipt exists. If your own check stalls, stop only the exact process group you started and preserve its failure evidence. Never use killall, broad pkill patterns, or stop shared services/processes to free resources. Report a resource blocker instead. These instructions do not provide operating-system isolation."
	if proof.ReviewRequestID != "" {
		revision += "\nThis is the single permitted read-only revalidation after source paths named in the rejected inspection changed. Recheck every original criterion on this exact revision; the earlier assessment and changed files do not prove completion. Do not resume implementation, waive requirements or claim the old failed attempt succeeded. A signal exit alone is not successful execution: preserve explicit confirmed-report completion receipts and failure/cancellation guards."
	}
	if task.IsUnrecordedLocalFailure(d.LatestAttempt) {
		revision += "\nThe historical local attempt is recorded as Failed, but its exit code and finish receipt were not saved. Its cause and stop time are unknown. Never infer timeout, successful execution or implementation authorship, fill in missing termination fields, or resume that process. Only inspect this new exact-revision snapshot against the original requirements."
	}
	var expires []time.Time
	if proof.PreparationLaunchID != "" {
		parent, err := s.taskSvc.GetLaunch(ctx, proof.PreparationLaunchID)
		if err != nil || parent == nil || task.ValidateLiveInspectionSource(ctx, s.taskSvc.Store(), &d.Task, proof) != nil {
			return false
		}
		expires = append(expires, parent.ExpiresAt)
	}
	s.startAuthorizedLaunch(ctx, origin, d, id, origin.RequestID, revision, expires...)
	launch, err := s.taskSvc.GetLaunch(ctx, task.LaunchID(d.ID, id))
	if err != nil || launch == nil || (launch.State != "queued" && launch.State != "claimed" && launch.State != "started") {
		return false
	}
	now := time.Now().UTC()
	_ = s.projectStudioRepo.CreateMessage(ctx, &db.ProjectMessage{ID: delegationMessageID(id + ":notice"), RoomID: d.SourceRoomID, AuthorType: "system", AuthorName: "Cuan", Content: "Cuan memeriksa hasil pekerjaan pada versi yang sudah berjalan.\n\n[View work](/tasks/" + d.ID + ")", CreatedAt: now, UpdatedAt: now})
	s.publishInbox(d.SourceRoomID)
	return true
}

func (s *Server) liveInspectionReviewContext(ctx context.Context, d *task.TaskDetailResponse) map[string]any {
	if d == nil || d.LatestAttempt == nil || d.LatestAttempt.Status != task.AttemptStatusSucceeded {
		return nil
	}
	proof, err := task.ValidateLiveInspection(ctx, s.taskSvc.Store(), &d.Task, d.LatestAttempt.ID)
	if err != nil || proof == nil || !s.liveInspectionMatchesRelease(ctx, d, proof) {
		return nil
	}
	_, unchanged, err := s.inboxReviewRepositoryScope(ctx, d)
	checks := task.AttemptVerificationEvidence(d)
	if err != nil || !unchanged || checks.Author != d.LatestAttempt.RunnerID || !task.ProjectChecksPassed(d) {
		return nil
	}
	for _, event := range d.Activities {
		if event == nil || event.TaskID != d.ID || event.AttemptID == nil || *event.AttemptID != d.LatestAttempt.ID || event.ActivityType != task.ActivityLiveInspection || event.Author != d.LatestAttempt.RunnerID || event.ID != "live-result:"+d.LatestAttempt.ID {
			continue
		}
		var receipt task.LiveInspectionReceipt
		if json.Unmarshal([]byte(event.Message), &receipt) == nil && receipt.Purpose == proof.Purpose && receipt.SourceAttemptID == proof.SourceAttemptID && receipt.SourceCommit == proof.SourceCommit && receipt.TargetCommit == proof.TargetCommit && receipt.Repository == proof.Repository && receipt.ProjectChecksHash == task.VerificationSummaryHash(checks.Current) {
			result := map[string]any{"source_attempt_id": proof.SourceAttemptID, "verified_result_commit": proof.TargetCommit, "repository": proof.Repository, "repository_unchanged": true, "current_project_checks": checks.Current, "observed_running_at": proof.ObservedAt}
			if proof.Purpose == task.CurrentOutcomeInspection {
				result["current_outcome_inspected"] = true
				source, readErr := s.taskSvc.Store().GetAttempt(ctx, proof.SourceAttemptID)
				if readErr != nil {
					return nil
				}
				if proof.SourceCommit != "" {
					result["source_unpublished_result"] = true
					result["unpublished_source_commit"] = proof.SourceCommit
					result["source_execution_state"] = source.Status
				}
				if task.IsUnrecordedLocalFailure(source) {
					result["source_termination_metadata_missing"] = true
				}
			} else {
				result["source_commit"] = proof.SourceCommit
				result["source_ancestry_verified"] = true
			}
			return result
		}
	}
	return nil
}

func (s *Server) liveInspectionMatchesRelease(ctx context.Context, d *task.TaskDetailResponse, proof *task.LiveInspectionIntent) bool {
	observed := s.runningProjectRelease(ctx, d)
	return observed != nil && proof != nil && observed.Commit == proof.TargetCommit && observed.Repository == proof.Repository
}

func (s *Server) prepareLiveInspection(ctx context.Context, d *task.TaskDetailResponse, requestID string) (string, *task.LiveInspectionIntent, error) {
	if d == nil || requestID == "" {
		return "", nil, nil
	}
	launch, err := s.taskSvc.GetLaunch(ctx, task.LaunchID(d.ID, requestID))
	if errors.Is(err, task.ErrTaskNotFound) {
		return "", nil, nil
	}
	if err != nil {
		return "", nil, err
	}
	proof, err := task.LiveInspectionForLaunch(&d.Task, launch)
	if err != nil || proof == nil {
		return "", proof, err
	}
	expected := proof.PredecessorAttemptID()
	if d.LatestAttempt == nil || d.LatestAttempt.ID != expected || !s.liveInspectionMatchesRelease(ctx, d, proof) {
		return "", nil, errors.New("the saved result inspection no longer matches this task and running release")
	}
	if err := task.ValidateLiveInspectionSource(ctx, s.taskSvc.Store(), &d.Task, proof); err != nil {
		return "", nil, err
	}
	p, err := s.projectSvc.Get(ctx, d.ProjectID)
	if err != nil || p == nil || p.IsArchived {
		return "", nil, errors.New("the inspection project is unavailable or archived")
	}
	path, err := s.projectSvc.VerificationSnapshot(ctx, d.ProjectID, d.ID, proof.SnapshotSourceCommit(), proof.TargetCommit)
	if err == nil && proof.ReviewRequestID != "" {
		changed, checkErr := s.projectSvc.VerificationPathsChanged(ctx, d.ProjectID, d.ID, proof.PreviousCommit, proof.TargetCommit, proof.ReviewPaths)
		if checkErr != nil || !changed {
			return "", nil, errors.New("reviewed source paths did not change in this result")
		}
	}
	return path, proof, err
}
