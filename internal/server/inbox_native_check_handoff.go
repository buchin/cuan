package server

import (
	"context"
	"encoding/json"

	"github.com/buchin/cuan/internal/db"
)

// A native failed check already determines the next stage. Do not spend a model
// turn rediscovering it. This is a correction request, never source acceptance;
// applyInboxReviewEvaluation retains authorization, budgets and launch dedup.
func (s *Server) inboxNativeCheckHandoff(ctx context.Context, q db.InboxRequest) (string, bool) {
	d := s.inboxReviewedTask(ctx, q)
	if d == nil || nativeProjectCheckFailure(d) == "" || db.InboxRetryNeedsReconciliation(q) || isCustomerReplyDelivery(q, d) {
		return "", false
	}
	if changed, err := s.inboxReviewOwnerContextChanged(ctx, q); changed || err != nil {
		return "", false
	}
	decision := s.requirePublicationReview(ctx, d, inboxReviewEvaluation{Decision: "revise"})
	if decision.RemainingWork != "implementation" || decision.ManagerAction != "replan" || decision.SourceReview != "not_checked" {
		return "", false
	}
	raw, err := json.Marshal(decision)
	return string(raw), err == nil
}
