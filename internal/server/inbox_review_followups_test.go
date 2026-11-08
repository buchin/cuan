package server

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/buchin/cuan/internal/db"
)

func TestReviewIncludesAuthenticatedOwnerCorrections(t *testing.T) {
	s, origin, d := phaseLineageSetup(t)
	ctx := context.Background()
	store := s.projectStudioRepo.(db.InboxRequestRepository)
	unbound := &db.ProjectMessage{ID: db.GenerateUUID(), RoomID: origin.RoomID, AuthorType: "user", Content: "unbound message must not leak", CreatedAt: time.Now(), UpdatedAt: time.Now()}
	if err := s.projectStudioRepo.CreateMessage(ctx, unbound); err != nil {
		t.Fatal(err)
	}
	for _, row := range []struct{ owner, kind, text string }{
		{origin.OwnerID, "user", "HTTP partial responses still pass: preserve this finding"},
		{"foreign-owner", "user", "foreign correction must not leak"},
		{origin.OwnerID, "agent", "agent claim must not become owner evidence"},
	} {
		q := origin
		q.OwnerID, q.RequestID, q.MessageID = row.owner, db.GenerateUUID(), db.GenerateUUID()
		q.CreatedAt = origin.CreatedAt.Add(time.Second)
		q.State = "queued"
		m := &db.ProjectMessage{ID: q.MessageID, RoomID: q.RoomID, AuthorType: row.kind, Content: row.text, CreatedAt: q.CreatedAt, UpdatedAt: q.CreatedAt}
		if _, _, err := store.AcceptInboxRequest(ctx, q, m); err != nil {
			t.Fatal(err)
		}
	}
	raw := s.inboxReviewData(ctx, d)
	var payload map[string]json.RawMessage
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		t.Fatal(err)
	}
	followups := string(payload["owner_followups"])
	if !strings.Contains(followups, "HTTP partial responses still pass") {
		t.Fatal("review omitted authenticated owner correction", followups)
	}
	if strings.Contains(followups, "foreign correction") || strings.Contains(followups, "agent claim") || strings.Contains(followups, "unbound message") {
		t.Fatal("review included unbound or non-owner correction", followups)
	}
	if !strings.Contains(followups, "not execution authority") {
		t.Fatal("correction context did not retain its evidence-only boundary")
	}
}

func TestReviewOwnerCorrectionsRemainBounded(t *testing.T) {
	s, origin, d := phaseLineageSetup(t)
	ctx := context.Background()
	store := s.projectStudioRepo.(db.InboxRequestRepository)
	for i := 0; i < 8; i++ {
		q := origin
		q.RequestID, q.MessageID = db.GenerateUUID(), db.GenerateUUID()
		q.CreatedAt = origin.CreatedAt.Add(time.Duration(i+1) * time.Second)
		m := &db.ProjectMessage{ID: q.MessageID, RoomID: q.RoomID, AuthorType: "user", Content: strings.Repeat("evidence ", 1000), CreatedAt: q.CreatedAt, UpdatedAt: q.CreatedAt}
		if _, _, err := store.AcceptInboxRequest(ctx, q, m); err != nil {
			t.Fatal(err)
		}
	}
	rows := s.inboxReviewOwnerFollowups(ctx, d)
	if len(rows) == 0 || len(rows) > 4 {
		t.Fatal("unbounded follow-up count", len(rows))
	}
	textBytes := 0
	for _, row := range rows {
		textBytes += len(row["text"])
	}
	if textBytes > 6100 {
		t.Fatal("unbounded owner evidence text", textBytes)
	}
	d.SourceMessageID = "unbound-original"
	if rows := s.inboxReviewOwnerFollowups(ctx, d); len(rows) != 0 {
		t.Fatal("missing original authority exposed follow-ups")
	}
}
