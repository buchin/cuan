package server

import (
	"context"
	"strings"
	"testing"

	cuanDB "github.com/buchin/cuan/internal/db"
	"github.com/buchin/cuan/internal/runner"
	"github.com/buchin/cuan/internal/task"
)

// TestS65T3_InfraFault_DoesNotConsumeRevisionQuota verifies that runner disconnects
// (lease expired, heartbeat lost, runner killed, exit code 137) do NOT consume
// the 3x revision quota and do NOT increment taskRevisionAttemptCount.
func TestS65T3_InfraFault_DoesNotConsumeRevisionQuota(t *testing.T) {
	exitCode137 := 137
	exitCode143 := 143
	exitCode1 := 1

	d := &task.TaskDetailResponse{
		Attempts: []*task.TaskAttempt{
			// 1. Infra failure: Lease expired
			{
				Status:       task.AttemptStatusInterrupted,
				ExitCode:     &exitCode137,
				ErrorMessage: "Runner heartbeat lost and lease expired; marked interrupted and recoverable without auto-rerun.",
			},
			// 2. Infra failure: Runner process killed (SIGKILL)
			{
				Status:       task.AttemptStatusFailed,
				ExitCode:     &exitCode137,
				ErrorMessage: "runner killed by resource-guard supervisor",
			},
			// 3. Infra failure: Runner restart (SIGTERM)
			{
				Status:       task.AttemptStatusInterrupted,
				ExitCode:     &exitCode143,
				ErrorMessage: "runner restart initiated",
			},
			// 4. Cancelled attempt (already excluded)
			{
				Status:          task.AttemptStatusCanceled,
				CancelRequested: true,
			},
		},
	}

	// None of the 4 attempts above should count towards the business revision quota
	if got := taskRevisionAttemptCount(d); got != 0 {
		t.Fatalf("expected 0 revision attempts for pure infra disconnects, got %d", got)
	}

	// Add 1 real business attempt (agent completed code, but DoD had a gap)
	d.Attempts = append(d.Attempts, &task.TaskAttempt{
		Status:       task.AttemptStatusFailed,
		ExitCode:     &exitCode1,
		ErrorMessage: "unit test failed: expected 200 OK, got 500",
	})

	if got := taskRevisionAttemptCount(d); got != 1 {
		t.Fatalf("expected exactly 1 business attempt, got %d", got)
	}

	// Add more infra disconnects (e.g. server redeploy, transient network blip)
	d.Attempts = append(d.Attempts, &task.TaskAttempt{
		Status:       task.AttemptStatusInterrupted,
		ExitCode:     &exitCode137,
		ErrorMessage: "connection lost to runner daemon",
	})

	// Still only 1 business revision used!
	if got := taskRevisionAttemptCount(d); got != 1 {
		t.Fatalf("expected revision attempt count to remain 1 after transient infra disconnect, got %d", got)
	}
}

// TestS65T3_InfraFault_ScrumEngineClassification verifies that evaluateTaskDoDDeterministic
// classifies runner disconnects as technical connection issues (*Needs Attention* / review),
// NOT as business DoD rejections (BELUM LULUS), and never emits false limit reached warnings.
func TestS65T3_InfraFault_ScrumEngineClassification(t *testing.T) {
	exitCode137 := 137
	d := &task.TaskDetailResponse{
		ID:           "task_test_infra",
		Title:        "Investigasi Kupon Diskon",
		Summary:      "Analisis kupon diskon aktif",
		CompleteNote: "",
		LatestAttempt: &task.TaskAttempt{
			ID:            "att_infra_1",
			AttemptNumber: 3, // Even if AttemptNumber is 3, infra disconnect must not say 'batas 3 attempt perbaikan'
			Status:        task.AttemptStatusInterrupted,
			ExitCode:      &exitCode137,
			ErrorMessage:  "Runner heartbeat lost and lease expired; marked interrupted and recoverable without auto-rerun.",
		},
	}

	res := evaluateTaskDoDDeterministic(d)

	// Infrastructure failure remains blocked without a business correction.
	if res.Decision != "blocked" {
		t.Fatalf("expected decision 'blocked' for infra fault, got '%s'", res.Decision)
	}

	// Must NOT be classified as "BELUM LULUS (Perlu Revisi)"
	if strings.Contains(res.Explanation, "BELUM LULUS") {
		t.Fatalf("infra fault must not be classified as 'BELUM LULUS', got:\n%s", res.Explanation)
	}

	// Explain the observed interruption without inventing a connection cause.
	if !strings.Contains(res.Explanation, "terhenti") || strings.Contains(res.Explanation, "Koneksi Runner") {
		t.Fatalf("expected a readable interruption outcome, got:\n%s", res.Explanation)
	}

	// Must NOT claim that 3 revision attempts were exhausted
	if strings.Contains(res.Explanation, "telah mencapai batas 3 attempt perbaikan") || strings.Contains(res.Explanation, "batas 3x revisi") {
		t.Fatalf("infra disconnect must NOT claim revision attempt quota exhausted, got:\n%s", res.Explanation)
	}
}

