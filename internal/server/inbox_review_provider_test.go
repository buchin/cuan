package server

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/buchin/cuan/internal/agent"
	"github.com/buchin/cuan/internal/db"
	"github.com/buchin/cuan/internal/task"
)

func TestReviewProviderTimeoutFailsOverOnlyWithinAllowedToolFreeRoutes(t *testing.T) {
	for _, scenario := range []string{"allowed", "disabled", "ordinary chat", "unsafe tool", "unknown stream", "cancelled", "remaining budget"} {
		t.Run(scenario, func(t *testing.T) {
			s, _, registered, p, b, home, backupHome := setupFailoverFixture(t)
			if scenario != "disabled" {
				agent.SetProviderAccountFailover(p.OwnerID, b.ID, true)
			}
			script := `#!/bin/sh
printf x >> "$HOME/calls"
if [ -f "$HOME/timeout" ];then
`
			if scenario == "unsafe tool" {
				script += `printf '%s\n' '{"event":"step_update","step_update":{"step_index":2,"state":"DONE","step_type":"tool","tool_name":"run_command"}}'
`
			}
			if scenario == "unknown stream" {
				script += "printf '%s\\n' 'invalid-native-event'\n"
			}
			script += `printf '%s\n' '{"event":"result","result":{"status":"TIMEOUT","response":""}}'
exit 3
fi
printf '%s\n' '{"event":"result","result":{"status":"SUCCESS","response":"{\"decision\":\"revise\",\"explanation\":\"Inspect saved result\",\"revision\":\"Check exact release\",\"manager_action\":\"replan\",\"manager_reason\":\"Use existing result\"}"}}'
`
			bin := strings.Split(os.Getenv("PATH"), ":")[0]
			os.WriteFile(filepath.Join(bin, "agy"), []byte(script), 0700)
			os.WriteFile(filepath.Join(home, "timeout"), nil, 0600)
			ctx := context.Background()
			if scenario != "ordinary chat" {
				ctx = context.WithValue(ctx, inboxTaskReviewKey{}, true)
			}
			if scenario == "remaining budget" {
				bounded, cancel := context.WithTimeout(ctx, inboxDirectReplyBudget()+9*time.Second)
				defer cancel()
				ctx = bounded
			}
			if scenario == "cancelled" {
				cancelled, cancel := context.WithCancel(ctx)
				cancel()
				ctx = cancelled
			}
			out, failure := s.executeInboxWithFailover(ctx, p.OwnerID, registered, "Review current result", home, "", false, nil)
			_, backupErr := os.Stat(filepath.Join(backupHome, "calls"))
			if scenario == "allowed" {
				if failure != "" || out == "" || backupErr != nil {
					t.Fatal("safe review did not fail over", failure)
				}
			} else {
				if out != "" || failure == "" || backupErr == nil {
					t.Fatal("unapproved/unsafe replay", scenario, failure)
				}
				if (scenario == "disabled" || scenario == "remaining budget") && failure != inboxReviewProviderWaiting {
					t.Fatal("safe review not left waiting", failure)
				}
				if (scenario == "unsafe tool" || scenario == "unknown stream" || scenario == "cancelled" || scenario == "ordinary chat") && failure == inboxReviewProviderWaiting {
					t.Fatal("unsafe failure marked retryable")
				}
			}
		})
	}
}

