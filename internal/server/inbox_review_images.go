package server

import (
	"context"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"os"
	"strings"
	"time"

	"github.com/buchin/cuan/internal/db"
	"github.com/buchin/cuan/internal/task"
)

const inboxVisualReviewPolicy = "Visual evidence review policy: v1"

type reviewImage struct {
	ID     string `json:"attachment_id"`
	Name   string `json:"name"`
	Path   string `json:"path"`
	Width  int    `json:"width,omitempty"`
	Height int    `json:"height,omitempty"`
}

// Only immutable delivered files from the exact attempt are review inputs.
// Paths are resolved by the server, never accepted from report text or a model.
func (s *Server) inboxReviewImages(ctx context.Context, d *task.TaskDetailResponse) []reviewImage {
	if d == nil || d.LatestAttempt == nil || s.projectStudioRepo == nil {
		return nil
	}
	m, err := s.inboxReviewDelivery(ctx, d)
	if err != nil || m == nil || m.AuthorType != "agent" || m.ReplyToID == nil || *m.ReplyToID != d.SourceMessageID {
		return nil
	}
	owner, err := s.taskArtifactOwner(ctx, d)
	if err != nil {
		return nil
	}
	_, ids := decodeAttachmentMessage(m.Content)
	var images []reviewImage
	for _, id := range ids {
		if len(images) >= 8 {
			break
		}
		a, path, err := loadAttachment(owner, id)
		if err != nil || !strings.HasPrefix(a.Type, "image/") {
			continue
		}
		file := reviewImage{ID: a.ID, Name: a.Name, Path: path}
		f, err := os.Open(path)
		if err != nil {
			continue
		}
		config, _, decodeErr := image.DecodeConfig(io.LimitReader(f, 1<<20))
		f.Close()
		if decodeErr == nil {
			file.Width = config.Width
			file.Height = config.Height
		}
		images = append(images, file)
	}
	return images
}
func (s *Server) inboxReviewImagePaths(ctx context.Context, q db.InboxRequest) []string {
	d := s.inboxReviewedTask(ctx, q)
	if d == nil {
		return nil
	}
	var paths []string
	for _, a := range s.inboxReviewImages(ctx, d) {
		paths = append(paths, a.Path)
	}
	return paths
}

func (s *Server) requireVisualReview(ctx context.Context, d *task.TaskDetailResponse, decision inboxReviewEvaluation) inboxReviewEvaluation {
	if decision.Decision == "pass" && len(s.inboxReviewImages(ctx, d)) > 0 && strings.TrimSpace(decision.VisualInspection) == "" {
		return inboxReviewEvaluation{Decision: "revise", Finding: "Bukti gambar belum diperiksa secara visual.", RemainingWork: "verification", Explanation: "Bukti gambar belum diperiksa secara visual; hasil belum diterima.", Revision: "Inspect the exact delivered images with a vision-capable reviewer. Compare actual rendered application content and viewport dimensions to original requirements; generated mockups and report claims cannot substitute for application screenshots. Do not change code or publish."}
	}
	return decision
}

// Recheck only accepted supplemental verifier attempts affected by the old
// text-only policy. Cancellation, new attempts and ordinary accepted history
// are untouched. This does not start an execution or reset any correction limit.
func (s *Server) reconcileVisualAcceptances(ctx context.Context) {
	reader, ok := s.taskSvc.Store().(task.VerificationAcceptanceAuditStore)
	if !ok {
		return
	}
	ids, err := reader.UninspectedVerificationTaskIDs(ctx)
	if err != nil {
		return
	}
	for _, id := range ids {
		d, err := s.taskSvc.GetTaskDetail(ctx, id)
		if err != nil || d.Status != task.TaskStatusDone || d.LatestAttempt == nil {
			continue
		}
		proof, err := task.ValidateVerificationSource(ctx, s.taskSvc.Store(), &d.Task, d.LatestAttempt.ID)
		if err != nil || proof == nil || d.LatestAttempt.CancelRequested || d.LatestAttempt.CanceledAt != nil {
			continue
		}
		assessment, found := reviewAssessment(d, "")
		state := "not_required"
		if len(s.inboxReviewImages(ctx, d)) > 0 {
			if !found || strings.TrimSpace(assessment.VisualInspection) == "" {
				review := task.TaskStatusReview
				if _, err = s.taskSvc.UpdateTask(ctx, d.ID, "coordinator", task.UpdateTaskRequest{Status: &review}); err != nil {
					continue
				}
				state = "review_required"
			} else {
				state = "inspected"
			}
		}
		_ = s.taskSvc.Store().CreateActivity(ctx, &task.TaskActivity{ID: "visual-audit:" + d.LatestAttempt.ID, TaskID: d.ID, AttemptID: &d.LatestAttempt.ID, ActivityType: task.ActivityVisualAcceptanceAudit, Author: "coordinator", NewStatus: state, Message: "Supplemental verification image evidence requires independent visual review before acceptance.", CreatedAt: time.Now().UTC()})
	}
}
