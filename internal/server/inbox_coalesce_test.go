package server

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/buchin/cuan/internal/db"
)

func TestInboxRecoveryRequestsQueueReasons(t *testing.T) {
	now := time.Now().UTC()
	roomID := db.GenerateUUID()

	// Scenario 1: Running request is a task review -> Queued request gets "waiting_verification"
	reviewReq := db.InboxRequest{
		RequestID:   "review-req-1",
		RoomID:      roomID,
		State:       "running",
		PayloadHash: "task-review:att_123",
		CreatedAt:   now,
	}
	userReq := db.InboxRequest{
		RequestID: "user-req-1",
		RoomID:    roomID,
		State:     "queued",
		CreatedAt: now.Add(time.Second),
	}
	dtos := inboxRecoveryRequests([]db.InboxRequest{reviewReq, userReq})
	if len(dtos) != 2 {
		t.Fatalf("expected 2 DTOs, got %d", len(dtos))
	}
	if dtos[1].QueueReason != "waiting_verification" {
		t.Errorf("expected queue_reason 'waiting_verification', got %q", dtos[1].QueueReason)
	}

	// Scenario 2: Running request is an ordinary message -> Queued request gets "waiting_previous_message"
	normReq := db.InboxRequest{
		RequestID: "norm-req-1",
		RoomID:    roomID,
		State:     "running",
		CreatedAt: now,
	}
	dtos2 := inboxRecoveryRequests([]db.InboxRequest{normReq, userReq})
	if dtos2[1].QueueReason != "waiting_previous_message" {
		t.Errorf("expected queue_reason 'waiting_previous_message', got %q", dtos2[1].QueueReason)
	}

	// Scenario 3: Multiple queued requests -> second queued gets queue_position:2
	userReq2 := db.InboxRequest{
		RequestID: "user-req-2",
		RoomID:    roomID,
		State:     "queued",
		CreatedAt: now.Add(2 * time.Second),
	}
	dtos3 := inboxRecoveryRequests([]db.InboxRequest{normReq, userReq, userReq2})
	if dtos3[1].QueueReason != "waiting_previous_message" {
		t.Errorf("expected queue_reason 'waiting_previous_message', got %q", dtos3[1].QueueReason)
	}
	if dtos3[2].QueueReason != "queue_position:2" {
		t.Errorf("expected queue_reason 'queue_position:2', got %q", dtos3[2].QueueReason)
	}

	// Scenario 4: Waiting for AI capacity
	capacityReq := db.InboxRequest{
		RequestID: "cap-req-1",
		RoomID:    roomID,
		State:     "queued",
		SafeError: "AI capacity is temporarily full. Please wait.",
		CreatedAt: now,
	}
	dtos4 := inboxRecoveryRequests([]db.InboxRequest{capacityReq})
	if dtos4[0].QueueReason != "waiting_capacity" {
		t.Errorf("expected queue_reason 'waiting_capacity', got %q", dtos4[0].QueueReason)
	}
}

