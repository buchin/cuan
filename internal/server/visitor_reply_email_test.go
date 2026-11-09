package server

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/buchin/cuan/internal/db"
	"github.com/buchin/cuan/migrations"
)

func TestVisitorReplyEmailPrivacy(t *testing.T) {
	approval, _ := json.Marshal(attachmentMessage{Text: "Approved", Approval: &approvedInboxPlan{MessageID: db.GenerateUUID(), Plan: inboxApprovalPlan{Title: "Private"}}})
	for _, content := range []string{"@cuan investigate", attachmentEnvelope + string(approval), "", encodeAttachmentMessage("", []string{db.GenerateUUID()})} {
		if _, ok := visitorReplyEmail("visitor@example.com", content, "reply@example.com", "Cuan", "message"); ok {
			t.Fatal("private or empty reply became email", content)
		}
	}
	request, ok := visitorReplyEmail("visitor@example.com", "Halo sob,\nSudah beres.", "reply@example.com", "Cuan", "message")
	if !ok || request.Text != "Halo sob,\nSudah beres." || len(request.To) != 1 || request.To[0] != "visitor@example.com" || request.IdempotencyKey != "cuan-visitor/message" || request.HTML != "" {
		t.Fatal(request, ok)
	}
	for _, email := range []string{"", "Name <visitor@example.com>", "visitor@example.com\r\nBcc: other@example.com"} {
		if _, ok := visitorReplyEmail(email, "Public", "reply@example.com", "Cuan", "message"); ok {
			t.Fatal("invalid connected recipient accepted")
		}
	}
}

