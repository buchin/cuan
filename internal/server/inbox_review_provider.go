package server

import (
	"context"

	"github.com/buchin/cuan/internal/db"
)

const inboxReviewProviderWaiting = "The review service is temporarily unavailable. Saved work will be reviewed automatically."

// inboxProviderCapacityWait marks a turn whose provider ended on its own
// deadline. It is a routing signal for the failover loop, never a user-facing
// message: another permitted account or model should answer the same turn.
const inboxProviderCapacityWait = "\x00provider_capacity_wait"
const inboxReviewProviderPolicy = "Review inference reliability policy: v1"

// Only a validated tool-free evaluation can retry a transient provider failure.
// Unknown tools, malformed streams, cancellation, credentials and real work do
// not gain replay permission from a timeout-looking string.
func retryableInboxReviewProvider(ctx context.Context, failure string, safe bool, attachments int) bool {
	review, _ := ctx.Value(inboxTaskReviewKey{}).(bool)
	return review && attachments == 0 && safe && ctx.Err() == nil && failure == inboxReviewProviderWaiting
}

// Older workers converted a provider failure into a completed deterministic
// assessment. Re-evaluate that exact snapshot once; do not rerun task execution
// or rewrite the discarded provider result as a successful model review.
func legacyReviewProviderFailed(q db.InboxRequest) bool {
	if q.State != "completed" {
		return false
	}
	for i := len(q.Steps) - 1; i >= 0; i-- {
		step := q.Steps[i]
		if step.Kind == "result_validation" {
			return step.Text == "The provider timed out." || step.Text == "The provider reported a failure." || step.Text == "The runtime stopped without a final result."
		}
	}
	return false
}