func TestInboxCoalescingQueuedUserMessages(t *testing.T) {
	s, bin, _, _ := inboxFixture(t)
	ctx := context.Background()
	roomID := db.GenerateUUID()
	ownerID := "owner"

	repo := s.projectStudioRepo.(db.InboxRequestRepository)

	activeReqID := db.GenerateUUID()
	activeMsgID := db.GenerateUUID()
	activeReplyID := db.GenerateUUID()

	reqID1 := db.GenerateUUID()
	msgID1 := db.GenerateUUID()
	replyID1 := db.GenerateUUID()

	reqID2 := db.GenerateUUID()
	msgID2 := db.GenerateUUID()
	replyID2 := db.GenerateUUID()

	now := time.Now().UTC()

	// 1. In-flight active turn running in the room
	activeReq := db.InboxRequest{
		RoomID:      roomID,
		OwnerID:     ownerID,
		AgentID:     "agent_lifecycle",
		RequestID:   activeReqID,
		MessageID:   activeMsgID,
		ReplyID:     activeReplyID,
		State:       "running",
		CreatedAt:   now.Add(-time.Minute),
		Deadline:    now.Add(time.Hour),
		PayloadHash: "hash-active",
	}
	_, _, err := repo.AcceptInboxRequest(ctx, activeReq, &db.ProjectMessage{
		ID:         activeMsgID,
		RoomID:     roomID,
		AuthorType: "user",
		AuthorID:   "operator_user",
		Content:    "Pesan aktif yang sedang diproses model",
		CreatedAt:  now.Add(-time.Minute),
	})
	if err != nil {
		t.Fatalf("AcceptInboxRequest activeReq: %v", err)
	}

	// 2. First follow-up user message queued behind in-flight turn
	req1 := db.InboxRequest{
		RoomID:      roomID,
		OwnerID:     ownerID,
		AgentID:     "agent_lifecycle",
		RequestID:   reqID1,
		MessageID:   msgID1,
		ReplyID:     replyID1,
		State:       "queued",
		CreatedAt:   now,
		Deadline:    now.Add(time.Hour),
		PayloadHash: "hash-1",
	}
	_, _, err = repo.AcceptInboxRequest(ctx, req1, &db.ProjectMessage{
		ID:         msgID1,
		RoomID:     roomID,
		AuthorType: "user",
		AuthorID:   "operator_user",
		Content:    "Pesan pertama yang akan disusul pesan kedua",
		CreatedAt:  now,
	})
	if err != nil {
		t.Fatalf("AcceptInboxRequest req1: %v", err)
	}

	// 3. Second follow-up user message queued behind in-flight turn
	req2 := db.InboxRequest{
		RoomID:      roomID,
		OwnerID:     ownerID,
		AgentID:     "agent_lifecycle",
		RequestID:   reqID2,
		MessageID:   msgID2,
		ReplyID:     replyID2,
		State:       "queued",
		CreatedAt:   now.Add(time.Second),
		Deadline:    now.Add(time.Hour),
		PayloadHash: "hash-2",
	}
	_, _, err = repo.AcceptInboxRequest(ctx, req2, &db.ProjectMessage{
		ID:         msgID2,
		RoomID:     roomID,
		AuthorType: "user",
		AuthorID:   "operator_user",
		Content:    "Pesan kedua yang lebih baru dan menggantikan pesan pertama",
		CreatedAt:  now.Add(time.Second),
	})
	if err != nil {
		t.Fatalf("AcceptInboxRequest req2: %v", err)
	}

	// Verify canCoalesceQueuedRequest returns true for req1 (superseded) and false for req2 (newest)
	requests, err := repo.ListInboxRequests(ctx, roomID)
	if err != nil {
		t.Fatalf("ListInboxRequests: %v", err)
	}
	if !s.canCoalesceQueuedRequest(ctx, requests, req1) {
		t.Fatalf("expected req1 to be eligible for coalescing into req2")
	}
	if s.canCoalesceQueuedRequest(ctx, requests, req2) {
		t.Fatalf("expected req2 to NOT be coalesced because it is the latest pending message")
	}

	// Setup fake agy runner so when req2 executes it completes successfully
	os.WriteFile(filepath.Join(bin, "agy"), []byte("#!/bin/sh\nprintf '%s' '{\"event\":\"result\",\"result\":{\"status\":\"SUCCESS\",\"response\":\"{\\\"reply\\\":\\\"Jawaban terpadu untuk kedua pesan.\\\",\\\"topic_title\\\":\\\"\\\",\\\"action\\\":null}\"}}' \n"), 0700)

	// Run drainInboxRoom while activeReq is running so req1 is coalesced into req2
	s.drainInboxRoom(roomID)

	// Now finish active turn and drain again to execute req2
	activeReq.State = "completed"
	_ = repo.FinishInboxRequest(ctx, activeReq, nil)
	s.drainInboxRoom(roomID)

	// Check final states
	allReqs, err := repo.ListInboxRequests(ctx, roomID)
	if err != nil {
		t.Fatalf("ListInboxRequests after drain: %v", err)
	}

	var state1, state2 string
	var req1Steps []db.InboxStep
	for _, r := range allReqs {
		if r.RequestID == reqID1 {
			state1 = r.State
			req1Steps = r.Steps
		} else if r.RequestID == reqID2 {
			state2 = r.State
		}
	}

	if state1 != "completed" {
		t.Errorf("expected req1 to be completed via coalescing, got %s", state1)
	}
	hasCoalescedStep := false
	for _, step := range req1Steps {
		if step.Kind == "coalesced" {
			hasCoalescedStep = true
			if !strings.Contains(step.Text, "Digabungkan") {
				t.Errorf("expected coalesced text, got %s", step.Text)
			}
		}
	}
	if !hasCoalescedStep {
		t.Errorf("expected req1 to have a 'coalesced' step")
	}

	if state2 != "completed" {
		t.Errorf("expected req2 to be completed normally, got %s", state2)
	}
}