func TestVisitorReplyEmailPostgres(t *testing.T) {
	raw := os.Getenv("CUAN_VISITOR_EMAIL_TEST_DATABASE_URL")
	if raw == "" {
		t.Skip("isolated cuan_email_test_* database required")
	}
	u, err := url.Parse(raw)
	if err != nil || !strings.HasPrefix(u.Path, "/cuan_email_test_") {
		t.Fatal("isolated email test database required")
	}
	database, err := sql.Open("pgx", raw)
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	ctx := context.Background()
	_, err = database.ExecContext(ctx, `CREATE TABLE project_messages(id UUID PRIMARY KEY,room_id UUID NOT NULL,author_type TEXT NOT NULL,author_id TEXT NOT NULL,content TEXT NOT NULL)`)
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"000022_contacts_crm.up.sql", "000035_chat_receipts.up.sql"} {
		migration, _ := migrations.FS.ReadFile(name)
		if _, err = database.ExecContext(ctx, string(migration)); err != nil {
			t.Fatal(err)
		}
	}
	room, contact := db.GenerateUUID(), db.GenerateUUID()
	_, err = database.ExecContext(ctx, `INSERT INTO crm_contacts(id,product,guest_uuid,session_uuid,name,email) VALUES($1,'fixture','guest',$2,'Fixture','visitor@example.com')`, contact, room)
	if err != nil {
		t.Fatal(err)
	}
	insert := func(author, content string) string {
		t.Helper()
		id := db.GenerateUUID()
		if _, err := database.ExecContext(ctx, `INSERT INTO project_messages(id,room_id,author_type,author_id,content) VALUES($1,$2,'user',$3,$4)`, id, room, author, content); err != nil {
			t.Fatal(err)
		}
		return id
	}
	insert("operator_user", "Historical public reply")
	migration, _ := migrations.FS.ReadFile("000038_visitor_reply_email.up.sql")
	if _, err = database.ExecContext(ctx, string(migration)); err != nil {
		t.Fatal(err)
	}
	var count int
	if err = database.QueryRowContext(ctx, `SELECT COUNT(*) FROM chat_visitor_email_outbox`).Scan(&count); err != nil || count != 0 {
		t.Fatal("historical messages backfilled", count, err)
	}
	s, _ := setupTestChatServer(t)
	s.cfg.WorkspacesDir = filepath.Join(t.TempDir(), "workspaces")
	s.db = database
	settings := &ResendSettings{Enabled: true, APIKey: "fixture-only", FromEmail: "reply@example.com", FromName: "Cuan"}
	if err = s.SaveResendSettings(ctx, settings); err != nil {
		t.Fatal(err)
	}
	client := &mockResendClient{}
	s.resendClient = client
	state := func(id string) string {
		t.Helper()
		var result string
		if err := database.QueryRowContext(ctx, `SELECT state FROM chat_visitor_email_outbox WHERE message_id=$1`, id).Scan(&result); err != nil {
			t.Fatal(err)
		}
		return result
	}
	due := func(id string) {
		t.Helper()
		if _, err := database.ExecContext(ctx, `UPDATE chat_visitor_email_outbox SET next_attempt_at=NOW()-INTERVAL '1 second' WHERE message_id=$1`, id); err != nil {
			t.Fatal(err)
		}
	}
	public := insert("operator_user", "Halo sob, hasil sudah siap.")
	if err = s.sendPendingVisitorEmails(ctx); err != nil || len(client.getSentEmails()) != 0 {
		t.Fatal("sent before unread delay", err)
	}
	private := insert("operator_user", "@cuan private whisper")
	read := insert("operator_user", "Already read")
	_, err = database.ExecContext(ctx, `INSERT INTO chat_message_receipts(message_id,read_confirmed_at) VALUES($1,NOW())`, read)
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{public, private, read} {
		due(id)
	}
	if err = s.sendPendingVisitorEmails(ctx); err != nil {
		t.Fatal(err)
	}
	if len(client.getSentEmails()) != 1 || state(public) != "sent" || state(private) != "skipped" || state(read) != "skipped" {
		t.Fatal("incorrect public/unread filtering", client.getSentEmails())
	}
	var recipientType, status string
	if err = database.QueryRowContext(ctx, `SELECT e.recipient_type,r.status FROM chat_message_emails e JOIN chat_email_receipts r ON r.email_id=e.email_id WHERE e.message_id=$1`, public).Scan(&recipientType, &status); err != nil || recipientType != "visitor" || status != "sent" {
		t.Fatal("missing visitor receipt", recipientType, status, err)
	}
	// A fresh client instance simulates the next process after deployment.
	restarted := &mockResendClient{}
	s.resendClient = restarted
	if err = s.sendPendingVisitorEmails(ctx); err != nil || state(public) != "sent" || len(restarted.getSentEmails()) != 0 {
		t.Fatal(err)
	}
	client = &mockResendClient{sendErr: errors.New("transport timeout")}
	s.resendClient = client
	retry := insert("operator_user", "Freeze this exact reply")
	due(retry)
	if err = s.sendVisitorEmail(ctx, retry); err == nil {
		t.Fatal("missing provider failure")
	}
	settings.FromEmail = "changed@example.com"
	if err = s.SaveResendSettings(ctx, settings); err != nil {
		t.Fatal(err)
	}
	client.sendErr = nil
	s.resendClient = client
	due(retry)
	if err = s.sendVisitorEmail(ctx, retry); err != nil {
		t.Fatal(err)
	}
	sent := client.getSentEmails()
	if len(sent) != 1 || !strings.Contains(sent[0].From, "reply@example.com") || sent[0].IdempotencyKey != "cuan-visitor/"+retry {
		t.Fatal("retry payload changed", sent)
	}
	expired := insert("operator_user", "Do not replay outside idempotency window")
	due(expired)
	_, err = database.ExecContext(ctx, `UPDATE chat_visitor_email_outbox SET created_at=NOW()-INTERVAL '24 hours' WHERE message_id=$1`, expired)
	if err != nil {
		t.Fatal(err)
	}
	if err = s.sendVisitorEmail(ctx, expired); err != nil || state(expired) != "failed" || len(client.getSentEmails()) != 1 {
		t.Fatal("expired send replayed", err)
	}
	// Concurrent scheduler processes serialize the same message's send.
	concurrent := insert("operator_user", "Send once across processes")
	due(concurrent)
	var workers sync.WaitGroup
	results := make(chan error, 2)
	for range 2 {
		workers.Add(1)
		go func() { defer workers.Done(); results <- s.sendVisitorEmail(ctx, concurrent) }()
	}
	workers.Wait()
	close(results)
	for e := range results {
		if e != nil {
			t.Fatal(e)
		}
	}
	if len(client.getSentEmails()) != 2 {
		t.Fatal("concurrent duplicate send")
	}
	// Provider accepted, but saving its receipt failed: preserve pending state
	// and retry the exact already frozen request, under the same provider key.
	uncertain := insert("operator_user", "Do not duplicate after receipt failure")
	due(uncertain)
	_, err = database.ExecContext(ctx, `CREATE FUNCTION reject_fixture_receipt() RETURNS TRIGGER LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'fixture receipt failure'; END; $$; CREATE TRIGGER reject_fixture_receipt BEFORE INSERT ON chat_message_emails FOR EACH ROW EXECUTE FUNCTION reject_fixture_receipt()`)
	if err != nil {
		t.Fatal(err)
	}
	if err = s.sendVisitorEmail(ctx, uncertain); err == nil || state(uncertain) != "pending" {
		t.Fatal("missing persisted-receipt failure", err)
	}
	if _, err = database.ExecContext(ctx, `DROP TRIGGER reject_fixture_receipt ON chat_message_emails; DROP FUNCTION reject_fixture_receipt()`); err != nil {
		t.Fatal(err)
	}
	if err = s.sendVisitorEmail(ctx, uncertain); err != nil {
		t.Fatal(err)
	}
	requests := client.getSentEmails()
	if len(requests) != 4 {
		t.Fatal("unexpected retry count", len(requests))
	}
	a, _ := json.Marshal(requests[2])
	b, _ := json.Marshal(requests[3])
	if string(a) != string(b) || requests[2].IdempotencyKey != requests[3].IdempotencyKey {
		t.Fatal("uncertain send changed payload or idempotency key")
	}
	changed := insert("operator_user", "Connection-bound retry")
	due(changed)
	if err = s.prepareVisitorEmail(ctx, changed); err != nil {
		t.Fatal(err)
	}
	settings.APIKey = "different-connection"
	if err = s.SaveResendSettings(ctx, settings); err != nil {
		t.Fatal(err)
	}
	s.resendClient = client
	if err = s.sendVisitorEmail(ctx, changed); err != nil || state(changed) != "failed" || len(client.getSentEmails()) != 4 {
		t.Fatal("retried through another connection", err)
	}
	settings.APIKey = "fixture-only"
	if err = s.SaveResendSettings(ctx, settings); err != nil {
		t.Fatal(err)
	}
	s.resendClient = client
	moved := insert("operator_user", "Do not send to a replaced contact email")
	due(moved)
	if _, err = database.ExecContext(ctx, `UPDATE crm_contacts SET email='replacement@example.com' WHERE id=$1`, contact); err != nil {
		t.Fatal(err)
	}
	if err = s.sendVisitorEmail(ctx, moved); err != nil || state(moved) != "skipped" || len(client.getSentEmails()) != 4 {
		t.Fatal("recipient change ignored", err)
	}
}
