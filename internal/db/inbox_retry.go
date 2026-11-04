package db

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

var ErrInboxRetryUnavailable = errors.New("request cannot be safely retried")

// Recovery decisions use recorded server evidence, never a client-provided flag.
func InboxRecoveryAction(q InboxRequest) string {
	if q.State != "failed" {
		if q.State == "interrupted" || q.State == "cancelled" {
			return "check_status"
		}
		return ""
	}
	safe, observedTool := false, false
	hasQuotaCooldown := false
	hasPermissionDenied := false
	hasToolBudgetExceeded := false
	for _, step := range q.Steps {
		if step.Kind == "runtime_permission_denied" {
			hasPermissionDenied = true
			return "check_status"
		}
		if step.Kind == "runtime_tool_budget_exceeded" || strings.Contains(strings.ToLower(step.Text), "tool budget") || strings.Contains(strings.ToLower(step.Text), "tool limit") {
			hasToolBudgetExceeded = true
		}
		if step.Kind == "provider_cooldown" || strings.Contains(strings.ToLower(step.Text), "usage limit") || strings.Contains(strings.ToLower(step.Text), "quota") {
			hasQuotaCooldown = true
		}
		if strings.HasPrefix(step.Kind, "runtime_tool_") {
			observedTool = true
			safe = false
		}
		if step.Kind == "provider" {
			safe = false
		}
		if step.Kind == "recovery_safe" {
			safe = true
		}
	}
	lower := strings.ToLower(q.SafeError)
	if (hasToolBudgetExceeded || strings.Contains(lower, "tool budget")) && !hasPermissionDenied {
		return "continue_analysis"
	}
	// The executor emits recovery_safe only after a verified final result with
	// no unsafe/unknown tool activity. Earlier tool entries alone are not proof.
	if observedTool && !safe {
		return "check_status"
	}
	if strings.Contains(lower, "location restriction") {
		return "check_status"
	}
	for _, unknown := range []string{"unknown", "interrupted", "timed out", "before a final result", "cannot be safely repeated", "invalid runtime", "size limit"} {
		if strings.Contains(lower, unknown) {
			return "check_status"
		}
	}
	if hasQuotaCooldown || strings.Contains(lower, "quota") || strings.Contains(lower, "no allowed account/model route") || strings.Contains(lower, "usage limit") {
		return "check_usage"
	}
	if strings.Contains(lower, "sign-in") || strings.Contains(lower, "agent connection") {
		return "check_agent"
	}
	for _, known := range []string{"The chat sandbox is unavailable.", "Provider is unavailable.", "The agent is unavailable or disabled.", "Agent list is unavailable.", "Could not read conversation context."} {
		if strings.HasPrefix(q.SafeError, known) {
			return "try_again"
		}
	}
	if safe {
		return "try_again"
	}
	return "check_status"
}

// One child per failed attempt. Repeated clicks/reloads return the same child.
func InboxRetryID(source string) string {
	sum := sha256.Sum256([]byte("cuan-inbox-retry-v1:" + source))
	return fmt.Sprintf("%x-%x-%x-%x-%x", sum[:4], sum[4:6], sum[6:8], sum[8:10], sum[10:16])
}

// Automatic recovery is opt-in per newly started turn and consumes at most one
// extra attempt. Old failures, later user turns, cancellations and unknown tool
// outcomes are never replayed by a deployment or a dispatcher restart.
func InboxAutomaticRecoveryCandidate(requests []InboxRequest, now time.Time) *InboxRequest {
	var latest *InboxRequest
	counts := map[string]int{}
	for i := range requests {
		q := &requests[i]
		counts[q.MessageID]++
		if q.State == "queued" || q.State == "running" || q.State == "cancelling" {
			return nil
		}
		if latest == nil || q.CreatedAt.After(latest.CreatedAt) || (q.CreatedAt.Equal(latest.CreatedAt) && q.RequestID > latest.RequestID) {
			latest = q
		}
	}
	if latest == nil || latest.OwnerID == "" || latest.AgentID == "" || counts[latest.MessageID] != 1 || InboxRecoveryAction(*latest) != "try_again" {
		return nil
	}
	policy, proof := false, false
	var finished time.Time
	for _, step := range latest.Steps {
		if step.Kind == "recovery_policy" && step.Text == "safe-reply-v1" {
			policy = true
		}
		if step.Kind == "provider" {
			proof = false
		}
		if step.Kind == "recovery_safe" {
			proof = true
		}
		if step.Kind == "finished" {
			finished = step.At
		}
	}
	elapsed := now.Sub(finished)
	if !policy || !proof || finished.IsZero() || elapsed < 2*time.Second || elapsed > 2*time.Minute {
		return nil
	}
	copy := *latest
	return &copy
}

