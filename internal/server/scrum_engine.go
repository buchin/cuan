package server

import (
	"context"
	"log"
	"strconv"
	"strings"
	"time"

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

	// If task is already done, no need to re-evaluate
	if detail.Status == task.TaskStatusDone {
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

	// Trigger autonomous task review in the room
	s.queueInboxTaskReview(ctx, detail)
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

	var sb strings.Builder
	sb.WriteString("### Evaluasi Kriteria Selesai (Definition of Done)\n")

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
			sb.WriteString("**Status:** PERLU PERHATIAN (Kendala Koneksi Runner / Needs Attention)\n\n")
			sb.WriteString("**Bukti yang Terverifikasi:**\n")
			sb.WriteString("- [x] **Penyimpanan Konteks**: Konteks pekerjaan dan perubahan tersimpan aman.\n\n")
			sb.WriteString("**Catatan Teknis (Infra):**\n")
			sb.WriteString("- [ ] **Koneksi Runner**: " + errMsg + "\n\n")
			sb.WriteString("**Tindakan:** Tugas memerlukan perhatian teknis dan dapat dijalankan ulang secara manual tanpa mengurangi kuota 3x revisi bisnis.")
			return inboxReviewEvaluation{
				Decision:    "blocked",
				Explanation: sb.String(),
				Revision:    "",
			}
		}

		sb.WriteString("**Status:** BELUM LULUS (Perlu Revisi)\n\n")
		sb.WriteString("**Bukti yang Terverifikasi:**\n")
		sb.WriteString("- [x] **Pencatatan Eksekusi**: Runner mencatat proses attempt #" + strconv.Itoa(att.AttemptNumber) + ".\n\n")
		sb.WriteString("**Belum Terbukti / Gap yang Perlu Diperbaiki:**\n")
		sb.WriteString("- [ ] **Eksekusi Berhasil**: " + errMsg + "\n\n")

		if taskRevisionAttemptCount(d) >= 3 {
			sb.WriteString("**Tindakan:** Memerlukan keputusan PO (telah mencapai batas 3 attempt perbaikan).")
			return inboxReviewEvaluation{
				Decision:    "blocked",
				Explanation: sb.String(),
				Revision:    "",
			}
		}

		sb.WriteString("**Tindakan:** Mengembalikan otomatis ke tim eksekusi untuk revisi.")
		return inboxReviewEvaluation{
			Decision:    "revise",
			Explanation: sb.String(),
			Revision:    "Perbaiki kegagalan eksekusi pada attempt #" + strconv.Itoa(att.AttemptNumber) + ": " + errMsg,
		}
	}

	// A recorded success can include intentional PTY shutdown after a validated report.
	hasVerification := len(task.AcceptanceGaps(&d.Task)) == 0
	if !hasVerification {
		sb.WriteString("**Status:** BELUM LULUS (Perlu Revisi)\n\n")
		sb.WriteString("**Bukti yang Terverifikasi:**\n")
		sb.WriteString("- [x] **Proses Selesai**: Attempt #" + strconv.Itoa(att.AttemptNumber) + " selesai dengan laporan hasil yang tercatat.\n\n")
		sb.WriteString("**Belum Terbukti / Gap yang Perlu Diperbaiki:**\n")
		sb.WriteString("- [ ] **Bukti Verifikasi DoD**: Belum ada catatan verifikasi atau bukti pengujian yang dilaporkan.\n\n")
		sb.WriteString("**Tindakan:** Mengembalikan otomatis ke tim eksekusi untuk revisi.")
		return inboxReviewEvaluation{
			Decision:    "revise",
			Explanation: sb.String(),
			Revision:    "Laporkan bukti pengujian konkret, verifikasi keluaran, dan penuhi seluruh kriteria Definition of Done sebelum menyatakan selesai.",
		}
	}

	sb.WriteString("**Status:** LULUS\n\n")
	sb.WriteString("**Bukti yang Terverifikasi:**\n")
	if len(d.DoDChecklist) > 0 {
		for _, item := range d.DoDChecklist {
			sb.WriteString("- [x] **" + item.Text + "**: Terverifikasi oleh tim eksekusi attempt #" + strconv.Itoa(att.AttemptNumber) + ".\n")
		}
	} else {
		sb.WriteString("- [x] **Kriteria Tugas**: Dilaporkan terverifikasi pada attempt #" + strconv.Itoa(att.AttemptNumber) + ".\n")
	}
	if strings.TrimSpace(d.CompleteNote) != "" {
		sb.WriteString("- [x] **Catatan Verifikasi**: " + strings.TrimSpace(d.CompleteNote) + "\n")
	}

	sb.WriteString("\n**Belum Terbukti / Gap yang Perlu Diperbaiki:**\n")
	sb.WriteString("- [ ] **Tidak ada**: Seluruh kriteria Definition of Done terpenuhi.\n\n")
	sb.WriteString("**Tindakan:** Menyatakan selesai.")

	return inboxReviewEvaluation{
		Decision:    "pass",
		Explanation: sb.String(),
		Revision:    "",
	}
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
