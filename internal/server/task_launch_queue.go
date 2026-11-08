package server

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/buchin/cuan/internal/agent"
	"github.com/buchin/cuan/internal/db"
	"github.com/buchin/cuan/internal/task"
)

// Only tool-free review application installs this observer. Ordinary chat turns
// must not be replayed merely because a later launch write failed.
type inboxReviewLaunchErrorKey struct{}

func recordInboxReviewLaunchError(ctx context.Context, err error) {
	if observed, ok := ctx.Value(inboxReviewLaunchErrorKey{}).(*error); ok {
		*observed = err
	}
}
func retryableReviewLaunchError(err error) bool {
	return err != nil && !errors.Is(err, task.ErrRequestConflict) && !errors.Is(err, task.ErrInvalidTaskInput) && !errors.Is(err, task.ErrTaskNotFound)
}

func (s *Server) startAuthorizedLaunch(ctx context.Context, q db.InboxRequest, t *task.TaskDetailResponse, launchID, sourceRequestID, revision string, deadline ...time.Time) string {
	// System-generated phase turns are execution context, never fresh consent.
	// All continuation paths (capacity, interruption, inspection and review) must
	// retain the same original owner approval rather than a generated request ID.
	if t != nil && q.MessageID == t.SourceMessageID && s.projectStudioRepo != nil {
		source, err := s.projectStudioRepo.GetMessageByID(ctx, q.MessageID)
		if err != nil {
			recordInboxReviewLaunchError(ctx, err)
			return "The approved continuation could not be checked. No execution was started."
		}
		if source != nil && source.AuthorType == "system" && source.AuthorID == "cuan_auto_delegation" {
			approval, err := s.phaseApproval(ctx, t, q.OwnerID, q.RoomID)
			if err != nil {
				return "The approved phase could not be validated. No execution was started."
			}
			sourceRequestID = approval.Request.RequestID
		}
	}
	now := time.Now().UTC()
	expires := now.Add(24 * time.Hour)
	if len(deadline) > 0 && deadline[0].Before(expires) {
		expires = deadline[0]
	}
	// Automatic corrections inherit the owner's approved continuation. Feedback
	// from a review cannot erase a prior no-push/no-deploy or read-only boundary.
	ownerTurn := false
	if launchID == q.RequestID && s.projectStudioRepo != nil {
		if message, err := s.projectStudioRepo.GetMessageByID(ctx, q.MessageID); err == nil && message != nil && message.AuthorType == "user" {
			ownerTurn = true
		}
	}
	if !ownerTurn {
		basis, err := s.ownerContinuationLaunch(ctx, t)
		if err != nil {
			recordInboxReviewLaunchError(ctx, err)
			return "The approved continuation could not be checked. No execution was started."
		}
		if basis != nil && basis.OwnerID == q.OwnerID {
			revision = "Binding approved continuation:\n" + basis.Revision + "\n\nAutomatic correction feedback:\n" + revision
		}
	}

	launchAgent := q.AgentID
	if t != nil && t.AssignedAgent != "" {
		launchAgent = t.AssignedAgent
	}
	candidate := task.TaskLaunch{TaskID: t.ID, ProjectID: t.ProjectID, RequestID: launchID, OwnerID: q.OwnerID, RoomID: q.RoomID, SourceRequestID: sourceRequestID, AgentID: launchAgent, Revision: revision, InstructionsHash: task.LaunchInstructionsHash(t.Instructions), CreatedAt: now, AvailableAt: now, ExpiresAt: expires}
	nativeRelease, nativeErr := task.NativeReleaseIntentForLaunch(&t.Task, &candidate)
	if nativeErr != nil {
		return "The native release scope could not be validated. No execution was started."
	}
	if nativeRelease != nil {
		if err := task.ValidateNativeReleaseSource(ctx, s.taskSvc.Store(), &t.Task, nativeRelease); err != nil {
			return "The native release source could not be verified. No execution was started."
		}
		trigger, authorized, err := s.authorizeSavedTaskLaunch(ctx, candidate, t)
		if err != nil || !authorized || !s.taskContinuationAfterCancel(ctx, trigger, t) {
			return "The original release authorization could not be verified. No execution was started."
		}
	}
	job, e := s.taskSvc.SaveLaunch(ctx, candidate)
	link := "\n\n[View work](/tasks/" + t.ID + ")"
	if e != nil {
		recordInboxReviewLaunchError(ctx, e)
		return "The work is saved, but its launch could not be recorded. No new execution was started." + link
	}
	handled := job.State == "started" || (job.State == "stopped" && t != nil && t.Status == task.TaskStatusDone)
	if job.State == "stopped" && t != nil && t.LatestAttempt != nil && t.LatestAttempt.Status == task.AttemptStatusSucceeded {
		if launches, ok := s.taskSvc.Store().(task.AttemptLaunchStore); ok {
			if completed, err := launches.LaunchForAttempt(ctx, t.LatestAttempt.ID); err == nil && completed != nil && completed.ID == job.ID {
				handled = true
			}
		}
	}
	if handled {
		return "This request was already handled; I have not started it again." + link
	}
	if job.State == "cancelled" || job.State == "stopped" {
		return "This launch has stopped. No new execution was started." + link
	}

	// Ensure task status is queued prior to creating/claiming launch to prevent race condition with terminal state checks
	status := task.TaskStatusQueued
	_, _ = s.taskSvc.UpdateTask(ctx, t.ID, "runner", task.UpdateTaskRequest{Status: &status})

	lane := t.ExecutionCategory
	if lane == "" {
		lane = task.ExecutionCategoryCode
	}
	if s.isProjectLaneActive(ctx, t.ProjectID, lane, t.ID) {
		status := task.TaskStatusQueued
		_, _ = s.taskSvc.UpdateTask(ctx, t.ID, "runner", task.UpdateTaskRequest{Status: &status})
		return "The work is queued waiting for the active project task to complete. I’ll start it automatically and report progress here." + link
	}
	claimed, e := s.taskSvc.ClaimLaunch(ctx, now, db.GenerateUUID(), job.ID)
	if e != nil {
		return "The work is saved. I’ll check its pending launch before starting anything." + link
	}
	if claimed == nil {
		status := task.TaskStatusQueued
		_, _ = s.taskSvc.UpdateTask(ctx, t.ID, "runner", task.UpdateTaskRequest{Status: &status})
		return "The work is waiting to start. I’ll report progress here." + link
	}
	return s.executeQueuedTaskLaunch(ctx, *claimed, false)
}

