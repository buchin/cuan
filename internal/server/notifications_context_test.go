package server

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/buchin/cuan/internal/config"
	"github.com/buchin/cuan/internal/db"
)

func TestNotificationContextAndResolution(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Chdir(t.TempDir())
	s := NewServer(&config.Config{Environment: "testing"}, nil)
	defer s.terminalManager.CloseAll()
	for _, decision := range []string{"approve", "defer"} {
		t.Run(decision, func(t *testing.T) { verifyNotificationResolution(t, s, decision) })
	}
}

func TestNotificationPostgresContextAndResolution(t *testing.T) {
	dsn := os.Getenv("CUAN_NOTIFICATION_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("requires dedicated notification PostgreSQL database")
	}
	u, err := url.Parse(dsn)
	if err != nil || !strings.HasPrefix(u.Path, "/cuan_notification_feed_") {
		t.Fatal("dedicated cuan_notification_feed_* database required")
	}
	database, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	var tables int
	if err = database.QueryRow(`SELECT count(*) FROM information_schema.tables WHERE table_schema='public'`).Scan(&tables); err != nil || tables != 0 {
		t.Fatal("test database must be empty", tables, err)
	}
	if err = db.Run(context.Background(), database, "up"); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", t.TempDir())
	t.Chdir(t.TempDir())
	s := NewServer(&config.Config{Environment: "testing", AuthSecret: strings.Repeat("fixture-", 5)}, database)
	defer s.terminalManager.CloseAll()
	for _, decision := range []string{"approve", "defer"} {
		t.Run(decision, func(t *testing.T) { verifyNotificationResolution(t, s, decision) })
	}
	// More than100 already-recovered requests must not hide reconciliation of
	// a retained old alert. Missing from a recent window is not resolution proof.
	items, err := s.notificationCandidates(context.Background(), "notification-owner")
	if err != nil {
		t.Fatal(err)
	}
	var old ownerNotification
	for _, item := range items {
		if item.Kind == "blocker" && item.Resolved {
			old = item
			break
		}
	}
	if old.ID == "" {
		t.Fatal("missing recovery fixture")
	}
	room := strings.SplitN(old.ID, ":", 3)[1]
	old.Read = true
	old.Resolved = false
	old.Status = "Needs attention"
	prefs := ownerPreferences{Items: []ownerNotification{old}}
	if err = writeOwnerPreferences("notification-owner", prefs); err != nil {
		t.Fatal(err)
	}
	_, err = database.Exec(`INSERT INTO inbox_requests(room_id,request_id,payload_hash,message_id,reply_id,state,created_at,deadline,owner_id,agent_id)
    SELECT $1::uuid,md5('notification-window-'||n||'-'||phase)::uuid,'fixture',md5('notification-message-'||n)::uuid,md5('notification-reply-'||n||'-'||phase)::uuid,
    CASE WHEN phase=0 THEN 'failed' ELSE 'completed' END,NOW()+interval '1 hour'+phase*interval '1 second',NOW()+interval '2 hours','notification-owner','agent_notify'
    FROM generate_series(1,101) n CROSS JOIN generate_series(0,1) phase`, room)
	if err != nil {
		t.Fatal(err)
	}
	items, err = s.notificationCandidates(context.Background(), "notification-owner")
	if err != nil {
		t.Fatal(err)
	}
	mergeNotifications(&prefs, items)
	if len(prefs.Items) != 1 || prefs.Items[0].ID != old.ID || !prefs.Items[0].Read || !prefs.Items[0].Resolved {
		t.Fatal("retained alert lost beyond recent window", prefs.Items)
	}
}