func TestProviderTimeoutDoesNotBecomeCompletedTaskAssessment(t *testing.T) {
	s, d, old := stalledReviewFixture(t, "completed")
	s.inboxStopping.Store(false)
	bin := strings.Split(os.Getenv("PATH"), ":")[0]
	script := `#!/bin/sh
printf '%s\n' '{"event":"result","result":{"status":"TIMEOUT","response":""}}'
exit 3
`
	os.WriteFile(filepath.Join(bin, "agy"), []byte(script), 0700)
	repo := s.projectStudioRepo.(db.InboxLifecycleRepository)
	ctx := context.Background()
	key := "task-review:" + d.LatestAttempt.ID + ":provider-assessment:" + inboxReviewEvidenceDigest(d)
	now := time.Now()
	q := db.InboxRequest{OwnerID: old.OwnerID, AgentID: old.AgentID, RoomID: old.RoomID, RequestID: delegationMessageID(key), MessageID: delegationMessageID(key + ":input"), ReplyID: delegationMessageID(key + ":reply"), State: "queued", CreatedAt: now, Deadline: now.Add(time.Hour)}
	repo.AcceptInboxRequest(ctx, q, &db.ProjectMessage{ID: q.MessageID, RoomID: q.RoomID, AuthorType: "system", AuthorID: "cuan_work_review", Content: "Review [/tasks/" + d.ID + "]\n" + inboxReviewProviderPolicy})
	q, claimed, err := repo.ClaimInboxRequest(ctx, q.RoomID, q.RequestID)
	if err != nil || !claimed {
		t.Fatal(err)
	}
	s.runInboxRequest(repo, q)
	rows, _ := repo.ListInboxRequests(ctx, q.RoomID)
	for _, row := range rows {
		if row.RequestID == q.RequestID {
			if row.State != "queued" || row.SafeError != inboxReviewProviderWaiting || !inboxCapacityWaiting(row, time.Now()) {
				t.Fatal("timeout converted to completed review", row.State, row.SafeError)
			}
		}
	}
	after, _ := s.taskSvc.GetTaskDetail(ctx, d.ID)
	if _, saved := reviewAssessment(after, q.RequestID); saved || after.Status != d.Status || len(after.Attempts) != len(d.Attempts) {
		t.Fatal("inference failure changed task acceptance or execution")
	}
	messages, _ := s.projectStudioRepo.ListMessagesByRoomID(ctx, q.RoomID)
	for _, m := range messages {
		if m.ID == q.ReplyID {
			t.Fatal("failure fabricated a review reply")
		}
	}
	s.inboxStopping.Store(true)
}

func TestLegacyProviderFallbackReassessmentIsBoundAndIdempotent(t *testing.T) {
	s, d, q := stalledReviewFixture(t, "completed")
	ctx := context.Background()
	repo := s.projectStudioRepo.(*db.MemoryProjectStudioRepository)
	// Seed only recorded stage metadata for the historical provider failure.
	q.Steps = []db.InboxStep{{Seq: 1, Kind: "provider", Text: "Provider running", At: time.Now()}, {Seq: 2, Kind: "result_validation", Text: "The provider timed out.", At: time.Now()}}
	// The fixture's original terminal record remains immutable. Create this
	// source shape using a fresh repository then reuse the saved source lineage.
	requests, _ := repo.ListInboxRequests(ctx, q.RoomID)
	replacement := db.NewMemoryProjectStudioRepository()
	for _, r := range requests {
		m, _ := repo.GetMessageByID(ctx, r.MessageID)
		if r.RequestID == q.RequestID {
			r.Steps = q.Steps
		}
		replacement.AcceptInboxRequest(ctx, r, m)
	}
	// Room and messages are required by the ordinary recovery resolver.
	room, _ := repo.GetRoomByID(ctx, q.RoomID)
	replacement.CreateRoom(ctx, room)
	s.projectStudioRepo = replacement
	s.queueInboxTaskReview(ctx, d)
	s.queueInboxTaskReview(ctx, d)
	rows := reviewRequests(t, s, q.RoomID)
	if len(rows) != 2 {
		t.Fatal("duplicate or missing reassessment", len(rows))
	}
	var repaired db.InboxRequest
	for _, r := range rows {
		if r.RequestID != q.RequestID {
			repaired = r
		}
	}
	if repaired.State != "queued" || s.inboxReviewedTask(ctx, repaired) == nil {
		raw, _ := json.Marshal(repaired)
		t.Fatal("unbound recovery", string(raw))
	}
	after, _ := s.taskSvc.GetTaskDetail(ctx, d.ID)
	if len(after.Attempts) != len(d.Attempts) || after.Status == task.TaskStatusDone {
		t.Fatal("review retry executed or accepted task")
	}
}

func TestReviewProviderRetryRequiresNativeProofAndCurrentToolFreeMode(t *testing.T) {
	review := context.WithValue(context.Background(), inboxTaskReviewKey{}, true)
	for _, item := range []struct {
		ctx         context.Context
		failure     string
		safe        bool
		attachments int
		want        bool
	}{
		{review, inboxReviewProviderWaiting, true, 0, true},
		{review, "The provider timed out.", true, 0, false},
		{context.Background(), inboxReviewProviderWaiting, true, 0, false},
		{review, inboxReviewProviderWaiting, false, 0, false},
		{review, inboxReviewProviderWaiting, true, 1, false},
	} {
		if retryableInboxReviewProvider(item.ctx, item.failure, item.safe, item.attachments) != item.want {
			t.Fatal("review retry proof changed")
		}
	}
	q := db.InboxRequest{State: "completed", Steps: []db.InboxStep{{Kind: "result_validation", Text: "The provider timed out."}, {Kind: "result_validation", Text: "Provider reply validated."}}}
	if legacyReviewProviderFailed(q) {
		t.Fatal("successful later provider result treated as failed")
	}
}