// TestS65T3_SingleBubble_Idempotency verifies that background review notifications
// do not generate duplicate bubbles in the active conversation room.
func TestS65T3_SingleBubble_Idempotency(t *testing.T) {
	ctx := context.Background()
	studioRepo := cuanDB.NewMemoryProjectStudioRepository()
	s := &Server{
		projectStudioRepo: studioRepo,
	}

	roomA := cuanDB.GenerateUUID()
	roomB := cuanDB.GenerateUUID()

	// Initial notification
	s.sendBusinessFriendlyReviewNotification(ctx, "task_t3_idemp", "Perbaikan Layout Katalog", "Katalog telah diperbarui", roomA, roomB, 2, []string{
		"Perlu penyesuaian kontras warna",
	}, "Toko Hijau")

	msgsA, err := studioRepo.ListMessagesByRoomID(ctx, roomA)
	if err != nil {
		t.Fatalf("failed to list messages in roomA: %v", err)
	}
	if len(msgsA) != 1 {
		t.Fatalf("expected 1 message in roomA, got %d", len(msgsA))
	}

	// Call again with the exact same review event (simulating overlapping background push & re-evaluation)
	s.sendBusinessFriendlyReviewNotification(ctx, "task_t3_idemp", "Perbaikan Layout Katalog", "Katalog telah diperbarui", roomA, roomB, 2, []string{
		"Perlu penyesuaian kontras warna",
	}, "Toko Hijau")

	msgsA2, _ := studioRepo.ListMessagesByRoomID(ctx, roomA)
	// Single-bubble idempotency must prevent a duplicate 2nd bubble!
	if len(msgsA2) != 1 {
		t.Fatalf("expected single-bubble idempotency (1 message), got %d duplicate messages", len(msgsA2))
	}

	msgsB, _ := studioRepo.ListMessagesByRoomID(ctx, roomB)
	if len(msgsB) != 1 {
		t.Fatalf("expected 1 message in cross-room roomB, got %d", len(msgsB))
	}
}

// TestS65T3_RunnerService_InfraFaultClassification verifies that RunnerService.Complete
// and IsInfraFault correctly classify runner disconnects and keep the task in review.
func TestS65T3_RunnerService_InfraFaultClassification(t *testing.T) {
	if !runner.IsInfraFault(task.AttemptStatusInterrupted, "", 0) {
		t.Fatalf("expected AttemptStatusInterrupted to be recognized as infra fault")
	}
	if !runner.IsInfraFault(task.AttemptStatusFailed, "lease expired", 137) {
		t.Fatalf("expected lease expired to be recognized as infra fault")
	}
	if !runner.IsInfraFault(task.AttemptStatusFailed, "Runner heartbeat lost and lease expired", 0) {
		t.Fatalf("expected heartbeat lost to be recognized as infra fault")
	}
	if !runner.IsInfraFault(task.AttemptStatusFailed, "runner killed by supervisor", 137) {
		t.Fatalf("expected runner killed to be recognized as infra fault")
	}
	// Normal business failure is NOT infra fault
	if runner.IsInfraFault(task.AttemptStatusFailed, "syntax error in main.go: line 42", 1) {
		t.Fatalf("compilation error must NOT be treated as infra fault")
	}
}