const supersededRecoveryLaunchReason = "A newer execution replaced this recovery request. Existing work was preserved."

const staleInspectionLaunchReason = task.InspectionPredecessorPreparationError

func (s *Server) finishQueuedTaskLaunch(ctx context.Context, job task.TaskLaunch, background bool, state, reason string) string {
	if state == "stopped" && reason == task.ActiveAttemptLaunchError {
		// Another request may win reservation after this dispatcher read the
		// task. Keep this unstarted intent queued; do not alter the active task.
		state = "queued"
	}
	link := "\n\n[View work](/tasks/" + job.TaskID + ")"
	save, cancel := context.WithTimeout(context.WithoutCancel(ctx), 3*time.Second)
	defer cancel()
	// The response describes admission, not the execution's current state.
	// Another dispatcher may have reserved this exact request while the
	// response was in flight. Its durable attempt wins over a late error.
	if state != "started" {
		detail, err := s.taskSvc.GetTaskDetail(save, job.TaskID)
		if err != nil {
			return "The saved execution is being reconciled. No replacement was started." + link
		}
		owned, err := s.launchOwnsLatestAttempt(save, &job, detail)
		if err != nil {
			return "The saved execution is being reconciled. No replacement was started." + link
		}
		if owned {
			a := detail.LatestAttempt
			actual := "stopped"
			if !a.CancelRequested && a.CanceledAt == nil && (a.Status == task.AttemptStatusRunning || a.Status == task.AttemptStatusQueued) {
				actual = "started"
			}
			if err := s.taskSvc.FinishLaunch(save, job.ID, job.LeaseToken, actual, time.Now().UTC(), ""); err != nil {
				return "The saved execution is being reconciled. No replacement was started." + link
			}
			return "This request already has a recorded execution. No replacement was started." + link
		}
	}
	if e := s.taskSvc.FinishLaunch(save, job.ID, job.LeaseToken, state, time.Now().UTC().Add(30*time.Second), reason); e != nil {
		return "The launch status is being reconciled. I have not started a replacement." + link
	}
	job.SafeError = reason
	text := "The work is saved, but could not start. " + reason + link
	if state == "queued" {
		text = "The work is waiting for an available AI session. I’ll start it automatically and report progress here." + link
		if strings.Contains(job.Revision, "Cuan native-release intent") {
			text = "The release is saved and will continue when its execution requirements are available." + link
		}
		if strings.Contains(reason, "project") || strings.Contains(reason, "task") || strings.Contains(reason, "active") {
			text = "The work is queued waiting for the active project task to complete. I’ll start it automatically and report progress here." + link
		}
	}
	if state == "queued" && strings.Contains(reason, "Project files could not be downloaded.") {
		text = "I’m preparing the project files and will continue automatically. Your request and approval are saved." + link
	}
	if state == "started" {
		text = strings.TrimSpace(link)
	}
	if background && state != "queued" {
		s.publishTaskLaunchOutcome(save, job, state, text)
	}
	return text
}

