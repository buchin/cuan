package server

import (
	"context"
	"encoding/json"
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
	if job.State == "started" {
		return "This request was already handled; I have not started it again." + link
	}
	if job.State == "cancelled" || job.State == "stopped" {
		return "This launch has stopped. No new execution was started." + link
	}
	claimed, e := s.taskSvc.ClaimLaunch(ctx, now, db.GenerateUUID(), job.ID)
	if e != nil {
		return "The work is saved. I’ll check its pending launch before starting anything." + link
	}
	if claimed == nil {
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
	if e != nil || detail.ProjectID != job.ProjectID || detail.SourceRoomID != job.RoomID || detail.AssignedAgent != job.AgentID || task.LaunchInstructionsHash(detail.Instructions) != job.InstructionsHash {
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
	authorized := false
	for _, q := range requests {
		if q.RequestID == job.SourceRequestID && q.MessageID == detail.SourceMessageID && q.OwnerID == job.OwnerID && q.AgentID == job.AgentID && q.State != "cancelled" && q.State != "canceled" && q.State != "cancelling" {
			authorized = true
		}
	}
	if !authorized {
		return finish("stopped", "The original user authorization is unavailable or cancelled.")
	}
	var trigger db.InboxRequest
	for _, q := range requests {
		if q.RequestID == job.RequestID && q.OwnerID == job.OwnerID && q.AgentID == job.AgentID {
			trigger = q
			break
		}
	}
	if !s.taskContinuationAfterCancel(ctx, trigger, detail) {
		return finish("stopped", "The work was cancelled. A new user instruction is required to continue.")
	}
	response := &delegationResponse{header: make(http.Header)}
	request, _ := http.NewRequestWithContext(ctx, "POST", "http://localhost/internal-queued-launch", nil)
	s.runTaskAgent(response, request, job.TaskID, runTaskAgentPayload{RequestID: job.RequestID, AgentID: job.AgentID, Revision: job.Revision}, job.OwnerID)
	if response.code == 200 {
		return finish("started", "")
	}
	var problem struct {
		Error string `json:"error"`
		Code  string `json:"code"`
	}
	json.Unmarshal(response.body.Bytes(), &problem)
	if problem.Code == "provider_capacity" || problem.Code == "provider_unavailable" || problem.Code == "repository_preparing" {
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
	go func() { defer s.inboxWorkers.Delete(key); s.drainTaskLaunches(context.Background(), time.Now().UTC()) }()
}
func (s *Server) drainTaskLaunches(ctx context.Context, now time.Time) {
	for i := 0; i < 4 && !s.inboxStopping.Load(); i++ {
		job, e := s.taskSvc.ClaimLaunch(ctx, now, db.GenerateUUID())
		if e != nil || job == nil {
			return
		}
		run, cancel := context.WithTimeout(ctx, 30*time.Second)
		s.executeQueuedTaskLaunch(run, *job, true)
		cancel()
	}
}
func (s *Server) publishTaskLaunchOutcome(ctx context.Context, job task.TaskLaunch, state, text string) {
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
