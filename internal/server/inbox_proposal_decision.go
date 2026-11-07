package server

import (
	"context"

	"github.com/buchin/cuan/internal/db"
)

const inboxProposalDecisionInstruction = `This is a chat reply to the immediately preceding verified saved proposal. Do not use tools or inspect repository/host files. Use the supplied conversation and original proposal to understand the user's intent. A question, objection, deferral or requested change is NOT approval: answer it or propose the revised scope without executing. If the user clearly authorizes the saved proposal, emit action kind delegation with payload containing ONLY proposal_message_id (the preceding proposal's message UUID) and authorization_message_id (the current owner message UUID). Do not copy or rewrite its title, description, project, category or limits: the application loads the exact saved proposal. Do not invent a technical investigation or another approval ceremony. Missing consequential information still requires clarification. This mode selects conversational context, never grants execution authority.`

func (s *Server) inboxProposalDecision(ctx context.Context, q db.InboxRequest) bool {
	if len(s.inboxAttachmentIDs(ctx, q)) != 0 {
		return false
	}
	m, err := s.projectStudioRepo.GetMessageByID(ctx, q.MessageID)
	if err != nil || m == nil || m.RoomID != q.RoomID || m.AuthorType != "user" {
		return false
	}
	p, err := s.delegationPlan(ctx, m, q.OwnerID, "")
	return err == nil && p != nil && p.Plan.Continuation == nil
}
