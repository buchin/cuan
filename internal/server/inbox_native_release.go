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

// Developer handoffs enter independent verification first. Only a source-stage
// approval can admit native publication; neither stage accepts the whole task.
func (s *Server) recoverNativeReleaseHandoff(ctx context.Context, d *task.TaskDetailResponse, requests []db.InboxRequest) bool {
	if d == nil || d.LatestAttempt == nil || d.LatestAttempt.Status != task.AttemptStatusSucceeded || d.Status != task.TaskStatusReview || s.projectSvc == nil {
		return false
	}
	source := d.LatestAttempt
	reviewID := ""
	if approved, err := task.NativeSourceReviewForAttempt(ctx, s.taskSvc.Store(), &d.Task, d.LatestAttempt.ID); err == nil && approved != nil {
		original, proof := s.verifiedSupplementalSource(ctx, d)
		if original == nil || proof == nil || proof.SourceAttemptID != approved.SourceAttemptID {
			return false
		}
		source, reviewID = original.LatestAttempt, d.LatestAttempt.ID
	}
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
		proof := task.NativeReleaseIntent{SourceReviewAttemptID: reviewID, SourceAttemptID: source.ID, SourceCommit: handoff.SourceCommit, SourceVerificationHash: handoff.VerificationHash, InstructionsHash: handoff.InstructionsHash, OwnerID: origin.OwnerID, RoomID: d.SourceRoomID, SourceRequestID: origin.RequestID, Repository: handoff.Repository, TargetBranch: handoff.TargetBranch, ManifestHash: handoff.ManifestHash}
		if reviewID == "" {
			if task.ValidateNativeDeveloperHandoff(ctx, s.taskSvc.Store(), &d.Task, &proof) != nil {
				return false
			}
			assessmentRequest := origin
			assessmentRequest.RequestID = delegationMessageID("native-source-verification:" + source.ID + ":" + proof.InstructionsHash)
			assessment := inboxReviewEvaluation{Decision: "revise", RequestedOutcome: "unfinished", RemainingWork: "verification", Explanation: "Source implementation is ready for independent review; publication remains pending.", Revision: "Independently inspect the exact source against every original source requirement and scope restriction, run relevant checks, and report concrete defects or observed passing evidence. Preserve code, configuration, refs, commits and production. Do not waive pending publication or live checks."}
			saved, err := s.saveReviewAssessment(ctx, d, assessmentRequest, assessment)
			if err != nil {
				return false
			}
			_, started := s.startVerificationFollowup(ctx, assessmentRequest, d, saved)
			return started
		}
		if task.ValidateNativeReleaseSource(ctx, s.taskSvc.Store(), &d.Task, &proof) != nil {
			return false
		}
		// A namespace upgrade is not permission to repeat an existing effect.
		if observed := s.inboxReviewPublication(ctx, d); observed != nil && observed.Verified {
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