func verifyNotificationResolution(t *testing.T, s *Server, action string) {
	t.Helper()
	ctx := context.Background()
	now := time.Now().UTC()
	owner := "notification-owner"
	room := db.GenerateUUID()
	if err := s.projectStudioRepo.CreateRoom(ctx, &db.ProjectRoom{ID: room, ProjectID: "default", Title: "Website refresh password=hide-this-value", Topic: "agent:agent_notify", CreatedAt: now, UpdatedAt: now}); err != nil {
		t.Fatal(err)
	}
	store := s.projectStudioRepo.(db.InboxRequestRepository)
	accept := func(content string) db.InboxRequest {
		q := db.InboxRequest{OwnerID: owner, AgentID: "agent_notify", RoomID: room, RequestID: db.GenerateUUID(), MessageID: db.GenerateUUID(), ReplyID: db.GenerateUUID(), PayloadHash: db.GenerateUUID(), State: "queued", CreatedAt: time.Now().UTC(), Deadline: now.Add(time.Hour)}
		if _, _, err := store.AcceptInboxRequest(ctx, q, &db.ProjectMessage{ID: q.MessageID, RoomID: room, AuthorType: "user", Content: content, CreatedAt: q.CreatedAt, UpdatedAt: q.CreatedAt}); err != nil {
			t.Fatal(err)
		}
		return q
	}
	proposal := accept("RAW_MESSAGE_SECRET must not become a notification preview")
	proposal.State = "completed"
	if err := store.FinishInboxRequest(ctx, proposal, &db.ProjectMessage{ID: proposal.ReplyID, RoomID: room, AuthorType: "agent", Content: "```cuan-approval\n{\"title\":\"Improve navigation\",\"description\":\"PRIVATE_PLAN_DETAIL\",\"project_id\":\"default\"}\n```", CreatedAt: now, UpdatedAt: now}); err != nil {
		t.Fatal(err)
	}
	proposalID := "approval:" + room + ":" + proposal.ReplyID
	find := func(items []ownerNotification, id string) ownerNotification {
		for _, item := range items {
			if item.ID == id {
				return item
			}
		}
		t.Fatal("missing notification", id)
		return ownerNotification{}
	}
	items, err := s.notificationCandidates(ctx, owner)
	if err != nil {
		t.Fatal(err)
	}
	pending := find(items, proposalID)
	if pending.Resolved || pending.Context == "" || strings.Contains(pending.Context, "hide-this-value") || pending.ActionLabel != "Review proposal" {
		t.Fatal("missing private/contextual proposal", pending)
	}
	raw, _ := json.Marshal(items)
	if strings.Contains(string(raw), "RAW_MESSAGE_SECRET") || strings.Contains(string(raw), "PRIVATE_PLAN_DETAIL") {
		t.Fatal("raw message/plan preview leaked")
	}
	preferences := ownerPreferences{Items: []ownerNotification{pending}}
	preferences.Items[0].Read = true
	metadata, _ := json.Marshal(attachmentMessage{Text: "User decision", Approval: &approvedInboxPlan{Action: action, MessageID: proposal.ReplyID, Plan: inboxApprovalPlan{Title: "Improve navigation", Description: "PRIVATE_PLAN_DETAIL", ProjectID: "default"}}})
	decision := accept(attachmentEnvelope + string(metadata))
	decision.State = "completed"
	if err = store.FinishInboxRequest(ctx, decision, nil); err != nil {
		t.Fatal(err)
	}
	items, err = s.notificationCandidates(ctx, owner)
	if err != nil {
		t.Fatal(err)
	}
	handled := find(items, proposalID)
	status := "Approved"
	if action == "defer" {
		status = "Saved for later"
	}
	if !handled.Resolved || handled.Status != status {
		t.Fatal("decision remained actionable", handled)
	}
	mergeNotifications(&preferences, items)
	if retained := find(preferences.Items, proposalID); !retained.Read || !retained.Resolved {
		t.Fatal("read state lost during metadata refresh", retained)
	}
	failed := accept("another private request")
	failed.State = "failed"
	failed.SafeError = "The chat sandbox is unavailable."
	if err = store.FinishInboxRequest(ctx, failed, nil); err != nil {
		t.Fatal(err)
	}
	items, err = s.notificationCandidates(ctx, owner)
	if err != nil {
		t.Fatal(err)
	}
	failureID := "request:" + room + ":" + failed.RequestID
	if alert := find(items, failureID); alert.Resolved || alert.Context == "" || alert.ActionLabel != "Check reply" {
		t.Fatal("failure context missing", alert)
	}
	mergeNotifications(&preferences, items)
	retry, _, err := s.projectStudioRepo.(db.InboxRetryRepository).RetryInboxRequest(ctx, room, failed.RequestID, owner)
	if err != nil {
		t.Fatal(err)
	}
	items, err = s.notificationCandidates(ctx, owner)
	if err != nil {
		t.Fatal(err)
	}
	if waiting := find(items, failureID); !waiting.Resolved || waiting.Status != "Recovering" {
		t.Fatal("active retry incorrectly requests human attention", waiting)
	}
	if action == "defer" {
		retry.State = "failed"
		retry.SafeError = "The chat sandbox is unavailable."
		if err = store.FinishInboxRequest(ctx, retry, nil); err != nil {
			t.Fatal(err)
		}
		items, err = s.notificationCandidates(ctx, owner)
		if err != nil {
			t.Fatal(err)
		}
		if older := find(items, failureID); !older.Resolved || older.Status == "Recovered" {
			t.Fatal("failed retry was mistaken for success", older)
		}
		retry, _, err = s.projectStudioRepo.(db.InboxRetryRepository).RetryInboxRequest(ctx, room, retry.RequestID, owner)
		if err != nil {
			t.Fatal(err)
		}
	}
	retry.State = "completed"
	if err = store.FinishInboxRequest(ctx, retry, &db.ProjectMessage{ID: retry.ReplyID, RoomID: room, AuthorType: "agent", Content: "Recovered fixture reply", CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC()}); err != nil {
		t.Fatal(err)
	}
	items, err = s.notificationCandidates(ctx, owner)
	if err != nil {
		t.Fatal(err)
	}
	recovered := find(items, failureID)
	if !recovered.Resolved || recovered.Title != "Reply recovered" {
		t.Fatal("successful retry still requires attention", recovered)
	}
	mergeNotifications(&preferences, items)
	if !find(preferences.Items, failureID).Resolved {
		t.Fatal("persisted failure not reconciled")
	}
	empty := defaultOwnerPreferences()
	mergeNotifications(&empty, items)
	for _, item := range empty.Items {
		if item.ID == failureID || item.ID == proposalID {
			t.Fatal("new resolved alerts should not create fresh notification noise")
		}
	}
}

func TestNotificationMergeReconcilesTaskProgress(t *testing.T) {
	now := time.Now().UTC()
	p := ownerPreferences{Items: []ownerNotification{{ID: "task:one:review:attempt", Kind: "approval", Title: "Old generic text", Read: true, At: now}}}
	state := map[string]notificationTaskState{"one": {Status: "doing", Attempt: "next", Title: "Website navigation", Context: "Website", Href: "/tasks/one"}}
	mergeNotifications(&p, nil, state)
	if !p.Items[0].Resolved || !p.Items[0].Read || p.Items[0].Status != "In progress" || p.Items[0].Title != "Website navigation" {
		t.Fatal(p.Items)
	}
	state["one"] = notificationTaskState{Status: "done", Attempt: "next", Title: "Website navigation", Context: "Website", Href: "/tasks/one"}
	mergeNotifications(&p, []ownerNotification{{ID: "task:one:done:next", Kind: "completed", Title: "Website navigation", At: now.Add(time.Second)}}, state)
	if len(p.Items) != 2 || p.Items[0].Read || p.Items[1].Status != "Completed" {
		t.Fatal(p.Items)
	}
}
