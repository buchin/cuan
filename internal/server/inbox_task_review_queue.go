package server

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/buchin/cuan/internal/db"
	"github.com/buchin/cuan/internal/project"
	"github.com/buchin/cuan/internal/runner"
	"github.com/buchin/cuan/internal/task"
)

// Queue one coordinator review per terminal attempt, including failures with no
// final report. The synthetic input is system-authored, never user authorization.
func (s *Server) queueInboxTaskReview(ctx context.Context, detail *task.TaskDetailResponse) {
	gate := "eligibility"
	defer func() {
		if ctx.Value(inboxDeliveryDiagnosticKey{}) == true && detail != nil {
			log.Printf("inbox review checkpoint task=%s gate=%s", detail.ID, gate)
		}
	}()
	// Accepted work is final until a fresh user instruction reopens it. A late
	// review or repaired provider must never reject an already accepted result.
	if detail == nil || detail.LatestAttempt == nil || detail.Status == task.TaskStatusDone || detail.Status == "cancelled" || detail.Status == "canceled" || detail.LatestAttempt.Status == task.AttemptStatusRunning || detail.LatestAttempt.Status == task.AttemptStatusQueued || (detail.Launch != nil && (detail.Launch.State == "queued" || detail.Launch.State == "claimed")) {
		return
	}
	if detail.Status == task.TaskStatusQueued {
		if detail.Launch == nil || detail.Launch.State == "stopped" || detail.Launch.State == "cancelled" {
			reviewStatus := task.TaskStatusReview
			if _, err := s.taskSvc.UpdateTask(ctx, detail.ID, "coordinator", task.UpdateTaskRequest{Status: &reviewStatus}); err == nil {
				detail.Status = reviewStatus
			}
		} else {
			return
		}
	}

	gate = "finalization_artifacts"
	if ok, err := s.linkFinalizationArtifacts(ctx, detail); err != nil || !ok {
		return
	}
	gate = "origin_lookup"
	store, ok := s.projectStudioRepo.(db.InboxRequestRepository)
	if !ok {
		return
	}
	targetRoomID := detail.SourceRoomID
	if launch, err := s.taskSvc.LatestLaunch(ctx, detail.ID); err == nil && launch != nil && launch.RoomID != "" {
		targetRoomID = launch.RoomID
	}
	if targetRoomID == "" && detail.ProjectID != "" {
		if rooms, err := s.projectStudioRepo.ListAllRooms(ctx); err == nil {
			for _, r := range rooms {
				if r.ProjectID == detail.ProjectID {
					targetRoomID = r.ID
					break
				}
			}
		}
	}
	if targetRoomID == "" {
		if rooms, err := s.projectStudioRepo.ListAllRooms(ctx); err == nil && len(rooms) > 0 {
			targetRoomID = rooms[0].ID
		}
	}
	if targetRoomID == "" {
		return
	}
	requests, err := store.ListInboxRequests(ctx, targetRoomID)
	if err != nil {
		return
	}
	var origin *db.InboxRequest
	for i := range requests {
		if requests[i].MessageID == detail.SourceMessageID {
			origin = &requests[i]
			break
		}
	}
	if origin == nil && targetRoomID != detail.SourceRoomID && detail.SourceRoomID != "" {
		if srcReqs, err := store.ListInboxRequests(ctx, detail.SourceRoomID); err == nil {
			for i := range srcReqs {
				if srcReqs[i].MessageID == detail.SourceMessageID {
					origin = &srcReqs[i]
					break
				}
			}
		}
	}
	if origin == nil || origin.OwnerID == "" || origin.AgentID == "" {
		ownerID := ""
		if o, err := s.taskArtifactOwner(ctx, detail); err == nil && o != "" {
			ownerID = o
		}
		if ownerID == "" && len(requests) > 0 {
			ownerID = requests[0].OwnerID
		}
		if ownerID == "" {
			ownerID = "operator_user"
		}
		agentID := detail.AssignedAgent
		if agentID == "" && len(requests) > 0 {
			agentID = requests[0].AgentID
		}
		if agentID == "" {
			agentID = "scrum_master"
		}
		origin = &db.InboxRequest{
			OwnerID: ownerID,
			AgentID: agentID,
			RoomID:  targetRoomID,
		}
	}
	gate = "account_outcome_inspection"
	if (task.IsAccountOutcome(&detail.Task) || legacyAccountPublicationMismatch(detail) || s.accountOutcomeSourceVerification(ctx, detail) != nil) && !s.accountOutcomeIndependentlyVerified(ctx, detail) && s.recoverSavedEvidence(ctx, detail, *origin) {
		return
	}
	gate = "native_release_verification"
	if s.recoverNativeReleaseVerification(ctx, detail, requests) {
		return
	}
	gate = "native_release"
	if s.recoverNativeReleaseHandoff(ctx, detail, requests) {
		return
	}
	s.observeInboxPublicationConfiguration(ctx, detail, *origin)
	s.diagnoseInboxStorageFailure(ctx, detail, *origin)
	s.observeInboxGitHubConnection(ctx, detail, *origin)
	if s.recoverReviewedPhaseHandoff(ctx, detail, requests) {
		return
	}
	if s.recoverSourceVerifier(ctx, detail, *origin) {
		return
	}
	if task.IsPublicationReconciliationOutcome(&detail.Task, detail.LatestAttempt, detail.Activities) {
		gate = "source_verification_handoff"
		if s.recoverManagerVerificationHandoff(ctx, detail, *origin, requests) {
			return
		}
	}
	gate = "live_inspection"
	if s.recoverLiveResultInspection(ctx, detail, *origin) {
		return
	}
	gate = "manager_handoff"
	if s.recoverManagerVerificationHandoff(ctx, detail, *origin, requests) {
		return
	}
	gate = "saved_evidence"
	if s.recoverSavedEvidence(ctx, detail, *origin) {
		return
	}
	if s.recoverInterruptedInboxTask(ctx, detail, *origin) {
		return
	}
	recoverSnapshot := s.reviewSnapshotRecoveryAuthorized(ctx, detail, *origin)
	if recoverSnapshot && taskRevisionAttemptCount(s.reviewBudgetDetail(ctx, detail)) >= 3 {
		latest, found := reviewAssessment(detail, "")
		if found && latest.RemainingWork == "verification" {
			for _, reviewed := range requests {
				if reviewed.State != "completed" {
					continue
				}
				if saved, ok := reviewAssessment(detail, reviewed.RequestID); ok && saved == latest {
					if _, scheduled := s.startVerificationFinalization(ctx, reviewed, detail, saved); scheduled {
						return
					}
					if _, scheduled := s.startVerificationFollowup(ctx, reviewed, detail, saved); scheduled {
						return
					}
				}
			}
		}
	}
	key := "task-review:" + detail.LatestAttempt.ID
	requestID := delegationMessageID(key)
	// Reconcile terminal legacy reviews using current snapshots, with deterministic
	// keys and a bounded retry. Never replay the original provider or user execution.
	for step := 0; step < 30; step++ {
		var matched *db.InboxRequest
		for _, q := range requests {
			if q.RequestID == requestID || matchInboxRequestIDOrRetry(q.RequestID, requestID) {
				matched = &q
				if q.State == "completed" || q.State == "running" || q.State == "queued" {
					break
				}
			}
		}
		if matched == nil {
			break
		}
		if matched.State == "queued" || matched.State == "running" || matched.State == "cancelling" {
			return
		}
		if matched.State == "completed" {
			previous, err := s.projectStudioRepo.GetMessageByID(ctx, matched.MessageID)
			if err != nil || previous == nil || previous.AuthorType != "system" || previous.AuthorID != "cuan_work_review" || !strings.Contains(previous.Content, "/tasks/"+detail.ID) {
				return
			}
			if recoverSnapshot && s.verifierCorrectionContext(ctx, detail) != nil && !strings.Contains(previous.Content, inboxVerifierCorrectionPolicy) {
				key += ":verifier-correction-v6:" + inboxReviewEvidenceDigest(detail)
				requestID = delegationMessageID(key)
			} else if recoverSnapshot && legacyReviewProviderFailed(*matched) && !strings.Contains(previous.Content, inboxReviewProviderPolicy) {
				key += ":provider-assessment:" + inboxReviewEvidenceDigest(detail)
				requestID = delegationMessageID(key)
			} else if observation := s.inboxReviewPublication(ctx, detail); recoverSnapshot && observation != nil && observation.Required && !observation.Verified && !strings.Contains(previous.Content, inboxPublicationReviewPolicy) && publicationReviewNeedsRefresh(previous.Content, s.missingManagedSourceAdmission(ctx, detail)) {
				key += ":publication-assessment:" + inboxReviewEvidenceDigest(detail)
				requestID = delegationMessageID(key)
			} else if recoverSnapshot && isReconciliationOperation(detail) && !strings.Contains(previous.Content, inboxOperationalReviewPolicy) {
				key += ":operational-assessment:" + inboxReviewEvidenceDigest(detail)
				requestID = delegationMessageID(key)
			} else if recoverSnapshot && inboxReviewNeedsProvenanceRepair(detail) && !strings.Contains(previous.Content, inboxReportProvenancePolicy) {
				// Re-evaluate a misattributed historical report once. This does not
				// reset execution limits or replay the previous provider process.
				key += ":report-provenance:" + inboxReviewEvidenceDigest(detail)
				requestID = delegationMessageID(key)
			} else if recoverSnapshot && reviewAssessmentMissing(detail) && !strings.Contains(previous.Content, inboxReviewAssessmentPolicy) {
				// Recover the discarded assessment once, not execution approval
				// or additional correction budget. The review remains tool-free.
				key += ":assessment:" + inboxReviewEvidenceDigest(detail)
				requestID = delegationMessageID(key)
			} else if recoverSnapshot && s.sourceReviewClassificationMissing(ctx, detail) && !strings.Contains(previous.Content, inboxSourceReviewPolicy) {
				key += ":source-assessment:" + inboxReviewEvidenceDigest(detail)
				requestID = delegationMessageID(key)
			} else if recoverSnapshot && visualReviewNeedsRefresh(previous.Content, len(s.inboxReviewImages(ctx, detail))) {
				key += ":visual-assessment-v2:" + inboxReviewEvidenceDigest(detail)
				requestID = delegationMessageID(key)
			} else if recoverSnapshot && verificationClassificationMissing(detail) && !strings.Contains(previous.Content, inboxVerificationReviewPolicy) {
				key += ":verification-assessment:" + inboxReviewEvidenceDigest(detail)
				requestID = delegationMessageID(key)
			} else if recoverSnapshot && !strings.Contains(previous.Content, inboxOwnershipPolicy) {
				key += ":ownership-assessment:" + inboxReviewEvidenceDigest(detail)
				requestID = delegationMessageID(key)
			} else if !strings.Contains(previous.Content, "Review evidence digest: ") && recoverSnapshot {
				// Historical text-only reviews did not reliably apply their decision.
				// Evaluate the current snapshot once under the tool-free contract.
				key += ":reconcile:" + inboxReviewEvidenceDigest(detail)
				requestID = delegationMessageID(key)
			} else if recoverSnapshot && inboxReviewEvidenceChanged(detail, previous.Content) {
				key += ":evidence:" + inboxReviewEvidenceDigest(detail)
				requestID = delegationMessageID(key)
			} else if recoverSnapshot && inboxReviewExecutionSucceeded(detail) && len(task.AcceptanceGaps(&detail.Task)) == 0 && (reviewAssessmentMissing(detail) || detail.Status == task.TaskStatusReview || detail.Status == task.TaskStatusBlocked) {
				if !strings.Contains(key, ":evaluation:") && !strings.Contains(key, ":evaluation-v2:") && !strings.Contains(key, ":evaluation-v3:") {
					key += ":evaluation:" + inboxReviewEvidenceDigest(detail)
					requestID = delegationMessageID(key)
				} else if strings.Contains(key, ":evaluation:") && !strings.Contains(key, ":evaluation-v2:") && !strings.Contains(key, ":evaluation-v3:") {
					idx := strings.Index(key, ":evaluation:")
					key = key[:idx] + ":evaluation-v2:" + inboxReviewEvidenceDigest(detail)
					requestID = delegationMessageID(key)
				} else if strings.Contains(key, ":evaluation-v2:") && !strings.Contains(key, ":evaluation-v3:") {
					idx := strings.Index(key, ":evaluation-v2:")
					key = key[:idx] + ":evaluation-v3:" + inboxReviewEvidenceDigest(detail)
					requestID = delegationMessageID(key)
				} else {
					return
				}
			} else {
				return
			}
		} else if matched.State == "failed" && strings.Contains(matched.SafeError, "tool permission was denied") && recoverSnapshot {
			if strings.HasSuffix(key, ":snapshot") {
				key += ":evaluation"
			} else {
				key += ":snapshot"
			}
			requestID = delegationMessageID(key)
		} else if (matched.State == "failed" || matched.State == "interrupted") && recoverSnapshot {
			if strings.Contains(key, ":verifier-correction-v6:") || strings.Contains(key, ":verifier-correction-v5:") || strings.Contains(key, ":source-assessment:") || strings.Contains(key, ":reconcile:") || strings.Contains(key, ":report-provenance:") || strings.Contains(key, ":assessment:") || strings.Contains(key, ":verification-assessment:") || strings.Contains(key, ":visual-assessment:") || strings.Contains(key, ":operational-assessment:") || strings.Contains(key, ":publication-assessment:") || strings.Contains(key, ":ownership-assessment:") || strings.Contains(key, ":manager:") || strings.Contains(key, ":provider-assessment:") || strings.Contains(key, ":evidence:") || strings.Contains(key, ":evaluation:") || strings.Contains(key, ":evaluation-v2:") || strings.Contains(key, ":evaluation-v3:") {
				if strings.HasSuffix(key, ":retry") {
					if matched.State == "interrupted" && !strings.HasSuffix(key, ":resume") {
						key += ":resume"
						requestID = delegationMessageID(key)
					} else if strings.Contains(key, ":evaluation-v2:") && !strings.Contains(key, ":evaluation-v3:") && inboxReviewExecutionSucceeded(detail) {
						idx := strings.Index(key, ":evaluation-v2:")
						key = key[:idx] + ":evaluation-v3:" + inboxReviewEvidenceDigest(detail)
						requestID = delegationMessageID(key)
					} else {
						return
					}
				} else {
					key += ":retry"
					requestID = delegationMessageID(key)
				}
			} else {
				key += ":reconcile:" + inboxReviewEvidenceDigest(detail)
				requestID = delegationMessageID(key)
			}
		} else {
			return
		}
	}
	for _, q := range requests {
		if q.RequestID == requestID || matchInboxRequestIDOrRetry(q.RequestID, requestID) {
			return
		}
	}

	now := time.Now().UTC()
	reported := inboxReportEvidence(detail)
	content := fmt.Sprintf("Review the result for [%s](/tasks/%s).\n\nProject: %s. Current task: %s; execution: %s.\n\nOriginal request and approved scope:\n%s\n\nReported result:\n%s\n\nVerification evidence:\n%s\n\nExecution limitation:\n%s\n\nReview the supplied task snapshot against the request without calling native tools, inspecting files or running commands. A failed attempt or missing verification is not completion. Have the task session inspect its existing workspace, perform checks and correct the concrete gap within the original approval; do not attempt that execution in Inbox. Report the outcome here in the user's language and distinguish reported evidence from checks actually observed. For a fix within approved scope emit a cuan-delegate block with a concrete revision of task_id %s; do not create duplicate work or ask the user to operate tasks. Ask for approval only for new scope. This automated result is not new user authorization.", detail.Title, detail.ID, detail.ProjectID, detail.Status, detail.LatestAttempt.Status, truncateProjectText(project.TaskScopeInstructions(detail.Instructions), 9000), truncateProjectText(reported.Summary, 6000), truncateProjectText(reported.Verification, 6000), truncateProjectText(detail.LatestAttempt.ErrorMessage, 1000), detail.ID)
	content += fmt.Sprintf("\n\nReport provenance: %s; reported attempt: %s.\n\nHistorical task state (reference only, not current verification):\n%s\n%s\nPrior report from attempt %s:\n%s", reported.Provenance, reported.AttemptID, truncateProjectText(reported.HistoricalSummary, 3000), truncateProjectText(reported.HistoricalVerification, 3000), reported.HistoricalAttemptID, truncateProjectText(reported.HistoricalReport, 3000))
	content += "\n\nReview evidence digest: " + inboxReviewEvidenceDigest(detail)
	content += "\n" + inboxReportProvenancePolicy
	content += "\n" + inboxReviewAssessmentPolicy
	content += "\n" + inboxVerifierCorrectionPolicy
	content += "\n" + inboxSourceReviewPolicy
	content += "\n" + inboxVerificationReviewPolicy
	content += "\n" + inboxVisualReviewPolicy
	content += "\n" + inboxOperationalReviewPolicy
	content += "\n" + inboxPublicationReviewPolicy
	content += "\n" + inboxOwnershipPolicy
	content += "\n" + inboxReviewProviderPolicy
	content += taskRecoveryContext(detail)
	q := db.InboxRequest{OwnerID: origin.OwnerID, AgentID: origin.AgentID, RoomID: targetRoomID, RequestID: requestID, MessageID: delegationMessageID(key + ":input"), ReplyID: delegationMessageID(key + ":reply"), PayloadHash: requestID, State: "queued", CreatedAt: now, Deadline: now.Add(10 * time.Minute)}
	message := &db.ProjectMessage{ID: q.MessageID, RoomID: q.RoomID, AuthorType: "system", AuthorID: "cuan_work_review", AuthorName: "Cuan", Content: content, CreatedAt: now, UpdatedAt: now}
	if _, fresh, err := store.AcceptInboxRequest(ctx, q, message); err == nil && fresh {
		s.publishInbox(q.RoomID)
		if targetRoomID != detail.SourceRoomID && detail.SourceRoomID != "" {
			s.publishInbox(detail.SourceRoomID)
		}
		s.kickInboxRoom(q.RoomID)
	}
}

