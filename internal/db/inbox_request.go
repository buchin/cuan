package db

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"sort"
	"time"
)

var ErrInboxQueueFull = errors.New("inbox queue full")

var ErrInboxRequestConflict = errors.New("request key already belongs to different content")

type InboxRequest struct {
	OwnerID     string      `json:"-"`
	AgentID     string      `json:"agent_id,omitempty"`
	RoomID      string      `json:"room_id"`
	RequestID   string      `json:"request_id"`
	PayloadHash string      `json:"-"`
	MessageID   string      `json:"message_id"`
	ReplyID     string      `json:"reply_id"`
	State       string      `json:"state"`
	SafeError   string      `json:"safe_error,omitempty"`
	CreatedAt   time.Time   `json:"created_at"`
	Deadline    time.Time   `json:"deadline"`
	StartedAt   *time.Time  `json:"started_at,omitempty"`
	Steps       []InboxStep `json:"steps,omitempty"`
}

type InboxStep struct {
	Seq  int       `json:"seq"`
	Kind string    `json:"kind"`
	Text string    `json:"text"`
	At   time.Time `json:"at"`
}

// InboxLifecycleRepository extends legacy acceptance with atomic FIFO claims.
type InboxLifecycleRepository interface {
	InboxRequestRepository
	ClaimInboxRequest(context.Context, string, string) (InboxRequest, bool, error)
	AppendInboxStep(context.Context, string, string, string, string) error
	CancelInboxRequest(context.Context, string, string) (bool, error)
}

type InboxRequestRepository interface {
	AcceptInboxRequest(context.Context, InboxRequest, *ProjectMessage) (InboxRequest, bool, error)
	FinishInboxRequest(context.Context, InboxRequest, *ProjectMessage) error
	ListInboxRequests(context.Context, string) ([]InboxRequest, error)
}

const interruptedInbox = "The process was interrupted or timed out. Your message is saved, but the provider's result is unknown. Refresh history to check. This request was not rerun."

func (r *SQLProjectStudioRepository) AcceptInboxRequest(ctx context.Context, q InboxRequest, m *ProjectMessage) (InboxRequest, bool, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return q, false, err
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, "inbox:"+q.RoomID); err != nil {
		return q, false, err
	}
	var count int
	if err = tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM inbox_requests WHERE room_id=$1 AND state IN ('queued','running','cancelling') AND NOT EXISTS(SELECT 1 FROM inbox_requests WHERE room_id=$1 AND request_id=$2)`, q.RoomID, q.RequestID).Scan(&count); err != nil {
		return q, false, err
	}
	if count >= 20 {
		return q, false, ErrInboxQueueFull
	}
	result, err := tx.ExecContext(ctx, `INSERT INTO inbox_requests(room_id,request_id,payload_hash,message_id,reply_id,state,created_at,deadline,owner_id,agent_id) VALUES($1,$2,$3,$4,$5,$8,$6,$7,$9,$10) ON CONFLICT DO NOTHING`, q.RoomID, q.RequestID, q.PayloadHash, q.MessageID, q.ReplyID, q.CreatedAt, q.Deadline, q.State, q.OwnerID, q.AgentID)
	if err != nil {
		return q, false, err
	}
	n, err := result.RowsAffected()
	if err != nil {
		return q, false, err
	}
	if n == 0 {
		var old InboxRequest
		var steps []byte
		err = tx.QueryRowContext(ctx, `SELECT room_id,request_id,payload_hash,message_id,reply_id,state,safe_error,created_at,deadline,started_at,steps,owner_id,agent_id FROM inbox_requests WHERE room_id=$1 AND request_id=$2`, q.RoomID, q.RequestID).Scan(&old.RoomID, &old.RequestID, &old.PayloadHash, &old.MessageID, &old.ReplyID, &old.State, &old.SafeError, &old.CreatedAt, &old.Deadline, &old.StartedAt, &steps, &old.OwnerID, &old.AgentID)
		if err == nil {
			err = json.Unmarshal(steps, &old.Steps)
		}
		if err != nil {
			return q, false, err
		}
		if old.PayloadHash != q.PayloadHash {
			return old, false, ErrInboxRequestConflict
		}
		return old, false, tx.Commit()
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO project_messages(id,room_id,author_type,author_id,author_name,content,created_at,updated_at) VALUES($1,$2,$3,$4,$5,$6,$7,$7)`, m.ID, m.RoomID, m.AuthorType, m.AuthorID, m.AuthorName, m.Content, m.CreatedAt)
	if err != nil {
		return q, false, err
	}
	return q, true, tx.Commit()
}

