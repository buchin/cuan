package db

import (
	"context"
	"encoding/json"
	"time"
)

// MessageReceipt records explicit recipient acknowledgements, never polling.
type MessageReceipt struct {
	DeliveredAt     *time.Time `json:"delivered_at,omitempty"`
	ReadConfirmedAt *time.Time `json:"read_confirmed_at,omitempty"`
}

type ChatReceiptRepository interface {
	AcknowledgeMessages(context.Context, string, []string, bool) error
	MessageReceipts(context.Context, string) (map[string]MessageReceipt, error)
}

func (r *SQLProjectStudioRepository) AcknowledgeMessages(ctx context.Context, room string, ids []string, read bool) error {
	if len(ids) == 0 {
		return nil
	}
	raw, err := json.Marshal(ids)
	if err != nil {
		return err
	}
	_, err = r.db.ExecContext(ctx, `INSERT INTO chat_message_receipts(message_id,delivered_at,read_confirmed_at)
 SELECT id,NOW(),CASE WHEN $3 THEN NOW() END FROM project_messages
 WHERE room_id=$1::uuid AND id IN (SELECT value::uuid FROM jsonb_array_elements_text($2::jsonb) AS value)
 ON CONFLICT(message_id) DO UPDATE SET delivered_at=COALESCE(chat_message_receipts.delivered_at,EXCLUDED.delivered_at),
 read_confirmed_at=COALESCE(chat_message_receipts.read_confirmed_at,EXCLUDED.read_confirmed_at)`, room, string(raw), read)
	return err
}
func (r *SQLProjectStudioRepository) MessageReceipts(ctx context.Context, room string) (map[string]MessageReceipt, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT m.id::text,r.delivered_at,r.read_confirmed_at FROM chat_message_receipts r JOIN project_messages m ON m.id=r.message_id WHERE m.room_id=$1::uuid`, room)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]MessageReceipt{}
	for rows.Next() {
		var id string
		var v MessageReceipt
		if err = rows.Scan(&id, &v.DeliveredAt, &v.ReadConfirmedAt); err != nil {
			return nil, err
		}
		out[id] = v
	}
	return out, rows.Err()
}
func (m *MemoryProjectStudioRepository) AcknowledgeMessages(ctx context.Context, room string, ids []string, read bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.receipts == nil {
		m.receipts = map[string]MessageReceipt{}
	}
	now := time.Now().UTC()
	for _, id := range ids {
		msg := m.messages[id]
		if msg == nil || msg.RoomID != room {
			continue
		}
		v := m.receipts[id]
		if v.DeliveredAt == nil {
			v.DeliveredAt = &now
		}
		if read && v.ReadConfirmedAt == nil {
			v.ReadConfirmedAt = &now
		}
		m.receipts[id] = v
	}
	return nil
}
func (m *MemoryProjectStudioRepository) MessageReceipts(ctx context.Context, room string) (map[string]MessageReceipt, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := map[string]MessageReceipt{}
	for id, v := range m.receipts {
		if msg := m.messages[id]; msg != nil && msg.RoomID == room {
			out[id] = v
		}
	}
	return out, nil
}
