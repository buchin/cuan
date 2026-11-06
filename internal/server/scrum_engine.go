package server

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/buchin/cuan/internal/db"
	"github.com/buchin/cuan/internal/runner"
	"github.com/buchin/cuan/internal/task"
)

// InitScrumEngine initializes the autonomous event-driven Scrum Engine,
// binding completion listeners to runner events.
func (s *Server) InitScrumEngine(ctx context.Context) {
	bus := runner.GlobalEventBus
	if s.runnerSvc != nil && s.runnerSvc.EventBus() != nil {
		bus = s.runnerSvc.EventBus()
	}
	if bus != nil {
		bus.SubscribeCompleted(func(evtCtx context.Context, evt runner.AttemptCompletedEvent) error {
			s.handleRunnerAttemptCompleted(evtCtx, evt.TaskID, evt.AttemptID, evt.Status, evt.ExitCode, evt.Summary, evt.CompleteNote, evt.ErrorMessage, evt.SourceRoomID)
			return nil
		})
	}
}

// onRunnerAttemptCompleted is called when cuan-runner reports an attempt finished via HTTP POST.
func (s *Server) onRunnerAttemptCompleted(ctx context.Context, att *task.TaskAttempt) {
	if att == nil || att.TaskID == "" {
		return
	}
	exitCode := 0
	if att.ExitCode != nil {
		exitCode = *att.ExitCode
	}
	bgCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 60*time.Second)
	go func() {
		defer cancel()
		s.handleRunnerAttemptCompleted(bgCtx, att.TaskID, att.ID, string(att.Status), exitCode, "", "", att.ErrorMessage, "")
	}()
}

// handleRunnerAttemptCompleted processes the completion of an attempt (exit 0 or non-zero/failed),
// automatically triggering a Scrum Master / Chat Agent review turn in the associated chat room.
func (s *Server) handleRunnerAttemptCompleted(ctx context.Context, taskID, attemptID, status string, exitCode int, summary, completeNote, errMsg, sourceRoomID string) {
	if s.taskSvc == nil || s.projectStudioRepo == nil || taskID == "" {
		return
	}

	detail, err := s.taskSvc.GetTaskDetail(ctx, taskID)
	if err != nil || detail == nil || detail.LatestAttempt == nil {
		return
	}

	// A stale completion must not review another attempt or restart queued work.
	if detail.LatestAttempt.ID != attemptID || detail.LatestAttempt.Status != status || detail.Status == task.TaskStatusQueued || detail.Status == "cancelled" {
		return
	}

	// Resolve the target discussion/chat room
	targetRoomID := detail.SourceRoomID
	if targetRoomID == "" && sourceRoomID != "" {
		targetRoomID = sourceRoomID
	}
	if targetRoomID == "" {
		if launch, err := s.taskSvc.LatestLaunch(ctx, detail.ID); err == nil && launch != nil && launch.RoomID != "" {
			targetRoomID = launch.RoomID
		}
	}
	if targetRoomID == "" && detail.ProjectID != "" {
		if rooms, err := s.projectStudioRepo.ListAllRooms(ctx); err == nil {
			for _, r := range rooms {
				if r.ProjectID == detail.ProjectID {
					targetRoomID = r.ID
					break
				}
			}
		}
	}
	if targetRoomID == "" {
		if rooms, err := s.projectStudioRepo.ListAllRooms(ctx); err == nil && len(rooms) > 0 {
			targetRoomID = rooms[0].ID
		}
	}
	if targetRoomID == "" {
		log.Printf("[ScrumEngine] No target chat room found for task %s", taskID)
		return
	}

	if detail.SourceRoomID == "" {
		detail.SourceRoomID = targetRoomID
	}

	// If task is already done, ensure the completion summary report is delivered
	if detail.Status == task.TaskStatusDone {
		s.ensureTaskCompletionDelivered(ctx, detail, targetRoomID)
		return
	}

	// Trigger autonomous task review in the room
	s.queueInboxTaskReview(ctx, detail)
}