func (r *SQLProjectStudioRepository) FinishInboxRequest(ctx context.Context, q InboxRequest, m *ProjectMessage) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var finalState string
	var raw []byte
	var archived int
	err = tx.QueryRowContext(ctx, `SELECT state,steps,COALESCE((SELECT MAX(seq) FROM inbox_request_steps s WHERE s.room_id=q.room_id AND s.request_id=q.request_id),0) FROM inbox_requests q WHERE room_id=$1 AND request_id=$2 AND state IN ('running','queued','cancelling') FOR UPDATE`, q.RoomID, q.RequestID).Scan(&finalState, &raw, &archived)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	var steps []InboxStep
	if err = json.Unmarshal(raw, &steps); err != nil {
		return err
	}
	if err = recordInboxStepTx(ctx, tx, q.RoomID, q.RequestID, steps, archived, "finished", "Reply processing ended."); err != nil {
		return err
	}
	err = tx.QueryRowContext(ctx, `UPDATE inbox_requests SET deadline=CASE WHEN $3='queued' THEN NOW()+INTERVAL '24 hours' ELSE deadline END,state=CASE WHEN state='cancelling' THEN 'cancelled' ELSE $3 END,safe_error=CASE WHEN state='cancelling' THEN safe_error ELSE $4 END WHERE room_id=$1 AND request_id=$2 RETURNING state`, q.RoomID, q.RequestID, q.State, q.SafeError).Scan(&finalState)
	if err != nil {
		return err
	}
	n := 1
	if finalState == "cancelled" {
		m = nil
	}
	if n > 0 && m != nil {
		_, err = tx.ExecContext(ctx, `INSERT INTO project_messages(id,room_id,author_type,author_id,author_name,content,reply_to_id,created_at,updated_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$8)`, m.ID, m.RoomID, m.AuthorType, m.AuthorID, m.AuthorName, m.Content, q.MessageID, m.CreatedAt)
		if err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (r *SQLProjectStudioRepository) ListInboxRequests(ctx context.Context, room string) ([]InboxRequest, error) {
	_, err := r.db.ExecContext(ctx, `UPDATE inbox_requests SET state='interrupted',safe_error=$2 WHERE room_id=$1 AND state IN ('running','cancelling') AND deadline < NOW()`, room, interruptedInbox)
	if err != nil {
		return nil, err
	}
	rows, err := r.db.QueryContext(ctx, `SELECT room_id,request_id,payload_hash,message_id,reply_id,state,safe_error,created_at,deadline,started_at,steps,owner_id,agent_id FROM inbox_requests WHERE room_id=$1 ORDER BY created_at,request_id`, room)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []InboxRequest{}
	for rows.Next() {
		var q InboxRequest
		var steps []byte
		if err := rows.Scan(&q.RoomID, &q.RequestID, &q.PayloadHash, &q.MessageID, &q.ReplyID, &q.State, &q.SafeError, &q.CreatedAt, &q.Deadline, &q.StartedAt, &steps, &q.OwnerID, &q.AgentID); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(steps, &q.Steps); err != nil {
			return nil, err
		}
		q.Steps = append([]InboxStep(nil), q.Steps...)
		out = append(out, q)
	}
	return out, rows.Err()
}

func (r *MemoryProjectStudioRepository) AcceptInboxRequest(ctx context.Context, q InboxRequest, m *ProjectMessage) (InboxRequest, bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	key := q.RoomID + q.RequestID
	if old, ok := r.requests[key]; ok {
		if old.PayloadHash != q.PayloadHash {
			return old, false, ErrInboxRequestConflict
		}
		return old, false, nil
	}
	count := 0
	for _, other := range r.requests {
		if other.RoomID == q.RoomID && (other.State == "queued" || other.State == "running" || other.State == "cancelling") {
			count++
		}
	}
	if count >= 20 {
		return q, false, ErrInboxQueueFull
	}
	r.requests[key] = q
	r.messages[m.ID] = cloneMessage(m)
	return q, true, nil
}
func (r *MemoryProjectStudioRepository) FinishInboxRequest(ctx context.Context, q InboxRequest, m *ProjectMessage) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	key := q.RoomID + q.RequestID
	if old, ok := r.requests[key]; ok && (old.State == "running" || old.State == "queued" || old.State == "cancelling") {
		if old.State == "cancelling" {
			q.State = "cancelled"
			q.SafeError = old.SafeError
			m = nil
		}
		q.Steps = append([]InboxStep(nil), old.Steps...)
		r.recordInboxStep(key, &q, "finished", "Reply processing ended.")
		q.StartedAt = old.StartedAt
		r.requests[key] = q
		if m != nil {
			r.messages[m.ID] = cloneMessage(m)
		}
	}
	return nil
}
func (r *MemoryProjectStudioRepository) ListInboxRequests(ctx context.Context, room string) ([]InboxRequest, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := []InboxRequest{}
	for key, q := range r.requests {
		if q.RoomID != room {
			continue
		}
		if (q.State == "running" || q.State == "cancelling") && time.Now().After(q.Deadline) {
			q.State = "interrupted"
			q.SafeError = interruptedInbox
			r.requests[key] = q
		}
		q.Steps = append([]InboxStep(nil), q.Steps...)
		out = append(out, q)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].CreatedAt.Equal(out[j].CreatedAt) {
			return out[i].RequestID < out[j].RequestID
		}
		return out[i].CreatedAt.Before(out[j].CreatedAt)
	})
	return out, nil
}

