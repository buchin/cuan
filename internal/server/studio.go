package server

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"sort"
	"strings"

	"github.com/buchin/cuan/internal/db"
	"github.com/go-chi/chi/v5"
)

// Studio accepts a goal, not a preconfigured execution. It reuses the same
// authenticated durable topic/message endpoints and coordinator queue as Inbox.
func (s *Server) handleStudioGoal(w http.ResponseWriter, r *http.Request) {
	owner := s.accountOwner(w, r)
	if owner == "" {
		return
	}
	var req struct {
		Goal      string `json:"goal"`
		RequestID string `json:"request_id"`
	}
	if json.NewDecoder(http.MaxBytesReader(w, r.Body, 32000)).Decode(&req) != nil || !db.IsValidUUID(req.RequestID) || strings.TrimSpace(req.Goal) == "" || len([]rune(req.Goal)) > 6000 {
		respondJSONError(w, 400, "Describe your goal in up to 6000 characters.")
		return
	}
	if s.agentRepo == nil {
		respondJSONError(w, 503, "Connect an agent in Settings → Agents to get started.")
		return
	}
	topicID := delegationMessageID("studio:" + strings.ToLower(req.RequestID))
	var selected *db.Agent
	if s.projectStudioRepo != nil {
		if room, e := s.projectStudioRepo.GetRoomByID(r.Context(), topicID); e == nil && room != nil {
			selected, _ = s.agentRepo.GetAgentByID(r.Context(), roomAgentID(room.Topic))
		}
	}
	if selected == nil {
		agents, err := s.agentRepo.ListAgents(r.Context(), db.AgentFilter{})
		if err != nil {
			respondJSONError(w, 503, "Could not load your coordinator. Try again.")
			return
		}
		priority := func(a db.Agent) int {
			role := strings.ToLower(a.Role + " " + a.Name)
			if strings.Contains(role, "scrum") || strings.Contains(role, "coordinator") || strings.Contains(role, "project_manager") || strings.EqualFold(strings.TrimPrefix(a.Name, "@"), "sm") {
				return 0
			}
			return 1
		}
		sort.SliceStable(agents, func(i, j int) bool {
			if priority(agents[i]) != priority(agents[j]) {
				return priority(agents[i]) < priority(agents[j])
			}
			return agents[i].ID < agents[j].ID
		})
		for _, a := range agents {
			if !a.Enabled || a.Status == "inactive" {
				continue
			}
			if _, err := s.ownedAgentProfileHome(r.Context(), owner, &a); err == nil {
				selected = &a
				break
			}
		}
	}
	if selected == nil {
		respondJSONError(w, 503, "Connect an agent in Settings → Agents to get started.")
		return
	}
	internal := func(path string, body any) *http.Request {
		raw, _ := json.Marshal(body)
		copy := r.Clone(r.Context())
		copy.URL.Path = path
		copy.Body = io.NopCloser(bytes.NewReader(raw))
		copy.ContentLength = int64(len(raw))
		return copy
	}
	topicResponse := &delegationResponse{header: make(http.Header)}
	s.handleCreateAgentTopic(topicResponse, internal("/api/v1/chat/conversations", map[string]string{"agent_id": selected.ID, "request_id": topicID}))
	if topicResponse.code != 200 {
		w.WriteHeader(topicResponse.code)
		_, _ = w.Write(topicResponse.body.Bytes())
		return
	}
	message := internal("/api/v1/chat/conversations/"+topicID+"/messages", map[string]string{"message": strings.TrimSpace(req.Goal), "request_id": req.RequestID})
	routing := chi.NewRouteContext()
	routing.URLParams.Add("id", topicID)
	message = message.WithContext(context.WithValue(message.Context(), chi.RouteCtxKey, routing))
	accepted := &delegationResponse{header: make(http.Header)}
	s.handleInternalSendConversationMessage(accepted, message)
	if accepted.code != 200 && accepted.code != 202 {
		w.WriteHeader(accepted.code)
		_, _ = w.Write(accepted.body.Bytes())
		return
	}
	respondJSON(w, 202, map[string]string{"conversation_id": topicID, "status": "accepted"})
}