func (s *Server) executeQueuedTaskLaunch(ctx context.Context, job task.TaskLaunch, background bool) string {
	link := "\n\n[View work](/tasks/" + job.TaskID + ")"
	finish := func(state, reason string) string {
		return s.finishQueuedTaskLaunch(ctx, job, background, state, reason)
	}
	current, e := s.taskSvc.GetLaunch(ctx, job.ID)
	if e != nil || current.State != "claimed" || current.LeaseToken != job.LeaseToken || current.LeaseUntil == nil || !current.LeaseUntil.After(time.Now()) {
		return "This launch is no longer active. No replacement was started." + link
	}
	detail, e := s.taskSvc.GetTaskDetail(ctx, job.TaskID)
	if e != nil || detail.ProjectID != job.ProjectID || task.LaunchInstructionsHash(detail.Instructions) != job.InstructionsHash {
		return finish("stopped", "The saved task scope has changed; check the current instructions before continuing.")
	}
	verification, verificationErr := task.VerificationIntentForLaunch(&detail.Task, &job)
	if verificationErr != nil {
		return finish("stopped", "The verification scope could not be validated.")
	}
	liveInspection, inspectionErr := task.LiveInspectionForLaunch(&detail.Task, &job)
	if inspectionErr != nil {
		return finish("stopped", task.InspectionScopePreparationError)
	}
	nativeRelease, nativeErr := task.NativeReleaseIntentForLaunch(&detail.Task, &job)
	if nativeErr != nil {
		return finish("stopped", "The native release scope could not be validated.")
	}
	recoverySource := automaticRecoverySource(detail, job.RequestID)
	trigger, authorized, authErr := s.authorizeSavedTaskLaunch(ctx, job, detail)
	if authErr != nil {
		return finish("queued", "Conversation storage is temporarily unavailable.")
	}
	if !authorized {
		return finish("stopped", "The original user authorization is unavailable or cancelled.")
	}
	if !s.taskContinuationAfterCancel(ctx, trigger, detail) {
		return finish("stopped", "The work was cancelled. A new user instruction is required to continue.")
	}
	// A launch response can be lost after its exact attempt was reserved. Once
	// that attempt exists, it is no longer supposed to match its predecessor.
	// Reconcile ownership rather than declaring the active inspection stale or
	// replaying it; completion/acceptance still belongs to the ordinary pipeline.
	if (liveInspection != nil || recoverySource != nil || nativeRelease != nil) && detail.LatestAttempt != nil {
		recorded, err := s.launchOwnsLatestAttempt(ctx, &job, detail)
		if err != nil {
			return finish("queued", "Saved execution identity is temporarily unavailable.")
		}
		if recorded {
			state := "stopped"
			a := detail.LatestAttempt
			if !a.CancelRequested && a.CanceledAt == nil && (a.Status == task.AttemptStatusRunning || a.Status == task.AttemptStatusQueued) {
				state = "started"
			}
			save, cancel := context.WithTimeout(context.WithoutCancel(ctx), 3*time.Second)
			defer cancel()
			if err := s.taskSvc.FinishLaunch(save, job.ID, job.LeaseToken, state, time.Now().UTC(), ""); err != nil {
				return "The saved execution is being reconciled. No replacement was started." + link
			}
			return "This request already has a recorded execution. No replacement was started." + link
		}
	}
	if recoverySource != nil && (detail.LatestAttempt == nil || detail.LatestAttempt.ID != recoverySource.ID || recoverySource.CancelRequested || recoverySource.CanceledAt != nil || (recoverySource.Status != task.AttemptStatusFailed && recoverySource.Status != task.AttemptStatusInterrupted)) {
		return finish("stopped", supersededRecoveryLaunchReason)
	}
	recoverySourceID := ""
	if recoverySource != nil {
		recoverySourceID = recoverySource.ID
	}
	if liveInspection != nil && (detail.LatestAttempt == nil || detail.LatestAttempt.ID != liveInspection.PredecessorAttemptID() || !s.liveInspectionMatchesRelease(ctx, detail, liveInspection)) {
		return finish("stopped", staleInspectionLaunchReason)
	}
	if nativeRelease != nil {
		if detail.LatestAttempt == nil || detail.LatestAttempt.ID != nativeRelease.SourceReviewAttemptID || task.ValidateNativeReleaseSource(ctx, s.taskSvc.Store(), &detail.Task, nativeRelease) != nil {
			return finish("stopped", "The native release source changed or is unavailable.")
		}
		_, err := s.taskSvc.CreateAttempt(ctx, detail.ID, job.OwnerID, task.CreateAttemptRequest{RunnerID: "cuan-runner", AgentName: job.AgentID, RequestID: job.RequestID, ExpectedPredecessorID: nativeRelease.SourceReviewAttemptID})
		if err != nil {
			if errors.Is(err, task.ErrRequestConflict) || errors.Is(err, task.ErrInvalidTaskInput) {
				return finish("stopped", "The native release source is no longer current.")
			}
			if errors.Is(err, task.ErrTaskActive) {
				return finish("queued", "Native release is waiting for the active project task.")
			}
			return finish("queued", "Native release admission is temporarily unavailable.")
		}
		return finish("started", "")
	}
	response := &delegationResponse{header: make(http.Header)}
	request, _ := http.NewRequestWithContext(ctx, "POST", "http://localhost/internal-queued-launch", nil)
	targetAgent := job.AgentID
	if detail.AssignedAgent != "" {
		targetAgent = detail.AssignedAgent
	}
	// Resolve the public assistant before reserving an attempt. Historical
	// persona aliases must agree with the canonical assignment checked by storage.
	if s.agentRepo != nil {
		if resolved, err := s.runtimeAgent(ctx, targetAgent); err == nil && resolved != nil && resolved.ID == db.AgentIDCuan && targetAgent != resolved.ID {
			targetAgent = resolved.ID
			if _, err := s.taskSvc.UpdateTask(ctx, detail.ID, "coordinator", task.UpdateTaskRequest{AssignedAgent: &targetAgent}); err != nil {
				return finish("queued", "Assistant routing could not be saved; no execution was started.")
			}
		}
	}
	runnerID := "cuan-runner"
	if verification == nil && liveInspection == nil && (targetAgent == "agent_lifecycle" || targetAgent == "agent_queued" || job.AgentID == "agent_lifecycle" || job.AgentID == "agent_queued") {
		runnerID = "local_terminal"
	}
	if verification != nil && verification.FinalizeAttemptID != "" {
		if _, err := task.ValidateVerificationSource(ctx, s.taskSvc.Store(), &detail.Task, detail.LatestAttempt.ID); err != nil {
			return finish("stopped", "The saved verification source could not be validated.")
		}
		if _, err := s.taskSvc.CreateAttempt(ctx, detail.ID, job.OwnerID, task.CreateAttemptRequest{RunnerID: "cuan-runner", AgentName: targetAgent, RequestID: job.RequestID}); err != nil {
			return finish("queued", "Verification finalization is waiting for its project lane.")
		}
		return finish("started", "")
	}
	s.runTaskAgent(response, request, job.TaskID, runTaskAgentPayload{RequestID: job.RequestID, AgentID: targetAgent, Revision: job.Revision, RunnerID: runnerID, RecoverySourceAttemptID: recoverySourceID}, job.OwnerID)
	if response.code == 200 {
		return finish("started", "")
	}
	var problem struct {
		Error string `json:"error"`
		Code  string `json:"code"`
	}
	json.Unmarshal(response.body.Bytes(), &problem)
	if targetAgent != db.AgentIDCuan && (problem.Code == "provider_capacity" || problem.Code == "provider_unavailable" || problem.Code == "provider_vision_unsupported") {
		needsVision := s.taskRequiresVision(ctx, job.OwnerID, detail, "", job.Revision)
		if altAgent := s.findHealthyTaskAgentFailover(ctx, job.OwnerID, targetAgent, detail, needsVision); altAgent != nil {
			log.Printf("[TaskLaunch] Auto-failover task %s from %s to alternative healthy agent %s (reason: %s)", job.TaskID, targetAgent, altAgent.ID, problem.Code)
			newAgentID := altAgent.ID
			if s.taskSvc != nil {
				_, _ = s.taskSvc.UpdateTask(ctx, job.TaskID, "coordinator", task.UpdateTaskRequest{AssignedAgent: &newAgentID})
			}
			altResp := &delegationResponse{header: make(http.Header)}
			altReq, _ := http.NewRequestWithContext(ctx, "POST", "http://localhost/internal-queued-launch", nil)
			s.runTaskAgent(altResp, altReq, job.TaskID, runTaskAgentPayload{RequestID: job.RequestID, AgentID: newAgentID, Revision: job.Revision, RunnerID: runnerID, RecoverySourceAttemptID: recoverySourceID}, job.OwnerID)
			if altResp.code == 200 {
				return finish("started", "")
			}
			json.Unmarshal(altResp.body.Bytes(), &problem)
		}
	}
	if problem.Code == "execution_storage" || problem.Code == "provider_capacity" || problem.Code == "provider_unavailable" || problem.Code == "provider_vision_unsupported" || problem.Code == "repository_preparing" || problem.Code == "project_concurrency_busy" {
		if job.Checks >= 10 && problem.Code != "execution_storage" && problem.Code != "provider_capacity" && problem.Code != "provider_unavailable" {
			status := task.TaskStatusBlocked
			_, _ = s.taskSvc.UpdateTask(ctx, job.TaskID, "runner", task.UpdateTaskRequest{Status: &status})
			return finish("stopped", "The task reached maximum queue retry attempts waiting for project concurrency or provider availability. Check active work or run again.")
		}
		status := task.TaskStatusQueued
		_, _ = s.taskSvc.UpdateTask(ctx, job.TaskID, "runner", task.UpdateTaskRequest{Status: &status})
		return finish("queued", problem.Error)
	}
	if problem.Error == "" {
		problem.Error = "Execution could not be confirmed. Check the saved attempt before continuing."
	}
	return finish("stopped", problem.Error)
}
func (s *Server) kickTaskLaunches() {
	const key = "__durable_task_launches"
	if s.inboxStopping.Load() || s.taskSvc == nil {
		return
	}
	if _, loaded := s.inboxWorkers.LoadOrStore(key, true); loaded {
		return
	}
	go func() {
		defer s.inboxWorkers.Delete(key)
		s.drainTaskLaunches(context.Background(), time.Now().UTC())
	}()
}
func (s *Server) drainTaskLaunches(ctx context.Context, now time.Time) {
	for i := 0; i < 4 && !s.inboxStopping.Load(); i++ {
		job, e := s.taskSvc.ClaimLaunch(ctx, now, db.GenerateUUID())
		if e != nil {
			log.Printf("[TaskLaunch] ClaimLaunch error: %v", e)
			return
		}
		if job == nil {
			return
		}
		log.Printf("[TaskLaunch] Claimed launch %s for task %s, starting execution...", job.ID, job.TaskID)
		run, cancel := context.WithTimeout(ctx, 30*time.Second)
		res := s.executeQueuedTaskLaunch(run, *job, true)
		log.Printf("[TaskLaunch] executeQueuedTaskLaunch for %s returned: %s", job.ID, res)
		cancel()
	}
}
func (s *Server) publishTaskLaunchOutcome(ctx context.Context, job task.TaskLaunch, state, text string) {
	// Customer / visitor rooms must NEVER receive internal intermediate status dumps or execution errors.
	if s.isCustomerRoom(ctx, job.RoomID) {
		return
	}

	// A stale parallel inspection is not a blocker for a different, recorded
	// execution. Keep its diagnostic in launch history without alarming the user
	// that approved work could not start while that work is actually running.
	if state == "stopped" && (job.SafeError == staleInspectionLaunchReason || job.SafeError == supersededRecoveryLaunchReason) && s.taskSvc != nil {
		if d, err := s.taskSvc.GetTaskDetail(ctx, job.TaskID); err == nil && d.LatestAttempt != nil {
			a := d.LatestAttempt
			if !a.CancelRequested && a.CanceledAt == nil && (a.Status == task.AttemptStatusRunning || a.Status == task.AttemptStatusQueued) {
				if reader, ok := s.taskSvc.Store().(task.AttemptLaunchStore); ok {
					bound, err := reader.LaunchForAttempt(ctx, a.ID)
					if err == nil && bound != nil && bound.ID != job.ID && bound.TaskID == job.TaskID && bound.OwnerID == job.OwnerID && bound.ProjectID == job.ProjectID && bound.InstructionsHash == job.InstructionsHash {
						return
					}
				}
			}
		}
	}

	// The foreground reply can itself deliver a stopped launch. Wait until that
	// durable reply is reconciled, then avoid a second identical agent bubble.
	if state == "stopped" && job.SourceRequestID != "" {
		if store, ok := s.projectStudioRepo.(db.InboxRequestRepository); ok {
			if requests, e := store.ListInboxRequests(ctx, job.RoomID); e == nil {
				for _, request := range requests {
					if request.RequestID != job.SourceRequestID {
						continue
					}
					if request.State == "queued" || request.State == "running" {
						return
					}
					if request.State == "completed" && request.ReplyID != "" && job.SafeError != "" {
						if messages, e := s.projectStudioRepo.ListMessagesByRoomID(ctx, job.RoomID); e == nil {
							for _, message := range messages {
								if message.ID == request.ReplyID && strings.Contains(message.Content, "/tasks/"+job.TaskID) && strings.Contains(message.Content, job.SafeError) {
									return
								}
							}
						}
					}
				}
			}
		}
	}

	now := time.Now().UTC()
	message := &db.ProjectMessage{ID: delegationMessageID("launch-outcome:" + job.ID + ":" + state), RoomID: job.RoomID, AuthorType: "agent", AuthorID: job.AgentID, AuthorName: "Cuan", Content: text, CreatedAt: now, UpdatedAt: now}
	if s.projectStudioRepo.CreateMessage(ctx, message) == nil {
		s.publishInbox(job.RoomID)
	}
}

