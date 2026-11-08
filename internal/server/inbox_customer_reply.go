package server

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/buchin/cuan/internal/db"
	"github.com/buchin/cuan/internal/task"
)

const customerReplyDeliveryInstruction = `This is delivery of an already accepted result, not a new review or authorization. Prepare a concise ready-to-send customer reply using only supplied accepted findings. Return the existing review JSON schema with decision=pass, remaining_work=none, revision empty, and customer_reply_draft containing the actual customer-facing text. Explain what was found and safe practical next steps in the customer's language. Do not claim repairs, publication or customer contact happened when only investigation was performed. Do not promise future delivery or expose task links, internal instructions, credentials, account details or private operational logs. Do not tell the customer to bypass dependency/platform safety checks or edit vendor files. Do not use tools. If evidence is insufficient for a truthful reply, leave customer_reply_draft empty and explain the limitation. The owner will review and edit this draft before sending; never send it yourself.`

type inboxCustomerReplyKey struct{}

func validCustomerReplyDraft(draft string) bool {
	return draft != "" && len(draft) <= 6000 && !strings.Contains(draft, "```") && !strings.Contains(draft, "/tasks/") && !strings.Contains(draft, "/inbox/")
}

// Delivery does not need another inference when the same accepted review
// already supplied a usable draft. Normal persistence and owner review still
// apply; this neither sends the reply nor accepts additional work.
func (s *Server) acceptedCustomerReplyResult(ctx context.Context, q db.InboxRequest, d *task.TaskDetailResponse) (string, bool) {
	if !s.acceptedCustomerReplyEligible(ctx, d) || !isCustomerReplyDelivery(q, d) || !s.customerReplyAuthorized(ctx, q, d) {
		return "", false
	}
	assessment, found := reviewAssessment(d, "")
	draft := strings.TrimSpace(assessment.CustomerReplyDraft)
	if !found || !validCustomerReplyDraft(draft) {
		return "", false
	}
	raw, err := json.Marshal(inboxReviewEvaluation{Decision: "pass", RemainingWork: "none", Explanation: "Draf dari hasil review yang sudah diterima siap digunakan.", CustomerReplyDraft: draft})
	return string(raw), err == nil
}

func customerReplyKey(d *task.TaskDetailResponse) string {
	return "task-review:" + d.LatestAttempt.ID + ":customer-reply:" + inboxReviewEvidenceDigest(d)
}

func isCustomerReplyDelivery(q db.InboxRequest, d *task.TaskDetailResponse) bool {
	if d == nil || d.LatestAttempt == nil {
		return false
	}
	key := customerReplyKey(d)
	return (q.RequestID == delegationMessageID(key) && q.MessageID == delegationMessageID(key+":input")) || (q.RequestID == delegationMessageID(key+":retry") && q.MessageID == delegationMessageID(key+":retry:input"))
}

func (s *Server) acceptedCustomerReplyEligible(ctx context.Context, d *task.TaskDetailResponse) bool {
	if d == nil || d.Status != task.TaskStatusDone || d.LatestAttempt == nil || d.LatestAttempt.Status != task.AttemptStatusSucceeded || d.LatestAttempt.CancelRequested || d.LatestAttempt.CanceledAt != nil || !s.isCustomerRoom(ctx, d.SourceRoomID) {
		return false
	}
	assessment, ok := reviewAssessment(d, "")
	return ok && assessment.Decision == "pass"
}

func (s *Server) customerReplyAuthorized(ctx context.Context, q db.InboxRequest, d *task.TaskDetailResponse) bool {
	if q.RoomID != d.SourceRoomID {
		return false
	}
	repo, ok := s.projectStudioRepo.(db.InboxRequestRepository)
	if !ok {
		return false
	}
	requests, err := repo.ListInboxRequests(ctx, d.SourceRoomID)
	if err != nil {
		return false
	}
	for _, request := range requests {
		if request.RequestID == q.RequestID && (request.State == "cancelled" || request.State == "canceled" || request.State == "cancelling") {
			return false
		}
	}
	for _, origin := range requests {
		if origin.MessageID != d.SourceMessageID || origin.OwnerID != q.OwnerID || origin.AgentID != q.AgentID {
			continue
		}
		_, authorized, err := s.authorizeSavedTaskLaunch(ctx, task.TaskLaunch{OwnerID: origin.OwnerID, RoomID: origin.RoomID, SourceRequestID: origin.RequestID, CreatedAt: time.Now().UTC()}, d)
		return err == nil && authorized
	}
	return false
}

