package server

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/buchin/cuan/internal/db"
	"github.com/buchin/cuan/internal/scheduler"
	"github.com/buchin/cuan/internal/task"
)

// executeScheduledTask is the callback invoked whenever a ScheduledTask routine is triggered.
// It creates a real Cuan task bound to the specified project, and initiates execution via an agent.
func (s *Server) executeScheduledTask(ctx context.Context, schedTask *scheduler.ScheduledTask) error {
	if schedTask == nil {
		return fmt.Errorf("scheduled task cannot be nil")
	}
	if s.taskSvc == nil {
		return fmt.Errorf("task service is not available")
	}

	projectID := strings.TrimSpace(schedTask.ProjectID)
	if projectID == "" && s.projectSvc != nil {
		if p, err := s.projectSvc.EnsureDefault(ctx); err == nil && p != nil {
			projectID = p.ID
		}
	}

	author := "scheduler"
	ownerID := "owner"
	if s.authSvc != nil {
		if id, err := s.authSvc.OwnerID(ctx); err == nil && id != "" {
			author = id
			ownerID = id
		}
	}

	targetAgent := db.AgentIDCuan
	req := task.CreateTaskRequest{
		ProjectID:     projectID,
		Title:         schedTask.Title,
		Instructions:  schedTask.Prompt,
		Priority:      task.PriorityNormal,
		Status:        task.TaskStatusQueued,
		AssignedAgent: targetAgent,
	}
	if projectID != "" {
		req.ProjectIDs = []string{projectID}
	}

	created, err := s.taskSvc.CreateTask(ctx, author, req)
	if err != nil {
		log.Printf("[Scheduler] Failed to create task for routine %q (%s): %v", schedTask.Title, schedTask.ID, err)
		return fmt.Errorf("create task: %w", err)
	}

	if s.taskProjectRepo != nil && len(req.ProjectIDs) > 0 {
		_ = s.taskProjectRepo.SetTaskProjects(ctx, created.ID, req.ProjectIDs)
	}

	log.Printf("[Scheduler] Routine %q (%s) created task %s in project %s", schedTask.Title, schedTask.ID, created.ID, projectID)

	// Attempt immediate agent launch if terminal manager is configured
	if s.terminalManager != nil {
		response := &delegationResponse{header: make(http.Header)}
		request, _ := http.NewRequestWithContext(ctx, "POST", "http://localhost/internal-queued-launch", nil)
		s.runTaskAgent(response, request, created.ID, runTaskAgentPayload{
			RequestID: db.GenerateUUID(),
			AgentID:   targetAgent,
			RunnerID:  "cuan-runner",
			Prompt:    schedTask.Prompt,
		}, ownerID)

		if response.code != 200 && response.code != 0 && response.code != http.StatusAccepted {
			log.Printf("[Scheduler] Routine %s immediate agent dispatch note (status %d): %s; task remains queued for runner supervisor", schedTask.ID, response.code, response.body.String())
		}
	}

	s.kickTaskLaunches()
	return nil
}
