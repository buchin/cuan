package server

import (
	"context"
	"encoding/json"
	"io"
	"strings"

	"github.com/buchin/cuan/internal/db"
	"github.com/buchin/cuan/internal/git"
	"github.com/buchin/cuan/internal/project"
	"github.com/buchin/cuan/internal/task"
)

// A canonical developer handoff determines the native next stage. It is not a
// semantic PM decision and must not spend another model turn to choose deploy.
func (s *Server) recoverNativeReleaseHandoff(ctx context.Context, d *task.TaskDetailResponse, requests []db.InboxRequest) bool {
	if d == nil || d.LatestAttempt == nil || d.LatestAttempt.Status != task.AttemptStatusSucceeded || d.Status != task.TaskStatusReview || s.projectSvc == nil {
		return false
	}
	source := d.LatestAttempt
	var handoff *task.NativeReleaseHandoff
	for _, event := range d.Activities {
		if event == nil || event.ID != "native-release-handoff:"+source.ID || event.ActivityType != task.ActivityNativeReleaseHandoff || event.Author != source.RunnerID || event.AttemptID == nil || *event.AttemptID != source.ID {
			continue
		}
		var saved task.NativeReleaseHandoff
		decoder := json.NewDecoder(strings.NewReader(event.Message))
		decoder.DisallowUnknownFields()
		if decoder.Decode(&saved) != nil || decoder.Decode(&struct{}{}) != io.EOF || saved.Version != 1 || saved.TaskID != d.ID || saved.AttemptID != source.ID {
			return false
		}
		handoff = &saved
	}
	if handoff == nil {
		return false
	}
	p, err := s.projectSvc.TaskContext(ctx, d.ProjectID, d.Instructions)
	if err != nil || p == nil || p.IsArchived {
		return false
	}
	owner, repo, err := git.ParseGitHubURL(p.RepoURL)
	settings := project.SettingsFromMetadata(p.Metadata)
	branch := strings.TrimSpace(settings.DeployBranch)
	if branch == "" {
		branch = p.DefaultBranch
	}
	if err != nil || handoff.Repository != owner+"/"+repo || handoff.TargetBranch != branch || !settings.IsAutoDeployEnabled() || project.ParseEnvironment(settings.Environment)["CUAN_MANAGED_RELEASE"] != "1" {
		return false
	}
	for _, origin := range requests {
		if origin.MessageID != d.SourceMessageID || origin.RoomID != d.SourceRoomID || origin.OwnerID == "" || origin.RequestID == "" {
			continue
		}
		proof := task.NativeReleaseIntent{SourceAttemptID: source.ID, SourceCommit: handoff.SourceCommit, SourceVerificationHash: handoff.VerificationHash, InstructionsHash: handoff.InstructionsHash, OwnerID: origin.OwnerID, RoomID: d.SourceRoomID, SourceRequestID: origin.RequestID, Repository: handoff.Repository, TargetBranch: handoff.TargetBranch, ManifestHash: handoff.ManifestHash}
		if task.ValidateNativeReleaseSource(ctx, s.taskSvc.Store(), &d.Task, &proof) != nil {
			return false
		}
		raw, _ := json.Marshal(proof)
		requestID := task.NativeReleaseRequestID(d.ID, proof.SourceCommit)
		s.startAuthorizedLaunch(ctx, origin, d, requestID, origin.RequestID, task.NativeReleaseIntentMarker+string(raw))
		saved, err := s.taskSvc.GetLaunch(ctx, task.LaunchID(d.ID, requestID))
		if err != nil || saved == nil {
			return false
		}
		parsed, err := task.NativeReleaseIntentForLaunch(&d.Task, saved)
		return err == nil && parsed != nil && *parsed == proof && (saved.State == "queued" || saved.State == "claimed" || saved.State == "started")
	}
	return false
}