type InboxAutoRetryRepository interface {
	AutoRetryInboxRequest(context.Context, string, string, string) (InboxRequest, bool, error)
}

type InboxRetryRepository interface {
	RetryInboxRequest(context.Context, string, string, string) (InboxRequest, bool, error)
}

func retryRequest(q InboxRequest) InboxRequest {
	now := time.Now().UTC()
	child := InboxRequest{OwnerID: q.OwnerID, AgentID: q.AgentID, RoomID: q.RoomID, RequestID: InboxRetryID(q.RequestID),
		PayloadHash: q.PayloadHash, MessageID: q.MessageID, ReplyID: GenerateUUID(), State: "queued", CreatedAt: now, Deadline: now.Add(24 * time.Hour),
		Steps: []InboxStep{{Seq: 1, Kind: "retry", Text: "A new attempt was requested for the saved message.", At: now}}}
	if InboxRetryNeedsReconciliation(q) || InboxRecoveryAction(q) == "check_status" || q.State == "interrupted" {
		child.Steps[0].Kind = "retry_reconcile"
		child.Steps[0].Text = "Re-evaluate the saved message from current recorded state; do not replay uncertain actions."
	}
	return child
}

func InboxRetryNeedsReconciliation(q InboxRequest) bool {
	return len(q.Steps) > 0 && q.Steps[0].Kind == "retry_reconcile"
}

func manualInboxRetryAllowed(q InboxRequest) bool {
	return q.State == "failed" || q.State == "interrupted"
}

func (r *MemoryProjectStudioRepository) RetryInboxRequest(ctx context.Context, room, source, owner string) (InboxRequest, bool, error) {
	return r.retryInboxRequest(ctx, room, source, owner, false)
}
func (r *MemoryProjectStudioRepository) AutoRetryInboxRequest(ctx context.Context, room, source, owner string) (InboxRequest, bool, error) {
	return r.retryInboxRequest(ctx, room, source, owner, true)
}
func (r *MemoryProjectStudioRepository) retryInboxRequest(ctx context.Context, room, source, owner string, automatic bool) (InboxRequest, bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	q, ok := r.requests[room+source]
	if !ok || q.OwnerID != owner || q.AgentID == "" {
		return InboxRequest{}, false, ErrInboxRetryUnavailable
	}
	child := retryRequest(q)
	if old, ok := r.requests[room+child.RequestID]; ok {
		return old, false, nil
	}
	if automatic {
		rows := []InboxRequest{}
		for _, row := range r.requests {
			if row.RoomID == room {
				rows = append(rows, row)
			}
		}
		candidate := InboxAutomaticRecoveryCandidate(rows, time.Now())
		if candidate == nil || candidate.RequestID != source {
			return q, false, ErrInboxRetryUnavailable
		}
		child.Steps[0].Kind = "retry_auto"
		child.Steps[0].Text = "Recovering this reply automatically from the saved message."
		if InboxRecoveryAction(q) != "try_again" {
			return q, false, ErrInboxRetryUnavailable
		}
	} else {
		if !manualInboxRetryAllowed(q) {
			return q, false, ErrInboxRetryUnavailable
		}
	}
	count := 0
	for _, other := range r.requests {
		if other.RoomID == room && (other.State == "queued" || other.State == "running" || other.State == "cancelling") {
			count++
		}
	}
	if count >= 20 {
		return q, false, ErrInboxQueueFull
	}
	if m, ok := r.messages[q.MessageID]; !ok || m.RoomID != room {
		return q, false, ErrInboxRetryUnavailable
	}
	r.requests[room+child.RequestID] = child
	return child, true, nil
}

