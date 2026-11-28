package server

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"os"
	"strings"
	"time"

	"github.com/buchin/cuan/internal/db"
	"github.com/buchin/cuan/internal/runner"
	"github.com/buchin/cuan/internal/task"
)

const inboxVisualReviewPolicy = "Visual evidence review policy: v2"

// Version one supplied at most eight images. Revisit only unfinished reviews
// whose supplemental evidence was crowded out, or older text-only reviews.
func visualReviewNeedsRefresh(previous string, imageCount int) bool {
	if imageCount == 0 || strings.Contains(previous, inboxVisualReviewPolicy) {
		return false
	}
	return imageCount > 8 || !strings.Contains(previous, "Visual evidence review policy: v1")
}

type reviewImage struct {
	NativeCapture bool   `json:"runner_capture_verified"`
	ID            string `json:"attachment_id"`
	Name          string `json:"name"`
	Path          string `json:"path"`
	Width         int    `json:"width,omitempty"`
	Height        int    `json:"height,omitempty"`
}

// Only immutable delivered files from the exact attempt are review inputs.
// Paths are resolved by the server, never accepted from report text or a model.
func (s *Server) inboxReviewImages(ctx context.Context, d *task.TaskDetailResponse) []reviewImage {
	if d == nil || d.LatestAttempt == nil || s.projectStudioRepo == nil {
		return nil
	}
	var deliveries []*db.ProjectMessage
	if native, err := s.projectStudioRepo.GetMessageByID(ctx, delegationMessageID("task-visual-delivery:"+d.LatestAttempt.ID)); err == nil && native != nil {
		deliveries = append(deliveries, native)
	}
	if ordinary, err := s.inboxReviewDelivery(ctx, d); err == nil && ordinary != nil {
		deliveries = append(deliveries, ordinary)
	}
	owner, err := s.taskArtifactOwner(ctx, d)
	if err != nil {
		return nil
	}
	var ids []string
	seen := map[string]bool{}
	for _, m := range deliveries {
		if m.AuthorType != "agent" || m.RoomID != d.SourceRoomID || !matchesTaskSourceMessage(m.ReplyToID, d.SourceMessageID) {
			continue
		}
		_, attached := decodeAttachmentMessage(m.Content)
		for _, id := range attached {
			if !seen[id] {
				ids = append(ids, id)
				seen[id] = true
			}
		}
	}
	receipt := task.VisualVerificationForAttempt(d.Activities, d.ID, d.LatestAttempt.ID, d.LatestAttempt.RunnerID)
	var images []reviewImage
	for _, id := range ids {
		// Native regression captures and supplemental feature evidence from the same attempt.
		if len(images) >= 32 {
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
		file.NativeCapture = nativeImageMatches(d, receipt, file)
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
	if (decision.Decision == "pass" || decision.Decision == "revise") && d != nil && d.LatestAttempt != nil && d.LatestAttempt.Status == task.AttemptStatusFailed && strings.HasPrefix(d.LatestAttempt.ErrorMessage, runner.InspectionDependencyFailurePrefix) {
		return inboxReviewEvaluation{Decision: "revise", RequestedOutcome: "unfinished", RemainingWork: "verification", Finding: "Persiapan pengujian belum berhasil.", Explanation: "Pemeriksaan belum berjalan karena persiapan dependency gagal; hasil belum bisa diterima.", Revision: "Inspection setup failed before visual capture or feature verification. Diagnose the native dependency preparation error first, preserving the approved source, read-only snapshot and existing scope. No captures were produced at this stage; do not inspect nonexistent images, fabricate screenshots or repeat deployment. After the prerequisite is resolved, obtain fresh verification through the authorized native path. Recorded setup diagnostic: " + d.LatestAttempt.ErrorMessage}
	}

	if decision.Decision == "pass" && d != nil && d.LatestAttempt != nil {
		expected, err := task.BoundVisualRecipe(d.Activities, d.ID, d.LatestAttempt.ID)
		if err != nil || expected != nil {
			receipt := task.VisualVerificationForAttempt(d.Activities, d.ID, d.LatestAttempt.ID, d.LatestAttempt.RunnerID)
			matched := 0
			for _, picture := range s.inboxReviewImages(ctx, d) {
				if picture.NativeCapture {
					matched++
				}
			}
			if err != nil || receipt == nil || matched != len(receipt.Captures) {
				return inboxReviewEvaluation{Decision: "revise", RequestedOutcome: "unfinished", RemainingWork: "verification", Finding: "Bukti visual dari pemeriksaan proyek belum lengkap atau berubah.", Explanation: "Bukti visual dari pemeriksaan proyek belum lengkap atau berubah; hasil belum diterima.", Revision: "Inspect the existing files generated by the bound project visual verifier. Preserve scope and execution limits. Missing or substituted captures do not authorize another execution or reconstructed HTML evidence."}
			}
		}
	}
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

func nativeImageMatches(d *task.TaskDetailResponse, receipt *task.VisualVerification, picture reviewImage) bool {
	if receipt == nil || d == nil || d.LatestAttempt == nil {
		return false
	}
	for _, capture := range receipt.Captures {
		if picture.ID != delegationMessageID(task.VisualCaptureAttachmentKey(d.ID, d.LatestAttempt.ID, capture)) || picture.Width != capture.Width || picture.Height != capture.Height {
			continue
		}
		f, err := os.Open(picture.Path)
		if err != nil {
			return false
		}
		hash := sha256.New()
		size, err := io.Copy(hash, io.LimitReader(f, attachmentLimit+1))
		f.Close()
		return err == nil && size <= attachmentLimit && hex.EncodeToString(hash.Sum(nil)) == capture.SHA256
	}
	return false
}
func nativeVisualReviewData(d *task.TaskDetailResponse) *task.VisualVerification {
	if d == nil || d.LatestAttempt == nil {
		return nil
	}
	return task.VisualVerificationForAttempt(d.Activities, d.ID, d.LatestAttempt.ID, d.LatestAttempt.RunnerID)
}
