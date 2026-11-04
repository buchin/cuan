package server

import (
	"context"
	"encoding/json"
	"errors"
	"regexp"
	"strings"
	"time"

	"github.com/buchin/cuan/internal/db"
	"github.com/buchin/cuan/internal/project"
	"github.com/buchin/cuan/internal/task"
)

var liveInspectionCheckProhibition = regexp.MustCompile(`(?i)\b(?:(?:do not|don't|never|jangan|dilarang|tanpa|without)[^\n.!?]{0,40}\b(?:run|running|execute|jalankan|menjalankan|eksekusi)\b[^\n.!?]{0,40}\b(?:tests?|tes|pengujian)\b|(?:no|tanpa)\s+(?:tests?|tes|pengujian)\b)`)

func (s *Server) recoverLiveResultInspection(ctx context.Context, d *task.TaskDetailResponse, origin db.InboxRequest) bool {
	if d == nil || d.Status != task.TaskStatusBlocked || d.LatestAttempt == nil || d.ResultKind != "implementation" || !releaseObjectID.MatchString(d.CommitSHA) || len(task.AcceptanceGaps(&d.Task)) > 0 || origin.MessageID != d.SourceMessageID || origin.RoomID != d.SourceRoomID || !strings.HasPrefix(d.Instructions, "Cuan managed delegation v1.\n") || !s.sameRuntimeAgent(ctx, origin.AgentID, d.AssignedAgent) || !s.reviewSnapshotRecoveryAuthorized(ctx, d, origin) {
		return false
	}
	a := d.LatestAttempt
	if a.Status != task.AttemptStatusFailed || a.ExitCode == nil || *a.ExitCode != 0 || a.ErrorMessage != "Project verification failed" || a.CancelRequested || a.CanceledAt != nil || a.FinishedAt == nil || inboxReportEvidence(d).Provenance != "current_attempt_report" {
		return false
	}
	limits, err := s.taskPublicationPolicy(ctx, d)
	if err != nil || limits.ReadOnlyRepository {
		return false
	}
	checkScope := project.TaskScopeInstructions(d.Instructions)
	if launches, ok := s.taskSvc.Store().(task.AttemptLaunchStore); ok {
		launch, err := launches.LaunchForAttempt(ctx, a.ID)
		if err != nil {
			return false
		}
		if launch != nil {
			checkScope += "\n" + launch.Revision
		}
	}
	if liveInspectionCheckProhibition.MatchString(checkScope) {
		return false
	}
	observed := s.runningProjectRelease(ctx, d)
	if observed == nil || observed.Commit == d.CommitSHA {
		return false
	}
	_, authorized, err := s.authorizeSavedTaskLaunch(ctx, task.TaskLaunch{OwnerID: origin.OwnerID, RoomID: origin.RoomID, SourceRequestID: origin.RequestID, CreatedAt: time.Now().UTC()}, d)
	if err != nil || !authorized || !s.taskContinuationAfterCancel(ctx, origin, d) {
		return false
	}
	proof := &task.LiveInspectionIntent{SourceAttemptID: a.ID, SourceCommit: d.CommitSHA, TargetCommit: observed.Commit, Repository: observed.Repository, OwnerID: origin.OwnerID, RoomID: origin.RoomID, SourceRequestID: origin.RequestID, InstructionsHash: task.LaunchInstructionsHash(d.Instructions), ObservedAt: observed.ObservedAt}
	if task.ValidateLiveInspectionSource(ctx, s.taskSvc.Store(), &d.Task, proof) != nil {
		return false
	}
	id := task.LiveInspectionRequestID(d.ID, d.CommitSHA, proof.InstructionsHash)
	previous, err := s.taskSvc.GetLaunch(ctx, task.LaunchID(d.ID, id))
	if err != nil && !errors.Is(err, task.ErrTaskNotFound) {
		return false
	}
	if previous != nil {
		return previous.State == "queued" || previous.State == "claimed" || previous.State == "started"
	}
	raw, _ := json.Marshal(proof)
	revision := "Scope: read-only repository. Do not push. Do not deploy.\n" + task.LiveInspectionMarker + string(raw) + "\nInspect the already running result in the separate exact-commit snapshot. Do not resume or modify the old implementation workspace. Prior reports and failed tests are history, not proof of this result. Inspect the actual current implementation against every original requirement and run targeted read-only checks. The runner will independently run the fixed project verifier after your final report and verify unchanged state. Do not modify code, configuration, tests, documentation, git index or refs, install dependencies into the snapshot, publish, or repeat external effects. Use session artifacts for any required screenshots/reports. Report actual findings, original acceptance evidence and precise remaining gaps; never repair or waive missing implementation/publication. Reserve time for runner checks within this verification deadline."
	s.startAuthorizedLaunch(ctx, origin, d, id, origin.RequestID, revision)
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
		if json.Unmarshal([]byte(event.Message), &receipt) == nil && receipt.SourceAttemptID == proof.SourceAttemptID && receipt.SourceCommit == proof.SourceCommit && receipt.TargetCommit == proof.TargetCommit && receipt.Repository == proof.Repository && receipt.ProjectChecksHash == task.VerificationSummaryHash(checks.Current) {
			return map[string]any{"source_attempt_id": proof.SourceAttemptID, "source_commit": proof.SourceCommit, "verified_result_commit": proof.TargetCommit, "repository": proof.Repository, "source_ancestry_verified": true, "repository_unchanged": true, "current_project_checks": checks.Current, "observed_running_at": proof.ObservedAt}
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
	if d.LatestAttempt == nil || d.LatestAttempt.ID != proof.SourceAttemptID || !s.liveInspectionMatchesRelease(ctx, d, proof) {
		return "", nil, errors.New("the saved result inspection no longer matches this task and running release")
	}
	if err := task.ValidateLiveInspectionSource(ctx, s.taskSvc.Store(), &d.Task, proof); err != nil {
		return "", nil, err
	}
	p, err := s.projectSvc.Get(ctx, d.ProjectID)
	if err != nil || p == nil || p.IsArchived {
		return "", nil, errors.New("the inspection project is unavailable or archived")
	}
	path, err := s.projectSvc.VerificationSnapshot(ctx, d.ProjectID, d.ID, proof.SourceCommit, proof.TargetCommit)
	return path, proof, err
}
