package server

import (
	"context"
	"errors"

	"github.com/buchin/cuan/internal/db"
	"github.com/buchin/cuan/internal/task"
)

const inboxCorrectionLimitExplanation = "Batas perbaikan otomatis sudah tercapai; hasil sementara dan bukti tetap tersimpan."

func inboxCorrectionRecommendation(remaining string) string {
	switch remaining {
	case "verification":
		return "Saya sarankan menyusun pemeriksaan lanjutan yang hanya melengkapi bukti yang masih kurang, berdasarkan hasil yang sudah tersimpan."
	case "implementation":
		return "Saya sarankan mempersempit perbaikan ke hambatan yang ditemukan, sambil mempertahankan hasil yang sudah benar."
	case "external":
		return "Saya sarankan menyelesaikan prasyarat eksternal yang disebutkan sebelum pekerjaan dilanjutkan."
	default:
		return "Saya sarankan meninjau hasil yang tersimpan untuk menentukan satu langkah lanjutan yang paling tepat."
	}
}

// An accepted owner continuation starts a new bounded correction cycle. Old
// attempts remain evidence, but must not spend a newly authorized cycle's budget.
// A system review or an unrelated owner message cannot reset that cycle.
func (s *Server) ownerContinuationLaunch(ctx context.Context, d *task.TaskDetailResponse) (*task.TaskLaunch, error) {
	if d == nil || d.LatestAttempt == nil || d.LatestAttempt.CreatedAt.IsZero() || s.taskSvc == nil {
		return nil, nil
	}
	reader, ok := s.projectStudioRepo.(db.InboxRequestRepository)
	if !ok {
		return nil, errors.New("conversation approval storage unavailable")
	}
	rooms := []string{d.SourceRoomID}
	latest, err := s.taskSvc.LatestLaunch(ctx, d.ID)
	if err != nil && !errors.Is(err, task.ErrTaskNotFound) {
		return nil, err
	}
	if latest != nil && latest.RoomID != d.SourceRoomID {
		rooms = append(rooms, latest.RoomID)
	}
	var selected *task.TaskLaunch
	for _, room := range rooms {
		requests, err := reader.ListInboxRequests(ctx, room)
		if err != nil {
			return nil, err
		}
		for _, q := range requests {
			if q.MessageID == d.SourceMessageID || q.CreatedAt.After(d.LatestAttempt.CreatedAt) || q.State == "cancelled" || q.State == "canceled" || q.State == "cancelling" {
				continue
			}
			launch, err := s.taskSvc.GetLaunch(ctx, task.LaunchID(d.ID, q.RequestID))
			if err != nil && !errors.Is(err, task.ErrTaskNotFound) {
				return nil, err
			}
			if err != nil || launch == nil || launch.OwnerID != q.OwnerID || launch.RoomID != q.RoomID || launch.ProjectID != d.ProjectID || launch.InstructionsHash != task.LaunchInstructionsHash(d.Instructions) || launch.Revision == "" || launch.State == "cancelled" || launch.CreatedAt.After(d.LatestAttempt.CreatedAt) || (selected != nil && !launch.CreatedAt.After(selected.CreatedAt)) {
				continue
			}
			message, err := s.projectStudioRepo.GetMessageByID(ctx, q.MessageID)
			if err != nil {
				return nil, err
			}
			if message != nil && message.AuthorType == "user" && message.RoomID == q.RoomID && !message.CreatedAt.After(launch.CreatedAt) {
				selected = launch
			}
		}
	}
	return selected, nil
}

func (s *Server) reviewBudgetDetail(ctx context.Context, d *task.TaskDetailResponse) *task.TaskDetailResponse {
	selected, err := s.ownerContinuationLaunch(ctx, d)
	if err != nil || selected == nil {
		return d
	}
	filtered := *d
	filtered.Attempts = nil
	for _, a := range d.Attempts {
		if a != nil && !a.CreatedAt.Before(selected.CreatedAt) {
			filtered.Attempts = append(filtered.Attempts, a)
		}
	}
	return &filtered
}
