package server

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"time"

	svix "github.com/svix/svix-webhooks/go"
)

var newResendWebhook = svix.NewWebhook

type ChatEmailDelivery struct {
	Status        string `json:"status"`
	RecipientType string `json:"recipient_type"`
}

func (s *Server) handleResendReceiptWebhook(w http.ResponseWriter, r *http.Request) {
	settings, err := s.GetResendSettings(r.Context())
	if err != nil || settings.WebhookSecret == "" || s.db == nil {
		respondJSONError(w, 503, "Email receipts unavailable")
		return
	}
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 1<<20))
	if err != nil {
		respondJSONError(w, 400, "Invalid webhook")
		return
	}
	verifier, err := newResendWebhook(settings.WebhookSecret)
	if err != nil || verifier.Verify(body, r.Header) != nil {
		respondJSONError(w, 400, "Invalid webhook signature")
		return
	}
	var event struct {
		Type      string    `json:"type"`
		CreatedAt time.Time `json:"created_at"`
		Data      struct {
			EmailID string `json:"email_id"`
		} `json:"data"`
	}
	if json.Unmarshal(body, &event) != nil || event.Data.EmailID == "" || len(event.Data.EmailID) > 128 || event.CreatedAt.IsZero() {
		respondJSONError(w, 400, "Invalid webhook event")
		return
	}
	rank := map[string]int{"email.sent": 1, "email.delivery_delayed": 2, "email.delivered": 3, "email.failed": 4, "email.bounced": 4, "email.complained": 5, "email.suppressed": 4}[event.Type]
	if rank == 0 {
		respondJSON(w, 200, map[string]bool{"ok": true})
		return
	}
	status := event.Type[len("email."):]
	_, err = s.db.ExecContext(r.Context(), `INSERT INTO chat_email_receipts(email_id,status,status_rank,occurred_at) VALUES($1,$2,$3,$4)
 ON CONFLICT(email_id) DO UPDATE SET status=EXCLUDED.status,status_rank=EXCLUDED.status_rank,occurred_at=EXCLUDED.occurred_at
 WHERE EXCLUDED.status_rank>chat_email_receipts.status_rank OR (EXCLUDED.status_rank=chat_email_receipts.status_rank AND EXCLUDED.occurred_at>=chat_email_receipts.occurred_at)`, event.Data.EmailID, status, rank, event.CreatedAt)
	if err != nil {
		respondJSONError(w, 503, "Could not save email receipt")
		return
	}
	rows, err := s.db.QueryContext(r.Context(), `SELECT DISTINCT m.room_id::text FROM chat_message_emails e JOIN project_messages m ON m.id=e.message_id WHERE e.email_id=$1`, event.Data.EmailID)
	if err != nil {
		respondJSONError(w, 503, "Could not publish email receipt")
		return
	}
	defer rows.Close()
	for rows.Next() {
		var room string
		if rows.Scan(&room) == nil {
			s.publishInbox(room)
		}
	}
	if rows.Err() != nil {
		respondJSONError(w, 503, "Could not publish email receipt")
		return
	}
	respondJSON(w, 200, map[string]bool{"ok": true})
}

func (s *Server) recordChatEmail(ctx context.Context, room, emailID string, ids []string, recipient string) error {
	if s.db == nil || emailID == "" {
		return nil
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	_, err = tx.ExecContext(ctx, `INSERT INTO chat_email_receipts(email_id,status,status_rank,occurred_at) VALUES($1,'sent',1,NOW()) ON CONFLICT DO NOTHING`, emailID)
	if err != nil {
		return err
	}
	for _, id := range ids {
		_, err = tx.ExecContext(ctx, `INSERT INTO chat_message_emails(message_id,email_id,recipient_type) SELECT id,$2,$3 FROM project_messages WHERE id=$1::uuid AND room_id=$4::uuid ON CONFLICT DO NOTHING`, id, emailID, recipient, room)
		if err != nil {
			return err
		}
	}
	if err = tx.Commit(); err != nil {
		return err
	}
	s.publishInbox(room)
	return nil
}
func (s *Server) chatEmailDeliveries(ctx context.Context, room string) (map[string]*ChatEmailDelivery, error) {
	out := map[string]*ChatEmailDelivery{}
	if s.db == nil {
		return out, nil
	}
	rows, err := s.db.QueryContext(ctx, `SELECT DISTINCT ON (e.message_id) e.message_id::text,r.status,e.recipient_type FROM chat_message_emails e JOIN chat_email_receipts r ON r.email_id=e.email_id JOIN project_messages m ON m.id=e.message_id WHERE m.room_id=$1::uuid ORDER BY e.message_id,e.linked_at DESC,e.email_id DESC`, room)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var id string
		v := &ChatEmailDelivery{}
		if err = rows.Scan(&id, &v.Status, &v.RecipientType); err != nil {
			return nil, err
		}
		out[id] = v
	}
	return out, rows.Err()
}
