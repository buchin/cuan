package server

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/buchin/cuan/internal/db"
	"github.com/buchin/cuan/internal/task"
)

const recoveredVerificationArtifactsActivity = task.ActivityRecoveredVerificationArtifacts

// The dedicated session output directory is a delivery staging area. Preserve
// its bounded outputs after a verifier times out, without fabricating an agent
// report, successful execution, unchanged scope receipt or task acceptance.
func (s *Server) recoverTerminalVerificationArtifacts(ctx context.Context, d *task.TaskDetailResponse) {
	if d == nil || d.LatestAttempt == nil || d.Status == task.TaskStatusDone || d.Status == "cancelled" || d.Status == "canceled" {
		return
	}
	a := d.LatestAttempt
	if a.Status != task.AttemptStatusFailed || a.ExitCode == nil || *a.ExitCode != 124 || a.CancelRequested || a.CanceledAt != nil || a.FinishedAt == nil {
		return
	}
	for _, event := range d.Activities {
		if event.AttemptID != nil && *event.AttemptID == a.ID && event.ActivityType == recoveredVerificationArtifactsActivity && event.Author == "coordinator" {
			return
		}
	}
	latest, err := s.taskSvc.Store().GetLatestAttempt(ctx, d.ID)
	if err != nil || latest == nil || latest.ID != a.ID || latest.Status != task.AttemptStatusFailed || latest.CancelRequested || latest.CanceledAt != nil {
		return
	}
	current, err := s.taskSvc.Store().GetTask(ctx, d.ID)
	if err != nil || current == nil || current.Status == task.TaskStatusDone || current.Status == "cancelled" || current.Status == "canceled" || current.SourceRoomID != d.SourceRoomID || current.SourceMessageID != d.SourceMessageID || current.Instructions != d.Instructions || current.CommitSHA != d.CommitSHA {
		return
	}
	proof, err := task.ValidateVerificationSource(ctx, s.taskSvc.Store(), &d.Task, a.ID)
	if err != nil {
		return
	}
	if proof == nil {
		// Live/current-outcome inspections have their own bound launch proof.
		// Their files remain observations, never a final report or pass receipt.
		live, err := task.ValidateLiveInspection(ctx, s.taskSvc.Store(), &d.Task, a.ID)
		if err != nil || live == nil {
			return
		}
	}
	delivery, err := s.projectStudioRepo.GetMessageByID(ctx, delegationMessageID("task-delivery:"+a.ID))
	if err != nil && !errors.Is(err, db.ErrMessageNotFound) {
		return
	}
	var ids []string
	if delivery != nil {
		if delivery.RoomID != d.SourceRoomID || delivery.AuthorType != "agent" || delivery.ReplyToID == nil || *delivery.ReplyToID != d.SourceMessageID {
			return
		}
		_, ids = decodeAttachmentMessage(delivery.Content)
	} else {
		grant, ok := s.verificationArtifactGrant(d.ID, a.ID)
		if !ok {
			return
		}
		directory, err := grant.EvidenceRoot.Open(".")
		if err != nil {
			return
		}
		entries, err := directory.ReadDir(9)
		directory.Close()
		if err != nil && !errors.Is(err, io.EOF) {
			return
		}
		if len(entries) == 0 || len(entries) > 8 {
			return
		}
		var paths []string
		for _, entry := range entries {
			if !entry.Type().IsRegular() {
				return
			}
			paths = append(paths, "session:"+entry.Name())
		}
		sort.Strings(paths)
		files, err := readTaskArtifacts(nil, d.ID, a.ID, paths, grant.EvidenceRoot)
		if err != nil || len(files) == 0 {
			return
		}
		owner, err := s.taskArtifactOwner(ctx, d)
		if err != nil {
			return
		}
		if err = s.publishTaskArtifacts(ctx, d, owner, files); err != nil {
			return
		}
		for _, file := range files {
			ids = append(ids, file.attachment.ID)
		}
	}
	if len(ids) == 0 {
		return
	}
	raw, _ := json.Marshal(ids)
	event := &task.TaskActivity{ID: "recovered-files:" + a.ID, TaskID: d.ID, AttemptID: &a.ID, ActivityType: recoveredVerificationArtifactsActivity, Author: "coordinator", NewStatus: "unreviewed", Message: string(raw), CreatedAt: time.Now().UTC()}
	if err = s.taskSvc.Store().CreateActivity(ctx, event); err != nil {
		return
	}
	d.Activities = append(d.Activities, event)
}

func (s *Server) verificationArtifactGrant(taskID, attemptID string) (taskAgentGrant, bool) {
	var found taskAgentGrant
	s.taskAgentGrants.Range(func(_, value any) bool {
		g, ok := value.(taskAgentGrant)
		if ok && g.TaskID == taskID && g.AttemptID == attemptID && g.EvidenceRoot != nil && g.Expires.After(time.Now()) {
			found = g
			return false
		}
		return true
	})
	if found.EvidenceRoot != nil {
		return found, true
	}
	// Grants are encrypted and attempt-bound. Restore only the matching record;
	// never infer ownership from a /tmp directory name or inspect other outputs.
	entries, err := os.ReadDir(filepath.Dir(taskGrantPath([32]byte{})))
	if err != nil {
		return found, false
	}
	for _, entry := range entries {
		if entry.IsDir() || entry.Type()&os.ModeSymlink != 0 || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		raw, err := hex.DecodeString(strings.TrimSuffix(entry.Name(), ".json"))
		if err != nil || len(raw) != 32 {
			continue
		}
		var digest [32]byte
		copy(digest[:], raw)
		record, err := s.readTaskGrantRecord(digest)
		if err != nil || record.TaskID != taskID || record.AttemptID != attemptID || !record.Expires.After(time.Now()) {
			continue
		}
		grant, err := s.loadTaskGrant(digest)
		if err == nil && grant.EvidenceRoot != nil {
			return grant, true
		}
	}
	return found, false
}

func recoveredVerificationOutputs(d *task.TaskDetailResponse) bool {
	if d == nil || d.LatestAttempt == nil {
		return false
	}
	for _, a := range d.Activities {
		if a != nil && a.TaskID == d.ID && a.AttemptID != nil && *a.AttemptID == d.LatestAttempt.ID && a.ActivityType == recoveredVerificationArtifactsActivity && a.Author == "coordinator" {
			return true
		}
	}
	return false
}
