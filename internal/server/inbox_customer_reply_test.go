package server

import (
	"context"
	"database/sql"
	"strings"
	"testing"
	"time"

	"github.com/buchin/cuan/internal/db"
	"github.com/buchin/cuan/internal/task"
)

func TestAcceptedCustomerReplyDelivery(t *testing.T) { customerReplyDeliveryContract(t, nil) }
func TestAcceptedCustomerReplyDeliveryPostgres(t *testing.T) {
	withPhasePostgres(t, func(database *sql.DB) { customerReplyDeliveryContract(t, database) })
}

func customerReplyDeliveryContract(t *testing.T, database *sql.DB) {
	t.Helper()
	ctx := context.Background()
	var s *Server
	var origin db.InboxRequest
	var d *task.TaskDetailResponse
	if database == nil {
		s, origin, d = phaseLineageSetup(t)
	} else {
		s, origin, d = phaseLineageSetup(t, database)
	}
	s.inboxStopping.Store(true)
	room, err := s.projectStudioRepo.GetRoomByID(ctx, origin.RoomID)
	if err != nil {
		t.Fatal(err)
	}
	room.Topic = "customer:fixture"
	if err := s.projectStudioRepo.UpdateRoom(ctx, room); err != nil {
		t.Fatal(err)
	}
	d.Instructions = "Cuan managed delegation v1.\nInvestigate the customer's CLI failure and explain safe next steps; no repairs performed."
	d.Summary = "The package requires a supported PHP runtime; no production repair was published."
	d.CompleteNote = "Inspecting the release confirmed a runtime mismatch. Recommend the supported runtime; do not bypass vendor checks."
	if err := s.taskSvc.Store().UpdateTask(ctx, &d.Task); err != nil {
		t.Fatal(err)
	}
	if _, err := s.saveReviewAssessment(ctx, d, origin, inboxReviewEvaluation{Decision: "pass", Finding: "Investigation verified", Explanation: "Investigation verified", RemainingWork: "none"}); err != nil {
		t.Fatal(err)
	}
	before := len(d.Attempts)
	s.queueAcceptedCustomerReplies(ctx)
	s.queueAcceptedCustomerReplies(ctx)
	requests := reviewRequests(t, s, origin.RoomID)
	var queued *db.InboxRequest
	count := 0
	for i := range requests {
		if isCustomerReplyDelivery(requests[i], d) {
			count++
			queued = &requests[i]
		}
	}
	if count != 1 || queued == nil || queued.State != "queued" {
		t.Fatal("missing or duplicated customer draft delivery", count)
	}
	prompt, ok := s.inboxTaskReviewPrompt(ctx, *queued)
	if !ok || !strings.Contains(prompt, customerReplyDeliveryInstruction) || !strings.Contains(prompt, "no production repair") {
		t.Fatal("delivery lost accepted findings or tool-free boundary")
	}
	raw := `{"decision":"pass","finding":"Investigation verified","explanation":"Draft ready","remaining_work":"none","revision":"","customer_reply_draft":"Halo, kendala yang dilaporkan terkait versi runtime. Gunakan versi PHP yang didukung paket. Investigasi sudah selesai; perbaikan paket belum dirilis."}`
	foreign := *queued
	foreign.OwnerID = "other-owner"
	if _, draft := splitCustomerReplyDraft(s.applyInboxReviewEvaluation(ctx, foreign, raw)); draft != "" {
		t.Fatal("foreign owner received a customer draft")
	}
	out := s.applyInboxReviewEvaluation(ctx, *queued, raw)
	_, draft := splitCustomerReplyDraft(out)
	if !strings.Contains(draft, "perbaikan paket belum dirilis") || strings.Contains(draft, "/tasks/") {
		t.Fatal("missing usable draft or leaked internal link", out)
	}
	if database != nil {
		ids, err := s.taskSvc.Store().(task.CustomerReplyCandidateStore).CustomerReplyTaskIDs(ctx)
		if err != nil {
			t.Fatal(err)
		}
		found := false
		for _, id := range ids {
			if id == d.ID {
				found = true
			}
		}
		if !found {
			t.Fatal("prepared draft incorrectly treated as persisted Inbox delivery")
		}
	}
	if again := s.applyInboxReviewEvaluation(ctx, *queued, strings.ReplaceAll(raw, "Halo, kendala", "DIFFERENT, kendala")); again != out {
		t.Fatal("uncertain persistence regenerated prepared text", again)
	}
	queued.State = "completed"
	if err := s.projectStudioRepo.(db.InboxRequestRepository).FinishInboxRequest(ctx, *queued, &db.ProjectMessage{ID: queued.ReplyID, RoomID: queued.RoomID, AuthorType: "agent", AuthorID: origin.AgentID, AuthorName: "Cuan", Content: out, CreatedAt: time.Now().UTC()}); err != nil {
		t.Fatal(err)
	}
	formatted := s.getFormattedMessages(ctx, origin.RoomID, "")
	found := false
	for _, m := range formatted {
		if m.ReplyDraft == draft {
			found = true
			if !m.IsInternal || strings.Contains(m.Content, "Customer reply draft") {
				t.Fatal("draft not separated from internal report")
			}
		}
	}
	if !found {
		t.Fatal("ready-to-use draft missing from owner API")
	}
	after, err := s.taskSvc.GetTaskDetail(ctx, d.ID)
	if err != nil || after.Status != task.TaskStatusDone || len(after.Attempts) != before || after.Summary != d.Summary || after.CompleteNote != d.CompleteNote {
		t.Fatal("reply delivery altered accepted work", err)
	}
	s.queueAcceptedCustomerReplies(ctx)
	if len(reviewRequests(t, s, origin.RoomID)) != len(requests) {
		t.Fatal("prepared draft queued twice")
	}
	after.Status = "cancelled"
	if err := s.taskSvc.Store().UpdateTask(ctx, &after.Task); err != nil {
		t.Fatal(err)
	}
	if _, draft := splitCustomerReplyDraft(s.applyInboxReviewEvaluation(ctx, *queued, raw)); draft != "" {
		t.Fatal("canceled source authorization produced a draft")
	}
}