func (s *Server) sendBusinessFriendlyReviewNotification(ctx context.Context, taskID, title, summary, currentRoomID, sourceRoomID string, attemptNum int, gaps []string, projectName ...string) {
	if s.projectStudioRepo == nil {
		return
	}
	content, _ := runner.FormatBusinessFriendlyReviewMessage(title, summary, gaps, attemptNum, projectName...)
	taskLink := fmt.Sprintf("\n\n[Tinjau rincian pekerjaan](/tasks/%s)", taskID)
	fullMsg := content + taskLink

	rooms := make(map[string]bool)
	if currentRoomID != "" {
		rooms[currentRoomID] = true
	}
	if sourceRoomID != "" {
		rooms[sourceRoomID] = true
	}

	for rID := range rooms {
		// Single-bubble idempotency: do not create duplicate bubble if an identical message exists in room
		if msgs, err := s.projectStudioRepo.ListMessagesByRoomID(ctx, rID); err == nil && len(msgs) > 0 {
			alreadySent := false
			for i := len(msgs) - 1; i >= 0 && i >= len(msgs)-5; i-- {
				if msgs[i].Content == fullMsg || (strings.Contains(msgs[i].Content, taskID) && strings.Contains(msgs[i].Content, content)) {
					alreadySent = true
					break
				}
			}
			if alreadySent {
				continue
			}
		}

		msg := &db.ProjectMessage{
			ID:         db.GenerateUUID(),
			RoomID:     rID,
			AuthorType: "agent",
			AuthorID:   "cuan_assistant",
			AuthorName: "Cuan",
			Content:    fullMsg,
			CreatedAt:  time.Now().UTC(),
			UpdatedAt:  time.Now().UTC(),
		}
		if err := s.projectStudioRepo.CreateMessage(ctx, msg); err == nil {
			s.publishInbox(rID)
		}
	}
}
