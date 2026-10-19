package db

import (
	"context"
	"errors"
	"time"
)

var ErrInboxRequestConflict = errors.New("request key already belongs to different content")

type InboxRequest struct {
	RoomID      string    `json:"room_id"`
	RequestID   string    `json:"request_id"`
	PayloadHash string    `json:"-"`
	MessageID   string    `json:"message_id"`
	ReplyID     string    `json:"reply_id"`
	State       string    `json:"state"`
	SafeError   string    `json:"safe_error,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	Deadline    time.Time `json:"deadline"`
}

type InboxRequestRepository interface {
	AcceptInboxRequest(context.Context, InboxRequest, *ProjectMessage) (InboxRequest, bool, error)
	FinishInboxRequest(context.Context, InboxRequest, *ProjectMessage) error
	ListInboxRequests(context.Context, string) ([]InboxRequest, error)
}

const interruptedInbox = "Proses terputus atau melewati batas waktu. Pesan tersimpan; hasil provider belum dapat dipastikan. Segarkan untuk memeriksa riwayat. Permintaan ini tidak dijalankan ulang."

func (r *SQLProjectStudioRepository) AcceptInboxRequest(ctx context.Context, q InboxRequest, m *ProjectMessage) (InboxRequest, bool, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return q, false, err
	}
	defer tx.Rollback()
	result, err := tx.ExecContext(ctx, `INSERT INTO inbox_requests(room_id,request_id,payload_hash,message_id,reply_id,state,created_at,deadline) VALUES($1,$2,$3,$4,$5,'running',$6,$7) ON CONFLICT DO NOTHING`, q.RoomID, q.RequestID, q.PayloadHash, q.MessageID, q.ReplyID, q.CreatedAt, q.Deadline)
	if err != nil {
		return q, false, err
	}
	n, err := result.RowsAffected()
	if err != nil {
		return q, false, err
	}
	if n == 0 {
		var old InboxRequest
		err = tx.QueryRowContext(ctx, `SELECT room_id,request_id,payload_hash,message_id,reply_id,state,safe_error,created_at,deadline FROM inbox_requests WHERE room_id=$1 AND request_id=$2`, q.RoomID, q.RequestID).Scan(&old.RoomID, &old.RequestID, &old.PayloadHash, &old.MessageID, &old.ReplyID, &old.State, &old.SafeError, &old.CreatedAt, &old.Deadline)
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
	result, err := tx.ExecContext(ctx, `UPDATE inbox_requests SET state=$3,safe_error=$4 WHERE room_id=$1 AND request_id=$2 AND state='running'`, q.RoomID, q.RequestID, q.State, q.SafeError)
	if err != nil {
		return err
	}
	n, err := result.RowsAffected()
	if err != nil {
		return err
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
	_, err := r.db.ExecContext(ctx, `UPDATE inbox_requests SET state='interrupted',safe_error=$2 WHERE room_id=$1 AND state='running' AND deadline < NOW()`, room, interruptedInbox)
	if err != nil {
		return nil, err
	}
	rows, err := r.db.QueryContext(ctx, `SELECT room_id,request_id,payload_hash,message_id,reply_id,state,safe_error,created_at,deadline FROM inbox_requests WHERE room_id=$1 ORDER BY created_at`, room)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []InboxRequest{}
	for rows.Next() {
		var q InboxRequest
		if err := rows.Scan(&q.RoomID, &q.RequestID, &q.PayloadHash, &q.MessageID, &q.ReplyID, &q.State, &q.SafeError, &q.CreatedAt, &q.Deadline); err != nil {
			return nil, err
		}
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
	r.requests[key] = q
	r.messages[m.ID] = cloneMessage(m)
	return q, true, nil
}
func (r *MemoryProjectStudioRepository) FinishInboxRequest(ctx context.Context, q InboxRequest, m *ProjectMessage) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	key := q.RoomID + q.RequestID
	if old, ok := r.requests[key]; ok && old.State == "running" {
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
		if q.State == "running" && time.Now().After(q.Deadline) {
			q.State = "interrupted"
			q.SafeError = interruptedInbox
			r.requests[key] = q
		}
		out = append(out, q)
	}
	return out, nil
}
