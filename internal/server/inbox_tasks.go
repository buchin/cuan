package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/buchin/cuan/internal/db"
	"github.com/buchin/cuan/internal/task"
	"github.com/go-chi/chi/v5"
)

// Explicit user action: capture work from a message on the canonical task board.
// Creation does not authorize or pretend to start execution.
func (s *Server) handleInboxMessageTask(w http.ResponseWriter, r *http.Request) {
	owner := s.accountOwner(w, r)
	if owner == "" {
		return
	}
	if s.taskSvc == nil || s.projectStudioRepo == nil {
		respondJSONError(w, 503, "Tasks are unavailable")
		return
	}
	resolved := s.resolveConversationContext(r.Context(), chi.URLParam(r, "id"), "")
	id := chi.URLParam(r, "messageID")
	if !db.IsValidUUID(id) {
		respondJSONError(w, 400, "Invalid message ID")
		return
	}
	messages, err := s.projectStudioRepo.ListMessagesByRoomID(r.Context(), resolved.RoomID)
	if err != nil {
		respondJSONError(w, 503, "Could not read the message")
		return
	}
	var source *db.ProjectMessage
	for i := range messages {
		if messages[i].ID == id {
			source = &messages[i]
			break
		}
	}
	if source == nil {
		respondJSONError(w, 404, "Message not found in this conversation")
		return
	}
	var req struct {
		ProjectID    string       `json:"project_id"`
		Title        string       `json:"title"`
		Description  *string      `json:"description,omitempty"`
		Assignee     string       `json:"assignee,omitempty"`
		DoDChecklist []db.DoDItem `json:"dod_checklist,omitempty"`
	}
	if json.NewDecoder(http.MaxBytesReader(w, r.Body, 65536)).Decode(&req) != nil {
		respondJSONError(w, 400, "Invalid task request")
		return
	}
	req.ProjectID = strings.TrimSpace(req.ProjectID)
	req.Title = strings.TrimSpace(req.Title)
	if req.ProjectID == "" || req.Title == "" || len([]rune(req.Title)) > 200 {
		respondJSONError(w, 400, "Select a project and enter a title of up to 200 characters")
		return
	}
	if s.projectSvc == nil {
		respondJSONError(w, 503, "Projects are unavailable")
		return
	}
	project, err := s.projectSvc.Get(r.Context(), req.ProjectID)
	if err != nil || project == nil {
		respondJSONError(w, 404, "Project not found")
		return
	}
	assignee := resolved.TargetAgentID
	if strings.TrimSpace(req.Assignee) != "" {
		if s.agentRepo == nil {
			respondJSONError(w, 503, "Agent list is unavailable")
			return
		}
		a, e := s.agentRepo.GetAgentByID(r.Context(), req.Assignee)
		if e != nil || a == nil || !a.Enabled {
			respondJSONError(w, 400, "Select an active registered agent")
			return
		}
		assignee = a.ID
	}
	if project.IsArchived {
		respondJSONError(w, 409, "Project is archived. Select an active project.")
		return
	}
	description := source.Content
	if _, draft := splitInboxTaskDraft(source.AuthorType, source.Content); draft != nil {
		description = draft.Description
	}
	if req.Description != nil {
		description = strings.TrimSpace(*req.Description)
		if description == "" || len([]rune(description)) > 6000 {
			respondJSONError(w, 400, "Enter a task description of up to 6000 characters")
			return
		}
	}
	instructions := fmt.Sprintf("%s\n\nSource: conversation %s, message %s.\nFollow the instructions and report results with verification evidence. Do not mark the task complete solely because the process exits with code zero.", description, resolved.RoomID, id)
	if len(req.DoDChecklist) > 0 {
		instructions += "\n\nAcceptance criteria (not yet verified):"
		for _, item := range req.DoDChecklist {
			if text := strings.TrimSpace(item.Text); text != "" {
				instructions += "\n- " + text
			}
		}
	}
	created, err := s.taskSvc.CreateTaskFromMessage(r.Context(), owner, id, task.CreateTaskRequest{SourceRoomID: resolved.RoomID, ProjectID: req.ProjectID, Title: req.Title, Instructions: instructions, AssignedAgent: assignee})
	if err != nil {
		respondJSONError(w, 503, "Could not save the task. Try again from the same message")
		return
	}
	if created.ProjectID != req.ProjectID {
		respondJSONError(w, 409, "This message is already linked to a task in another project")
		return
	}
	_, _ = s.syncRoomTitleOnTaskCreated(r.Context(), resolved.RoomID, req.Title, owner)
	s.publishInbox(resolved.RoomID)
	respondJSON(w, 200, map[string]any{"task": created, "source_message_id": id, "status": created.Status})
}