func (s *Server) findHealthyTaskAgentFailover(ctx context.Context, ownerID, currentAgentID string, detail *task.TaskDetailResponse, needsVision bool) *db.Agent {
	if s.agentRepo == nil {
		return nil
	}
	agents, err := s.agentRepo.ListAgents(ctx, db.AgentFilter{})
	if err != nil || len(agents) == 0 {
		return nil
	}
	cleanCurrent := strings.TrimPrefix(strings.ToLower(currentAgentID), "@")
	for _, a := range agents {
		if !a.Enabled || (a.Status != db.AgentStatusActive && a.Status != "") {
			continue
		}
		cleanA := strings.TrimPrefix(strings.ToLower(a.ID), "@")
		if cleanA == cleanCurrent || strings.TrimPrefix(cleanA, "agent_") == cleanCurrent || cleanCurrent == "agent_"+cleanA {
			continue
		}
		harness := db.NormalizeHarness(a.Harness, a.Provider)
		if harness != "agy" && harness != "antigravity" && harness != "commandcode" && harness != "claude" {
			continue
		}
		if needsVision && !agentEntitySupportsVision(&a) {
			continue
		}
		if harness == "agy" || harness == "antigravity" {
			if pa, _ := agent.ProviderAccountForConnection(ownerID, a.ID, a.ConnectionID); pa != nil {
				if pa.State == "quota_limited" && (pa.QuotaRetryAt == nil || time.Now().Before(*pa.QuotaRetryAt)) {
					continue
				}
				if pa.State == "login_required" || pa.State == "unavailable" {
					continue
				}
			}
		}
		candidate := a
		return &candidate
	}
	return nil
}

