package server

import (
	"context"

	"github.com/buchin/cuan/internal/db"
)

const inboxProposalDecisionInstruction = `This is a chat reply to the immediately preceding verified saved proposal. Do not use tools or inspect repository/host files. Use the supplied conversation and original proposal to understand the user's intent. A question, objection, deferral or requested change is NOT approval: answer it or propose the revised scope without executing. If the user clearly authorizes the saved proposal, delegate its exact title, description and project with the current owner message as authorization; preserve all limits. Do not invent a technical investigation or another approval ceremony. Missing consequential information still requires clarification. This mode selects conversational context, never grants execution authority.`

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