func (r *SQLProjectStudioRepository) RetryInboxRequest(ctx context.Context, room, source, owner string) (InboxRequest, bool, error) {
	return r.retryInboxRequest(ctx, room, source, owner, false)
}
func (r *SQLProjectStudioRepository) AutoRetryInboxRequest(ctx context.Context, room, source, owner string) (InboxRequest, bool, error) {
	return r.retryInboxRequest(ctx, room, source, owner, true)
}
func (r *SQLProjectStudioRepository) retryInboxRequest(ctx context.Context, room, source, owner string, automatic bool) (InboxRequest, bool, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return InboxRequest{}, false, err
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, "SELECT pg_advisory_xact_lock(hashtextextended($1,0))", "inbox:"+room); err != nil {
		return InboxRequest{}, false, err
	}
	read := func(id string) (InboxRequest, error) {
		var q InboxRequest
		var steps []byte
		err := tx.QueryRowContext(ctx, `SELECT room_id,request_id,payload_hash,message_id,reply_id,state,safe_error,created_at,deadline,started_at,steps,owner_id,agent_id FROM inbox_requests WHERE room_id=$1 AND request_id=$2`, room, id).Scan(&q.RoomID, &q.RequestID, &q.PayloadHash, &q.MessageID, &q.ReplyID, &q.State, &q.SafeError, &q.CreatedAt, &q.Deadline, &q.StartedAt, &steps, &q.OwnerID, &q.AgentID)
		if err == nil {
			err = json.Unmarshal(steps, &q.Steps)
		}
		return q, err
	}
	q, err := read(source)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return InboxRequest{}, false, ErrInboxRetryUnavailable
		}
		return InboxRequest{}, false, err
	}
	if q.OwnerID != owner || q.AgentID == "" {
		return InboxRequest{}, false, ErrInboxRetryUnavailable
	}
	child := retryRequest(q)
	var exists bool
	if err = tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM inbox_requests WHERE room_id=$1 AND request_id=$2)`, room, child.RequestID).Scan(&exists); err != nil {
		return q, false, err
	}
	if exists {
		old, e := read(child.RequestID)
		if e != nil {
			return q, false, e
		}
		return old, false, tx.Commit()
	}
	if automatic {
		candidate := InboxAutomaticRecoveryCandidate([]InboxRequest{q}, time.Now())
		if candidate == nil {
			return q, false, ErrInboxRetryUnavailable
		}
		var latest string
		var siblings int
		if err = tx.QueryRowContext(ctx, `SELECT request_id FROM inbox_requests WHERE room_id=$1 ORDER BY created_at DESC,request_id DESC LIMIT 1`, room).Scan(&latest); err != nil {
			return q, false, err
		}
		if err = tx.QueryRowContext(ctx, `SELECT count(*) FROM inbox_requests WHERE room_id=$1 AND message_id=$2`, room, q.MessageID).Scan(&siblings); err != nil {
			return q, false, err
		}
		if latest != source || siblings != 1 {
			return q, false, ErrInboxRetryUnavailable
		}
		child.Steps[0].Kind = "retry_auto"
		child.Steps[0].Text = "Recovering this reply automatically from the saved message."
		if InboxRecoveryAction(q) != "try_again" {
			return q, false, ErrInboxRetryUnavailable
		}
	} else {
		if !manualInboxRetryAllowed(q) {
			return q, false, ErrInboxRetryUnavailable
		}
	}
	var count int
	if err = tx.QueryRowContext(ctx, `SELECT count(*) FROM inbox_requests WHERE room_id=$1 AND state IN ('queued','running','cancelling')`, room).Scan(&count); err != nil {
		return q, false, err
	}
	if automatic && count > 0 {
		return q, false, ErrInboxRetryUnavailable
	}
	if count >= 20 {
		return q, false, ErrInboxQueueFull
	}
	if err = tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM project_messages WHERE id=$1 AND room_id=$2)`, q.MessageID, room).Scan(&exists); err != nil {
		return q, false, err
	}
	if !exists {
		return q, false, ErrInboxRetryUnavailable
	}
	steps, _ := json.Marshal(child.Steps)
	_, err = tx.ExecContext(ctx, `INSERT INTO inbox_requests(room_id,request_id,payload_hash,message_id,reply_id,state,created_at,deadline,owner_id,agent_id,steps) VALUES($1,$2,$3,$4,$5,'queued',$6,$7,$8,$9,$10)`, room, child.RequestID, child.PayloadHash, child.MessageID, child.ReplyID, child.CreatedAt, child.Deadline, child.OwnerID, child.AgentID, steps)
	if err != nil {
		return q, false, err
	}
	return child, true, tx.Commit()
}