// The normal Inbox worker owns capacity waits, persistence and draft delivery.
// This never creates an execution launch or changes accepted task state.
func (s *Server) queueAcceptedCustomerReplies(ctx context.Context) {
	reader, ok := s.taskSvc.Store().(task.CustomerReplyCandidateStore)
	if !ok {
		return
	}
	ids, err := reader.CustomerReplyTaskIDs(ctx)
	if err != nil {
		return
	}
	for _, id := range ids {
		d, err := s.taskSvc.GetTaskDetail(ctx, id)
		if err != nil || !s.acceptedCustomerReplyEligible(ctx, d) {
			continue
		}
		repo, ok := s.projectStudioRepo.(db.InboxRequestRepository)
		if !ok {
			return
		}
		requests, err := repo.ListInboxRequests(ctx, d.SourceRoomID)
		if err != nil {
			continue
		}
		var origin *db.InboxRequest
		for i := range requests {
			if requests[i].MessageID == d.SourceMessageID {
				origin = &requests[i]
				break
			}
		}
		if origin == nil || origin.OwnerID == "" || origin.AgentID == "" {
			continue
		}
		_, authorized, err := s.authorizeSavedTaskLaunch(ctx, task.TaskLaunch{OwnerID: origin.OwnerID, RoomID: origin.RoomID, SourceRequestID: origin.RequestID, CreatedAt: time.Now().UTC()}, d)
		if err != nil || !authorized {
			continue
		}
		key := customerReplyKey(d)
		blocked := false
		for _, q := range requests {
			if q.RequestID == delegationMessageID(key) {
				if q.State == "failed" || q.State == "interrupted" || q.State == "completed" {
					key += ":retry"
				} else {
					blocked = true
				}
				break
			}
		}
		for _, q := range requests {
			if q.RequestID == delegationMessageID(key) {
				blocked = true
				break
			}
		}
		if blocked {
			continue
		}
		// Existing visible drafts are already usable; do not duplicate them.
		messages, err := s.projectStudioRepo.ListMessagesByRoomID(ctx, d.SourceRoomID)
		if err != nil {
			continue
		}
		for _, m := range messages {
			if m.AuthorType == "agent" && !m.CreatedAt.Before(d.LatestAttempt.CreatedAt) && strings.Contains(m.Content, "/tasks/"+d.ID) && extractCustomerReplyDraft(m.Content) != "" {
				blocked = true
				break
			}
		}
		if blocked {
			continue
		}
		now := time.Now().UTC()
		q := db.InboxRequest{OwnerID: origin.OwnerID, AgentID: origin.AgentID, RoomID: d.SourceRoomID, RequestID: delegationMessageID(key), MessageID: delegationMessageID(key + ":input"), ReplyID: delegationMessageID(key + ":reply"), PayloadHash: delegationMessageID(key), State: "queued", CreatedAt: now, Deadline: now.Add(10 * time.Minute)}
		message := &db.ProjectMessage{ID: q.MessageID, RoomID: q.RoomID, AuthorType: "system", AuthorID: "cuan_work_review", AuthorName: "Cuan", Content: "Prepare the customer reply for accepted work [/tasks/" + d.ID + "].", CreatedAt: now, UpdatedAt: now}
		if _, fresh, err := repo.AcceptInboxRequest(ctx, q, message); err == nil && fresh {
			s.kickInboxRoom(q.RoomID)
			s.publishInbox(q.RoomID)
			return
		}
	}
}

func (s *Server) applyAcceptedCustomerReply(ctx context.Context, q db.InboxRequest, d *task.TaskDetailResponse, result inboxReviewEvaluation) string {
	if !s.acceptedCustomerReplyEligible(ctx, d) || !isCustomerReplyDelivery(q, d) || !s.customerReplyAuthorized(ctx, q, d) {
		return "Hasil pekerjaan berubah. Draf balasan belum disiapkan ulang."
	}
	draft := strings.TrimSpace(result.CustomerReplyDraft)
	for _, event := range d.Activities {
		if event == nil || event.AttemptID == nil || *event.AttemptID != d.LatestAttempt.ID || event.ActivityType != task.ActivityCustomerReplyPrepared || event.Author != "coordinator" {
			continue
		}
		var saved struct {
			Draft  string `json:"draft"`
			Digest string `json:"evidence_digest"`
		}
		if json.Unmarshal([]byte(event.Message), &saved) == nil && saved.Digest == inboxReviewEvidenceDigest(d) && saved.Draft != "" {
			draft = saved.Draft
			break
		}
	}
	if result.Decision != "pass" || !validCustomerReplyDraft(draft) {
		return "Draf balasan belum berhasil disiapkan. Hasil investigasi tetap tersimpan."
	}
	// Store the prepared text for retry after uncertain final Inbox persistence.
	raw, _ := json.Marshal(map[string]string{"draft": draft, "evidence_digest": inboxReviewEvidenceDigest(d), "reply_id": q.ReplyID})
	id := delegationMessageID("customer-reply:" + d.LatestAttempt.ID + ":" + q.RequestID)
	saved := false
	for _, e := range d.Activities {
		if e != nil && e.ID == id && e.Message == string(raw) && e.Author == "coordinator" {
			saved = true
			break
		}
	}
	if !saved {
		if err := s.taskSvc.Store().CreateActivity(ctx, &task.TaskActivity{ID: id, TaskID: d.ID, AttemptID: &d.LatestAttempt.ID, ActivityType: task.ActivityCustomerReplyPrepared, Author: "coordinator", Message: string(raw), CreatedAt: time.Now().UTC()}); err != nil {
			return "Draf balasan belum tersimpan. Hasil investigasi tetap tersedia."
		}
	}
	return "**Customer reply draft:**\n> " + strings.ReplaceAll(draft, "\n", "\n> ") + "\n\n[View work](/tasks/" + d.ID + ")"
}