func (s *Server) ensureTaskCompletionDelivered(ctx context.Context, detail *task.TaskDetailResponse, roomID string) {
	if s.projectStudioRepo == nil || detail == nil {
		return
	}
	targetRoom := roomID
	if targetRoom == "" {
		targetRoom = detail.SourceRoomID
	}
	if targetRoom == "" {
		return
	}
	// Single-bubble idempotency: verify whether a completion notification was already delivered to targetRoom
	msgs, err := s.projectStudioRepo.ListMessagesByRoomID(ctx, targetRoom)
	if err == nil {
		for _, m := range msgs {
			if strings.Contains(m.Content, "/tasks/"+detail.ID) &&
				(strings.Contains(m.Content, "Hasil pekerjaan memenuhi kriteria") ||
					strings.Contains(m.Content, "Pekerjaan ditandai selesai") ||
					strings.Contains(m.Content, "Pekerjaan telah diselesaikan")) {
				return
			}
		}
	}
	summary := strings.TrimSpace(detail.Summary)
	if summary == "" {
		summary = "Pekerjaan telah diselesaikan dan diverifikasi."
	}
	content := fmt.Sprintf("Hasil pekerjaan memenuhi kriteria yang disepakati.\n\n%s\n\n[View work](/tasks/%s)", summary, detail.ID)
	now := time.Now().UTC()
	msg := &db.ProjectMessage{
		ID:         db.GenerateUUID(),
		RoomID:     targetRoom,
		AuthorType: "agent",
		AuthorID:   "scrum_master",
		AuthorName: "Scrum Master",
		Content:    content,
		CreatedAt:  now,
		UpdatedAt:  now,
	}
	if err := s.projectStudioRepo.CreateMessage(ctx, msg); err == nil {
		s.publishInbox(targetRoom)
	}
}

// evaluateTaskDoDDeterministic performs autonomous deterministic DoD evaluation when
// language model responses are in plain text or during offline/fallback operations.
func evaluateTaskDoDDeterministic(d *task.TaskDetailResponse) inboxReviewEvaluation {
	if d == nil || d.LatestAttempt == nil {
		return inboxReviewEvaluation{
			Decision:    "blocked",
			Explanation: "Data tugas atau attempt tidak ditemukan.",
			Revision:    "",
		}
	}

	att := d.LatestAttempt
	if att.Status == task.AttemptStatusCanceled {
		return inboxReviewEvaluation{Decision: "blocked", Explanation: "Pekerjaan dibatalkan. Hasil dan kriteria selesai belum diverifikasi. Konteks task tersimpan; laporan ini tidak memulai ulang pekerjaan."}
	}
	isSuccess := inboxReviewExecutionSucceeded(d)

	if !isSuccess {
		errMsg := att.ErrorMessage
		if errMsg == "" {
			errMsg = "Eksekusi attempt runner keluar dengan kode kesalahan (non-zero)."
		}
		var exitCode int
		if att.ExitCode != nil {
			exitCode = *att.ExitCode
		}
		if att.Status == task.AttemptStatusInterrupted || runner.IsInfraFault(att.Status, att.ErrorMessage, exitCode) {
			return inboxReviewEvaluation{Decision: "blocked", Explanation: "Eksekusi terhenti sebelum hasilnya dapat diverifikasi."}
		}
		if taskRevisionAttemptCount(d) >= 3 {
			return inboxReviewEvaluation{Decision: "blocked", Explanation: inboxCorrectionLimitExplanation}
		}
		return inboxReviewEvaluation{
			Decision:    "revise",
			Explanation: "Pengerjaan gagal sebelum hasilnya bisa diverifikasi.",
			Revision:    "Perbaiki kegagalan eksekusi pada attempt #" + strconv.Itoa(att.AttemptNumber) + ": " + errMsg,
		}
	}
	// Acceptance still uses the full saved checklist and report. The chat copy
	// is a readable outcome, not another copy of the internal review document.
	if len(task.AcceptanceGaps(&d.Task)) != 0 {
		return inboxReviewEvaluation{
			Decision:    "revise",
			Explanation: "Hasil pekerjaan sudah dilaporkan, tetapi bukti pemeriksaannya belum lengkap.",
			Revision:    "Laporkan bukti pengujian konkret, verifikasi keluaran, dan penuhi seluruh kriteria Definition of Done sebelum menyatakan selesai.",
		}
	}
	return inboxReviewEvaluation{Decision: "pass", Explanation: "Hasil pekerjaan memenuhi kriteria yang disepakati."}

}

// FinishReportedAttempt intentionally preserves the real PTY shutdown code. A
// nonzero exit is reviewable only with a matching final report, never a manually
// changed success state or an older attempt's report. Acceptance still needs DoD.
func inboxReviewExecutionSucceeded(d *task.TaskDetailResponse) bool {
	if d == nil || d.LatestAttempt == nil || d.LatestAttempt.Status != task.AttemptStatusSucceeded {
		return false
	}
	a := d.LatestAttempt
	var final *task.TaskActivity
	for _, activity := range d.Activities {
		if activity != nil && activity.AttemptID != nil && *activity.AttemptID == a.ID && activity.ActivityType == "agent_report" && (final == nil || activity.CreatedAt.After(final.CreatedAt)) {
			final = activity
		}
	}
	if final != nil && final.NewStatus == task.TaskStatusBlocked {
		return false
	}
	if a.ExitCode == nil || *a.ExitCode == 0 {
		return true
	}
	return final != nil && (final.NewStatus == task.TaskStatusReview || final.NewStatus == task.TaskStatusDone) && strings.TrimSpace(d.Summary) != "" && strings.TrimSpace(d.CompleteNote) != ""
}