// ClaimInboxRequest serializes claimers across processes and claims only the FIFO
// head. A running request is never replayed after an unknown provider outcome.
func (r *SQLProjectStudioRepository) ClaimInboxRequest(ctx context.Context, room, id string) (InboxRequest, bool, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return InboxRequest{}, false, err
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1, 0))`, "inbox:"+room); err != nil {
		return InboxRequest{}, false, err
	}
	var q InboxRequest
	var steps []byte
	err = tx.QueryRowContext(ctx, `UPDATE inbox_requests SET state='running',safe_error='',started_at=NOW(),deadline=NOW()+INTERVAL '360 seconds'
 WHERE room_id=$1 AND request_id=$2 AND state='queued'
 AND NOT EXISTS(SELECT 1 FROM inbox_requests WHERE room_id=$1 AND state IN ('running','cancelling'))
 AND request_id=(SELECT request_id FROM inbox_requests WHERE room_id=$1 AND state='queued' ORDER BY created_at,request_id LIMIT 1)
 RETURNING room_id,request_id,payload_hash,message_id,reply_id,state,safe_error,created_at,deadline,started_at,steps,owner_id,agent_id`, room, id).Scan(&q.RoomID, &q.RequestID, &q.PayloadHash, &q.MessageID, &q.ReplyID, &q.State, &q.SafeError, &q.CreatedAt, &q.Deadline, &q.StartedAt, &steps, &q.OwnerID, &q.AgentID)
	if errors.Is(err, sql.ErrNoRows) {
		return q, false, nil
	}
	if err != nil {
		return q, false, err
	}
	if err = json.Unmarshal(steps, &q.Steps); err != nil {
		return q, false, err
	}
	return q, true, tx.Commit()
}
func (r *SQLProjectStudioRepository) AppendInboxStep(ctx context.Context, room, id, kind, text string) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var raw []byte
	var archived int
	err = tx.QueryRowContext(ctx, `SELECT steps,COALESCE((SELECT MAX(seq) FROM inbox_request_steps s WHERE s.room_id=q.room_id AND s.request_id=q.request_id),0) FROM inbox_requests q WHERE room_id=$1 AND request_id=$2 AND state='running' FOR UPDATE`, room, id).Scan(&raw, &archived)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	var steps []InboxStep
	if err = json.Unmarshal(raw, &steps); err != nil {
		return err
	}
	if err = recordInboxStepTx(ctx, tx, room, id, steps, archived, kind, text); err != nil {
		return err
	}
	return tx.Commit()
}

func (r *SQLProjectStudioRepository) CancelInboxRequest(ctx context.Context, room, id string) (bool, error) {
	result, err := r.db.ExecContext(ctx, `UPDATE inbox_requests SET state=CASE WHEN state='running' THEN 'cancelling' ELSE 'cancelled' END,safe_error='Dibatalkan. Pesan tersimpan; efek provider yang sudah terjadi tidak dapat ditarik kembali.' WHERE room_id=$1 AND request_id=$2 AND state IN ('queued','running')`, room, id)
	if err != nil {
		return false, err
	}
	n, err := result.RowsAffected()
	return n > 0, err
}
func (r *MemoryProjectStudioRepository) ClaimInboxRequest(ctx context.Context, room, id string) (InboxRequest, bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	key := room + id
	q, ok := r.requests[key]
	if !ok || q.State != "queued" {
		return q, false, nil
	}
	for _, other := range r.requests {
		if other.RoomID != room {
			continue
		}
		if (other.State == "running" || other.State == "cancelling") || (other.State == "queued" && (other.CreatedAt.Before(q.CreatedAt) || (other.CreatedAt.Equal(q.CreatedAt) && other.RequestID < id))) {
			return q, false, nil
		}
	}
	now := time.Now().UTC()
	q.State = "running"
	q.SafeError = ""
	q.StartedAt = &now
	q.Deadline = now.Add(360 * time.Second)
	r.requests[key] = q
	return q, true, nil
}
func (r *MemoryProjectStudioRepository) AppendInboxStep(ctx context.Context, room, id, kind, text string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	key := room + id
	q, ok := r.requests[key]
	if ok && q.State == "running" {
		r.recordInboxStep(key, &q, kind, text)
		r.requests[key] = q
	}
	return nil
}
func (r *MemoryProjectStudioRepository) CancelInboxRequest(ctx context.Context, room, id string) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	key := room + id
	q, ok := r.requests[key]
	if !ok || (q.State != "running" && q.State != "queued") {
		return false, nil
	}
	if q.State == "running" {
		q.State = "cancelling"
	} else {
		q.State = "cancelled"
	}
	q.SafeError = "Cancelled. Your message is saved. Provider actions that already occurred cannot be undone."
	r.requests[key] = q
	return true, nil
}
