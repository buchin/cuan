package server

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/buchin/cuan/internal/db"
	"github.com/buchin/cuan/internal/scheduler"
	"github.com/buchin/cuan/internal/task"
)

// executeScheduledTask is the callback invoked whenever a ScheduledTask routine is triggered.
// It creates a real Cuan task bound to the specified project, and initiates execution via an agent.
func (s *Server) executeScheduledTask(ctx context.Context, schedTask *scheduler.ScheduledTask) error {
	if schedTask == nil {
		return fmt.Errorf("scheduled task cannot be nil")
	}
	if s.taskSvc == nil {
		return fmt.Errorf("task service is not available")
	}

	projectID := strings.TrimSpace(schedTask.ProjectID)
	if projectID == "" && s.projectSvc != nil {
		if p, err := s.projectSvc.EnsureDefault(ctx); err == nil && p != nil {
			projectID = p.ID
		}
	}

	author := "scheduler"
	ownerID := "owner"
	if s.authSvc != nil {
		if id, err := s.authSvc.OwnerID(ctx); err == nil && id != "" {
			author = id
			ownerID = id
		}
	}

	targetAgent := db.AgentIDCuan
	instructions := schedTask.Prompt
	if isDevNullShipLogTask(schedTask.Title, schedTask.Prompt) {
		instructions = strings.TrimSpace(instructions) + "\n\nFormat Pengajuan Approval di Inbox:\n- Tulis ringkasan cerita siap posting dengan bahasa produk yang natural dan mudah dipahami (hindari jargon teknis internal seperti nama variabel, nama file skrip, atau istilah internal runner).\n- Akhiri teks postingan dengan CTA Sponsorware yang jelas sesuai bahasa postingan (misalnya: \"Ikuti progres build-nya atau dukung Cuan menuju open-source v1.0: https://cuan.work | https://github.com/buchin/cuan\" atau versi Inggris \"Follow the build or back Cuan toward open-source v1.0: https://cuan.work | https://github.com/buchin/cuan\").\n- Lampirkan hanya tangkapan layar UI yang benar-benar ada dan valid di folder screenshots/ via --artifact (atau pilih text-only jika perubahan murni backend).\n- Jangan sertakan header boilerplate atau instruksi balas teks manual; akhiri ringkasan dengan blok ```cuan-approval (sertakan \"execution_category\":\"ops\", \"approve_label\":\"Setuju & Posting\", dan \"action_note\":\"Menyetujui akan langsung memposting draft di atas ke dev-null.sh.\") agar pengguna dapat melihat preview postingan + gambar dan menyetujuinya dengan 1 klik."
	}
	req := task.CreateTaskRequest{
		ProjectID:     projectID,
		Title:         schedTask.Title,
		Instructions:  instructions,
		Priority:      task.PriorityNormal,
		Status:        task.TaskStatusQueued,
		AssignedAgent: targetAgent,
	}
	if projectID != "" {
		req.ProjectIDs = []string{projectID}
	}

	created, err := s.taskSvc.CreateTask(ctx, author, req)
	if err != nil {
		log.Printf("[Scheduler] Failed to create task for routine %q (%s): %v", schedTask.Title, schedTask.ID, err)
		return fmt.Errorf("create task: %w", err)
	}

	if s.taskProjectRepo != nil && len(req.ProjectIDs) > 0 {
		_ = s.taskProjectRepo.SetTaskProjects(ctx, created.ID, req.ProjectIDs)
	}

	log.Printf("[Scheduler] Routine %q (%s) created task %s in project %s", schedTask.Title, schedTask.ID, created.ID, projectID)

	// Attempt immediate agent launch if terminal manager is configured
	if s.terminalManager != nil {
		response := &delegationResponse{header: make(http.Header)}
		request, _ := http.NewRequestWithContext(ctx, "POST", "http://localhost/internal-queued-launch", nil)
		s.runTaskAgent(response, request, created.ID, runTaskAgentPayload{
			RequestID: db.GenerateUUID(),
			AgentID:   targetAgent,
			RunnerID:  "cuan-runner",
			Prompt:    schedTask.Prompt,
		}, ownerID)

		if response.code != 200 && response.code != 0 && response.code != http.StatusAccepted {
			log.Printf("[Scheduler] Routine %s immediate agent dispatch note (status %d): %s; task remains queued for runner supervisor", schedTask.ID, response.code, response.body.String())
			if response.code == http.StatusConflict {
				// Concurrency busy: ensure a queued attempt exists for cuan-runner
				_, _ = s.taskSvc.CreateAttempt(ctx, created.ID, ownerID, task.CreateAttemptRequest{
					RunnerID:             "cuan-runner",
					AgentName:            targetAgent,
					InitializeQueuedTask: true,
				})
			}
		}
	}

	s.kickTaskLaunches()
	return nil
}
