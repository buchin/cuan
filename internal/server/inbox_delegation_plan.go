package server

import (
	"context"
	"errors"

	"github.com/buchin/cuan/internal/db"
)

// A normal conversational turn may delegate an exact saved proposal through the
// existing authenticated delegation path. Recover that proposal's native scope
// without rewriting the user's message or interpreting arbitrary text as consent.
// Only the immediately preceding, provenance-checked proposal is eligible.
func (s *Server) delegationPlan(ctx context.Context, authorization *db.ProjectMessage, owner, agent string) (*approvedInboxPlan, error) {
	if p := decodeInboxPlanDecision(authorization.Content); p != nil {
		return decodeApprovedInboxPlan(authorization.Content), nil
	}
	if agent == "" {
		store, ok := s.projectStudioRepo.(db.InboxRequestRepository)
		if !ok {
			return nil, errors.New("approval storage unavailable")
		}
		requests, err := store.ListInboxRequests(ctx, authorization.RoomID)
		if err != nil {
			return nil, err
		}
		for _, q := range requests {
			if q.OwnerID == owner && q.MessageID == authorization.ID && q.State != "cancelled" && q.State != "canceled" && q.State != "cancelling" {
				agent = q.AgentID
			}
		}
		if agent == "" {
			return nil, errors.New("owner approval unavailable")
		}
	}
	messages, err := s.projectStudioRepo.ListMessagesByRoomID(ctx, authorization.RoomID)
	if err != nil {
		return nil, err
	}
	var previous *db.ProjectMessage
	for i := range messages {
		m := &messages[i]
		if !m.CreatedAt.Before(authorization.CreatedAt) {
			continue
		}
		if previous == nil || m.CreatedAt.After(previous.CreatedAt) || (m.CreatedAt.Equal(previous.CreatedAt) && m.ID > previous.ID) {
			previous = m
		}
	}
	if previous == nil || previous.AuthorType != "agent" || previous.AuthorID != agent || parseInboxApprovalPlan(previous.Content) == nil {
		return nil, nil
	}
	p, err := s.persistedInboxPlan(ctx, authorization.RoomID, owner, agent, previous.ID)
	if err != nil {
		return nil, err
	}
	if p == nil {
		return nil, errors.New("saved proposal unavailable")
	}
	return p, nil
}
