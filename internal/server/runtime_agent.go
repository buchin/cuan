package server

import (
	"context"
	"strings"

	"github.com/buchin/cuan/internal/db"
)

// Historical persona IDs are routing aliases for the editable public assistant.
// Keep their saved task/request IDs intact, but never pin old Inbox work to a
// retired persona's provider connection when the public assistant exists.
func (s *Server) runtimeAgent(ctx context.Context, id string) (*db.Agent, error) {
	clean := strings.ToLower(strings.TrimPrefix(strings.TrimSpace(id), "@"))
	switch clean {
	case "sm", "agent_sm", "dev", "agent_dev", "support", "agent_support", "cuan", "agent_cuan":
		if current, err := s.agentRepo.GetAgentByID(ctx, db.AgentIDCuan); err == nil && current != nil {
			return current, nil
		}
	}
	return s.agentRepo.GetAgentByID(ctx, id)
}

func (s *Server) sameRuntimeAgent(ctx context.Context, left, right string) bool {
	if left == "" || right == "" {
		return false
	}
	if left == right {
		return true
	}
	if s.agentRepo == nil {
		return false
	}
	a, err := s.runtimeAgent(ctx, left)
	if err != nil || a == nil {
		return false
	}
	b, err := s.runtimeAgent(ctx, right)
	return err == nil && b != nil && a.ID == b.ID
}
