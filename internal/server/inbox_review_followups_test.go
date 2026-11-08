package server

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
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

func TestReviewDoesNotApplyInferencePredatingOwnerMessage(t *testing.T) {
	s, d, q := stalledReviewFixture(t, "running")
	ctx := context.Background()
	started := time.Now().Add(-time.Second)
	q.StartedAt = &started
	owner := q
	owner.RequestID, owner.MessageID = db.GenerateUUID(), db.GenerateUUID()
	owner.CreatedAt = time.Now()
	owner.State = "queued"
	m := &db.ProjectMessage{ID: owner.MessageID, RoomID: owner.RoomID, AuthorType: "user", Content: "The reported result is still wrong; preserve original scope.", CreatedAt: owner.CreatedAt, UpdatedAt: owner.CreatedAt}
	if _, _, err := s.projectStudioRepo.(db.InboxRequestRepository).AcceptInboxRequest(ctx, owner, m); err != nil {
		t.Fatal(err)
	}
	s.applyInboxReviewEvaluation(ctx, q, `{"decision":"pass","explanation":"All work completed."}`)
	after, err := s.taskSvc.GetTaskDetail(ctx, d.ID)
	if err != nil || after.Status != d.Status || len(after.Attempts) != len(d.Attempts) {
		t.Fatal("stale inference changed task", err)
	}
	for _, event := range after.Activities {
		if event.ActivityType == reviewAssessmentActivity {
			t.Fatal("stale inference saved a review assessment")
		}
	}
}

func TestChangedOwnerContextRequeuesReviewWithoutRepeatingWork(t *testing.T) {
	s, d, q := stalledReviewFixture(t, "queued")
	s.inboxStopping.Store(false)
	ctx := context.Background()
	repo := s.projectStudioRepo.(db.InboxLifecycleRepository)
	claimed, ok, err := repo.ClaimInboxRequest(ctx, q.RoomID, q.RequestID)
	if err != nil || !ok {
		t.Fatal("claim review", err)
	}
	owner := q
	owner.RequestID, owner.MessageID = db.GenerateUUID(), db.GenerateUUID()
	owner.CreatedAt, owner.State = time.Now(), "queued"
	m := &db.ProjectMessage{ID: owner.MessageID, RoomID: owner.RoomID, AuthorType: "user", Content: "The result still has duplicate links.", CreatedAt: owner.CreatedAt, UpdatedAt: owner.CreatedAt}
	if _, _, err := repo.AcceptInboxRequest(ctx, owner, m); err != nil {
		t.Fatal(err)
	}
	response, _ := json.Marshal(map[string]any{"event": "result", "result": map[string]string{"status": "SUCCESS", "response": `{"decision":"pass","explanation":"All work completed."}`}})
	binary := filepath.Join(strings.Split(os.Getenv("PATH"), ":")[0], "agy")
	if err := os.WriteFile(binary, []byte("#!/bin/sh\nprintf '%s\\n' '"+string(response)+"'\n"), 0700); err != nil {
		t.Fatal(err)
	}
	s.runInboxRequest(repo, claimed)
	requests, err := repo.ListInboxRequests(ctx, q.RoomID)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, saved := range requests {
		if saved.RequestID == q.RequestID {
			found = true
			if saved.State != "queued" || saved.SafeError != inboxReviewRefreshWaiting || !inboxCapacityWaiting(saved, time.Now()) {
				t.Fatal("review was not durably deferred with backoff", saved)
			}
		}
	}
	if !found {
		t.Fatal("original review lost")
	}
	if reply, _ := s.projectStudioRepo.GetMessageByID(ctx, q.ReplyID); reply != nil {
		t.Fatal("stale review reply persisted", reply.Content)
	}
	after, err := s.taskSvc.GetTaskDetail(ctx, d.ID)
	if err != nil || after.Status != d.Status || len(after.Attempts) != len(d.Attempts) {
		t.Fatal("review refresh repeated task execution", err)
	}
	for _, event := range after.Activities {
		if event.ActivityType == reviewAssessmentActivity {
			t.Fatal("stale review assessment persisted")
		}
	}
	fresh, ok, err := repo.ClaimInboxRequest(ctx, q.RoomID, q.RequestID)
	if err != nil || !ok {
		t.Fatal("review could not be reclaimed", err)
	}
	if changed, err := s.inboxReviewOwnerContextChanged(ctx, fresh); err != nil || changed {
		t.Fatal("fresh review loops on the same owner message", changed, err)
	}
	prompt, review := s.inboxTaskReviewPrompt(ctx, fresh)
	if !review || !strings.Contains(prompt, m.Content) {
		t.Fatal("fresh review did not receive owner correction")
	}
}

func TestReviewRefreshIgnoresUnrelatedAndAlreadyKnownMessages(t *testing.T) {
	for _, scenario := range []string{"foreign owner", "agent", "already known", "cancelled"} {
		t.Run(scenario, func(t *testing.T) {
			s, _, q := stalledReviewFixture(t, "running")
			ctx := context.Background()
			started := time.Now().Add(-time.Minute)
			q.StartedAt = &started
			later := q
			later.RequestID, later.MessageID = db.GenerateUUID(), db.GenerateUUID()
			later.CreatedAt, later.State = time.Now(), "queued"
			kind := "user"
			switch scenario {
			case "foreign owner":
				later.OwnerID = "someone-else"
			case "agent":
				kind = "agent"
			case "already known":
				later.CreatedAt = started.Add(-time.Second)
			case "cancelled":
				later.State = "cancelled"
			}
			m := &db.ProjectMessage{ID: later.MessageID, RoomID: later.RoomID, AuthorType: kind, Content: "context", CreatedAt: later.CreatedAt, UpdatedAt: later.CreatedAt}
			if _, _, err := s.projectStudioRepo.(db.InboxRequestRepository).AcceptInboxRequest(ctx, later, m); err != nil {
				t.Fatal(err)
			}
			if changed, err := s.inboxReviewOwnerContextChanged(ctx, q); err != nil || changed {
				t.Fatal("unrelated message invalidated review", changed, err)
			}
		})
	}
}
