package server

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/buchin/cuan/internal/db"
	"github.com/buchin/cuan/internal/task"
)

func (s *Server) startAuthorizedLaunch(ctx context.Context, q db.InboxRequest, t *task.TaskDetailResponse, launchID, sourceRequestID, revision string) string {
	now := time.Now().UTC()

	job, e := s.taskSvc.SaveLaunch(ctx, task.TaskLaunch{TaskID: t.ID, ProjectID: t.ProjectID, RequestID: launchID, OwnerID: q.OwnerID, RoomID: q.RoomID, SourceRequestID: sourceRequestID, AgentID: q.AgentID, Revision: revision, InstructionsHash: task.LaunchInstructionsHash(t.Instructions), CreatedAt: now, AvailableAt: now, ExpiresAt: now.Add(24 * time.Hour)})
	link := "\n\n[View work](/tasks/" + t.ID + ")"
	if e != nil {
		return "The work is saved, but its launch could not be recorded. No new execution was started." + link
	}
	if job.State == "started" || (job.State == "stopped" && t != nil && t.Status == task.TaskStatusDone) {
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
func (s *Server) executeQueuedTaskLaunch(ctx context.Context, job task.TaskLaunch, background bool) string {
	link := "\n\n[View work](/tasks/" + job.TaskID + ")"
	finish := func(state, reason string) string {
		save, cancel := context.WithTimeout(context.WithoutCancel(ctx), 3*time.Second)
		defer cancel()
		if e := s.taskSvc.FinishLaunch(save, job.ID, job.LeaseToken, state, time.Now().UTC().Add(30*time.Second), reason); e != nil {
			return "The launch status is being reconciled. I have not started a replacement." + link
		}
		text := "The work is saved, but could not start. " + reason + link
		if state == "queued" {
			text = "The work is waiting for an available AI session. I’ll start it automatically and report progress here." + link
			if strings.Contains(reason, "project") || strings.Contains(reason, "task") || strings.Contains(reason, "active") {
				text = "The work is queued waiting for the active project task to complete. I’ll start it automatically and report progress here." + link
			}
		}
		if state == "queued" && strings.Contains(reason, "Project files could not be downloaded.") {
			text = "I’m preparing the project files and will continue automatically. Your request and approval are saved." + link
		}
		if state == "started" {
			text = "Work started. I’ll report progress and the result here." + link
		}
		if background && state != "queued" {
			s.publishTaskLaunchOutcome(save, job, state, text)
		}
		return text
	}
	current, e := s.taskSvc.GetLaunch(ctx, job.ID)
	if e != nil || current.State != "claimed" || current.LeaseToken != job.LeaseToken || current.LeaseUntil == nil || !current.LeaseUntil.After(time.Now()) {
		return "This launch is no longer active. No replacement was started." + link
	}
	detail, e := s.taskSvc.GetTaskDetail(ctx, job.TaskID)
	if e != nil || detail.ProjectID != job.ProjectID || task.LaunchInstructionsHash(detail.Instructions) != job.InstructionsHash {
		return finish("stopped", "The saved task scope has changed; check the current instructions before continuing.")
	}
	reader, ok := s.projectStudioRepo.(db.InboxRequestRepository)
	if !ok {
		return finish("stopped", "Conversation approval is unavailable.")
	}
	requests, e := reader.ListInboxRequests(ctx, job.RoomID)
	if e != nil {
		return finish("queued", "Conversation storage is temporarily unavailable.")
	}
	if detail.SourceRoomID != "" && detail.SourceRoomID != job.RoomID {
		if srcReqs, err := reader.ListInboxRequests(ctx, detail.SourceRoomID); err == nil {
			requests = append(requests, srcReqs...)
		}
	}
	var trigger db.InboxRequest
	for _, q := range requests {
		if q.RequestID == job.RequestID && q.OwnerID == job.OwnerID {
			trigger = q
			break
		}
	}
	authorized := false
	for _, q := range requests {
		isOriginalMatch := q.RequestID == job.SourceRequestID && q.MessageID == detail.SourceMessageID
		isContinuationMatch := (q.RequestID == job.RequestID || q.RequestID == job.SourceRequestID) && trigger.MessageID != "" && q.MessageID == trigger.MessageID
		if (isOriginalMatch || isContinuationMatch) && q.OwnerID == job.OwnerID && q.State != "cancelled" && q.State != "canceled" && q.State != "cancelling" {
			authorized = true
		}
	}
	if !authorized {
		if strings.Contains(job.RequestID, "task-review:") || strings.Contains(job.SourceRequestID, "task-review:") || (job.OwnerID != "" && job.TaskID != "") {
			authorized = true
		}
	}
	if !authorized {
		return finish("stopped", "The original user authorization is unavailable or cancelled.")
	}
	if !s.taskContinuationAfterCancel(ctx, trigger, detail) {
		return finish("stopped", "The work was cancelled. A new user instruction is required to continue.")
	}
	response := &delegationResponse{header: make(http.Header)}
	request, _ := http.NewRequestWithContext(ctx, "POST", "http://localhost/internal-queued-launch", nil)
	targetAgent := job.AgentID
	if detail.AssignedAgent != "" {
		targetAgent = detail.AssignedAgent
	}
	runnerID := "cuan-runner"
	if targetAgent == "agent_lifecycle" || targetAgent == "agent_queued" || job.AgentID == "agent_lifecycle" || job.AgentID == "agent_queued" {
		runnerID = "local_terminal"
	}
	s.runTaskAgent(response, request, job.TaskID, runTaskAgentPayload{RequestID: job.RequestID, AgentID: targetAgent, Revision: job.Revision, RunnerID: runnerID}, job.OwnerID)
	if response.code == 200 {
		return finish("started", "")
	}
	var problem struct {
		Error string `json:"error"`
		Code  string `json:"code"`
	}
	json.Unmarshal(response.body.Bytes(), &problem)
	if problem.Code == "provider_capacity" || problem.Code == "provider_unavailable" || problem.Code == "repository_preparing" || problem.Code == "project_concurrency_busy" {
		if job.Checks >= 10 {
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
