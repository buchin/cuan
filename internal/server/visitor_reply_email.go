package server

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/mail"
	"strings"
	"time"

	"github.com/buchin/cuan/internal/db"
	"github.com/buchin/cuan/internal/resend"
	"github.com/buchin/cuan/internal/scheduler"
)

func visitorReplyEmail(recipient, content, from, name, id string) (resend.EmailRequest, bool) {
	empty := resend.EmailRequest{}
	address, err := mail.ParseAddress(recipient)
	if err != nil || address.Address != recipient || strings.ContainsAny(recipient, "\r\n") {
		return empty, false
	}
	m := &db.ProjectMessage{AuthorType: "user", AuthorID: "operator_user", Content: content}
	if customerMessageInternal(m) {
		return empty, false
	}
	text, _ := decodeAttachmentMessage(content)
	if strings.TrimSpace(text) == "" {
		return empty, false
	}
	sender, err := mail.ParseAddress(from)
	if err != nil || sender.Address != from || strings.ContainsAny(from+name, "\r\n") {
		return empty, false
	}
	if name == "" {
		name = "Cuan"
	}
	return resend.EmailRequest{IdempotencyKey: "cuan-visitor/" + id, From: (&mail.Address{Name: name, Address: from}).String(), To: []string{recipient}, Subject: "Balasan untuk percakapan Anda", Text: text}, true
}

func (s *Server) registerVisitorReplyEmail() {
	if s.scheduler == nil {
		return
	}
	_ = s.scheduler.Register(&scheduler.Job{ID: "visitor_reply_email", Name: "Customer reply email", Description: "Send unread public replies to their connected customer", ScheduleType: scheduler.ScheduleInterval, Interval: time.Minute, Timeout: 55 * time.Second, Enabled: s.db != nil, Handler: s.sendPendingVisitorEmails})
}