func agentEntitySupportsVision(a *db.Agent) bool {
	if a == nil {
		return false
	}
	harness := db.NormalizeHarness(a.Harness, a.Provider)
	provider := a.Provider
	if provider == "" {
		provider = harness
	}
	modelID := a.ModelID
	if idx := strings.Index(modelID, "#"); idx != -1 {
		modelID = modelID[idx+1:]
	}
	if agent.IsModelVisionCapable(provider, modelID) || agent.IsModelVisionCapable(harness, modelID) {
		return true
	}
	fallbackID := a.FallbackModelID
	if idx := strings.Index(fallbackID, "#"); idx != -1 {
		fallbackID = fallbackID[idx+1:]
	}
	if fallbackID != "" {
		if agent.IsModelVisionCapable(provider, fallbackID) || agent.IsModelVisionCapable(harness, fallbackID) {
			return true
		}
	}
	if (harness == "agy" || harness == "antigravity") && (modelID == "" || modelID == "default" || strings.HasPrefix(modelID, "gemini")) {
		return true
	}
	return false
}

// A launch describes admission; its bound attempt owns execution and outcome.
func (s *Server) launchOwnsLatestAttempt(ctx context.Context, job *task.TaskLaunch, detail *task.TaskDetailResponse) (bool, error) {
	if job == nil || detail == nil || detail.LatestAttempt == nil {
		return false, nil
	}
	reader, ok := s.taskSvc.Store().(task.AttemptLaunchStore)
	if !ok {
		return false, errors.New("saved execution identity unavailable")
	}
	bound, err := reader.LaunchForAttempt(ctx, detail.LatestAttempt.ID)
	return bound != nil && bound.ID == job.ID, err
}