// Freeze the exact provider request before any network call. Restarts and
// changed sender settings cannot change an already attempted idempotent send.
func (s *Server) prepareVisitorEmail(ctx context.Context, id string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var recipient, content string
	var existing []byte
	err = tx.QueryRowContext(ctx, `SELECT recipient,content,payload FROM chat_visitor_email_outbox WHERE message_id=$1::uuid AND state='pending' FOR UPDATE`, id).Scan(&recipient, &content, &existing)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	if len(existing) > 0 {
		return tx.Commit()
	}
	settings, err := s.GetResendSettings(ctx)
	if err != nil {
		return err
	}
	request, valid := visitorReplyEmail(recipient, content, settings.FromEmail, settings.FromName, id)
	if !settings.Enabled || settings.APIKey == "" || !valid {
		_, err = tx.ExecContext(ctx, `UPDATE chat_visitor_email_outbox SET state='skipped',last_error='Email unavailable or message is not a public text reply' WHERE message_id=$1::uuid`, id)
	} else {
		raw, e := json.Marshal(request)
		if e != nil {
			return e
		}
		_, err = tx.ExecContext(ctx, `UPDATE chat_visitor_email_outbox SET payload=$2::jsonb,connection_hash=$3 WHERE message_id=$1::uuid`, id, string(raw), fmt.Sprintf("%x", sha256.Sum256([]byte(settings.APIKey))))
	}
	if err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Server) sendVisitorEmail(ctx context.Context, id string) error {
	if err := s.prepareVisitorEmail(ctx, id); err != nil {
		return err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var raw []byte
	var room, connectionHash string
	var read, expired, contactChanged bool
	err = tx.QueryRowContext(ctx, `SELECT o.payload,m.room_id::text,COALESCE(r.read_confirmed_at IS NOT NULL,false),o.created_at < NOW()-INTERVAL '23 hours',c.email<>o.recipient,o.connection_hash
 FROM chat_visitor_email_outbox o JOIN project_messages m ON m.id=o.message_id JOIN crm_contacts c ON c.id=o.contact_id
 LEFT JOIN chat_message_receipts r ON r.message_id=m.id
 WHERE o.message_id=$1::uuid AND o.state='pending' AND o.next_attempt_at<=NOW() FOR UPDATE OF o`, id).Scan(&raw, &room, &read, &expired, &contactChanged, &connectionHash)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	state, reason := "", ""
	if read || contactChanged {
		state, reason = "skipped", "Already read or connected recipient changed"
	}
	if expired {
		state, reason = "failed", "Retry window expired; no automatic resend outside provider idempotency window"
	}
	settings, err := s.GetResendSettings(ctx)
	if err != nil {
		return err
	}
	if !settings.Enabled || settings.APIKey == "" {
		state, reason = "skipped", "Email integration disabled"
	}
	if connectionHash != fmt.Sprintf("%x", sha256.Sum256([]byte(settings.APIKey))) {
		state, reason = "failed", "Sending connection changed; automatic retry stopped to preserve provider idempotency"
	}
	if state != "" {
		_, err = tx.ExecContext(ctx, `UPDATE chat_visitor_email_outbox SET state=$2,last_error=$3 WHERE message_id=$1::uuid`, id, state, reason)
		if err != nil {
			return err
		}
		return tx.Commit()
	}
	var request resend.EmailRequest
	if json.Unmarshal(raw, &request) != nil || len(request.To) != 1 || request.From == "" || request.Text == "" {
		return errors.New("Saved customer email request is invalid")
	}
	request.IdempotencyKey = "cuan-visitor/" + id
	client, err := s.ResendClient(ctx)
	if err != nil {
		return err
	}
	resp, sendErr := client.SendEmail(ctx, request)
	if sendErr == nil && (resp == nil || resp.ID == "") {
		sendErr = errors.New("Provider did not confirm an email identifier")
	}
	if sendErr != nil {
		_, err = tx.ExecContext(ctx, `UPDATE chat_visitor_email_outbox SET attempts=attempts+1,next_attempt_at=NOW()+INTERVAL '5 minutes',last_error='Provider send not confirmed; retry uses the same saved request' WHERE message_id=$1::uuid`, id)
		if err != nil {
			return err
		}
		if err = tx.Commit(); err != nil {
			return err
		}
		return errors.New("Customer email send not confirmed")
	}
	// Receipt insertion and completion share this transaction. A crash after
	// provider acceptance retries the frozen request under the same provider key.
	if _, err = tx.ExecContext(ctx, `INSERT INTO chat_email_receipts(email_id,status,status_rank,occurred_at) VALUES($1,'sent',1,NOW()) ON CONFLICT DO NOTHING`, resp.ID); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO chat_message_emails(message_id,email_id,recipient_type) VALUES($1::uuid,$2,'visitor') ON CONFLICT DO NOTHING`, id, resp.ID); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `UPDATE chat_visitor_email_outbox SET state='sent',attempts=attempts+1,last_error='' WHERE message_id=$1::uuid`, id); err != nil {
		return err
	}
	if err = tx.Commit(); err != nil {
		return err
	}
	s.publishInbox(room)
	return nil
}

func (s *Server) sendPendingVisitorEmails(ctx context.Context) error {
	if s.db == nil {
		return nil
	}
	rows, err := s.db.QueryContext(ctx, `SELECT message_id::text FROM chat_visitor_email_outbox WHERE state='pending' AND next_attempt_at<=NOW() ORDER BY next_attempt_at LIMIT 10`)
	if err != nil {
		return err
	}
	var ids []string
	for rows.Next() {
		var id string
		if err = rows.Scan(&id); err != nil {
			rows.Close()
			return err
		}
		ids = append(ids, id)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	var failures int
	for _, id := range ids {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if s.sendVisitorEmail(ctx, id) != nil {
			failures++
		}
	}
	if failures > 0 {
		return fmt.Errorf("%d customer email deliveries remain unconfirmed", failures)
	}
	return nil
}
