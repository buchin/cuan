package server

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"

	"github.com/buchin/cuan/internal/db"
	"github.com/buchin/cuan/internal/task"
)

func (s *Server) startVerificationFinalization(ctx context.Context, q db.InboxRequest, d *task.TaskDetailResponse, assessment inboxReviewEvaluation) (string, bool) {
	if d == nil || d.LatestAttempt == nil || d.Status == task.TaskStatusDone || d.Status == "cancelled" || d.Status == "canceled" || q.RoomID != d.SourceRoomID || assessment.Decision != "revise" || assessment.RemainingWork != "verification" || assessment.VisualInspection == "" || !recoveredVerificationOutputs(d) {
		return "", false
	}
	a := d.LatestAttempt
	if a.Status != task.AttemptStatusFailed || a.ExitCode == nil || *a.ExitCode != 124 || a.CancelRequested || a.CanceledAt != nil {
		return "", false
	}
	saved, found := reviewAssessment(d, q.RequestID)
	if !found || saved != assessment {
		return "", false
	}
	parent, err := task.ValidateVerificationSource(ctx, s.taskSvc.Store(), &d.Task, a.ID)
	if err != nil || parent == nil || parent.FinalizeAttemptID != "" || parent.RecoveryAttemptID == "" || parent.OwnerID != q.OwnerID {
		return "", false
	}
	baseline := task.VerificationScopeBaseline(d, parent.RecoveryAttemptID)
	if baseline == "" {
		return "", false
	}
	manifest := ""
	for _, event := range d.Activities {
		if event.TaskID == d.ID && event.AttemptID != nil && *event.AttemptID == a.ID && event.ActivityType == recoveredVerificationArtifactsActivity && event.Author == "coordinator" {
			manifest = task.VerificationSummaryHash(event.Message)
			break
		}
	}
	if manifest == "" {
		return "", false
	}
	proof := *parent
	proof.FinalizeAttemptID = a.ID
	proof.RepositoryBaseline = baseline
	proof.ArtifactManifestHash = manifest
	proof.AssessmentRequestID = q.RequestID
	if _, err = s.finalizationFiles(ctx, d, &proof); err != nil {
		return "", false
	}
	repo, ok := s.projectStudioRepo.(db.InboxRequestRepository)
	if !ok {
		return "", false
	}
	requests, err := repo.ListInboxRequests(ctx, d.SourceRoomID)
	if err != nil {
		return "", false
	}
	authorized := false
	for _, origin := range requests {
		if origin.RequestID == proof.SourceRequestID && origin.OwnerID == q.OwnerID && s.reviewSnapshotRecoveryAuthorized(ctx, d, origin) {
			authorized = true
			break
		}
	}
	if !authorized {
		return "", false
	}
	id := task.VerificationFinalizeRequestID(d.ID, proof.SourceCommit)
	raw, _ := json.Marshal(proof)
	revision := "Scope: read-only repository. Do not push. Do not deploy.\n" + task.VerificationIntentMarker + string(raw) + "\nDeterministic saved-output finalization only. Do not run an AI adapter, project commands, install tools, prepare a workspace, change files, push or deploy. Verify the exact clean source and the saved repository fingerprint, then submit machine evidence for independent review. The prior timeout remains historical fact."
	previous, err := s.taskSvc.GetLaunch(ctx, task.LaunchID(d.ID, id))
	if err != nil && !errors.Is(err, task.ErrTaskNotFound) {
		return "", false
	}
	if previous != nil {
		parsed, err := task.VerificationIntentForLaunch(&d.Task, previous)
		if err != nil || parsed == nil || parsed.FinalizeAttemptID != proof.FinalizeAttemptID || (previous.State != "queued" && previous.State != "claimed" && previous.State != "started") {
			return "", false
		}
		return verificationFinalizationNotice(d), true
	}
	s.startAuthorizedLaunch(ctx, q, d, id, proof.SourceRequestID, revision)
	launch, err := s.taskSvc.GetLaunch(ctx, task.LaunchID(d.ID, id))
	if err != nil || launch == nil || (launch.State != "queued" && launch.State != "claimed" && launch.State != "started") {
		return "", false
	}
	return verificationFinalizationNotice(d), true
}
func verificationFinalizationNotice(d *task.TaskDetailResponse) string {
	return "Berkas verifikasi sudah tersedia. Cuan memeriksa hasilnya sebelum menuntaskan review.\n\n[View work](/tasks/" + d.ID + ")"
}

// Reuse immutable delivered bytes, retaining their original attempt IDs and
// hashes. The finalizer does not claim to have generated the screenshots.
func (s *Server) finalizationFiles(ctx context.Context, d *task.TaskDetailResponse, proof *task.VerificationIntent) ([]preparedTaskArtifact, error) {
	bad := errors.New("saved verification files could not be authenticated")
	message, err := s.projectStudioRepo.GetMessageByID(ctx, delegationMessageID("task-delivery:"+proof.FinalizeAttemptID))
	if err != nil || message == nil || message.AuthorType != "agent" || message.RoomID != d.SourceRoomID || message.ReplyToID == nil || *message.ReplyToID != d.SourceMessageID {
		return nil, bad
	}
	_, ids := decodeAttachmentMessage(message.Content)
	raw, _ := json.Marshal(ids)
	if len(ids) == 0 || len(ids) > 8 || task.VerificationSummaryHash(string(raw)) != proof.ArtifactManifestHash {
		return nil, bad
	}
	owner, err := s.taskArtifactOwner(ctx, d)
	if err != nil {
		return nil, bad
	}
	var files []preparedTaskArtifact
	total := 0
	for _, id := range ids {
		metadata, path, err := loadAttachment(owner, id)
		if err != nil {
			return nil, bad
		}
		f, err := os.Open(path)
		if err != nil {
			return nil, bad
		}
		data, readErr := io.ReadAll(io.LimitReader(f, attachmentLimit+1))
		f.Close()
		total += len(data)
		if readErr != nil || len(data) > attachmentLimit || total > 25<<20 {
			return nil, bad
		}
		hash := sha256.Sum256(data)
		expected := delegationMessageID("task-artifact:" + d.ID + ":" + proof.FinalizeAttemptID + ":session:" + metadata.Name + ":" + hex.EncodeToString(hash[:]))
		if expected != id {
			return nil, bad
		}
		files = append(files, preparedTaskArtifact{attachment: metadata, data: data})
	}
	return files, nil
}
func (s *Server) linkFinalizationArtifacts(ctx context.Context, d *task.TaskDetailResponse) (bool, error) {
	proof, err := task.VerificationIntentForAttempt(ctx, s.taskSvc.Store(), &d.Task, d.LatestAttempt.ID)
	if err != nil {
		return false, err
	}
	if proof == nil || proof.FinalizeAttemptID == "" {
		return true, nil
	}
	if d.LatestAttempt.Status != task.AttemptStatusSucceeded {
		return true, nil
	}
	if _, err := s.finalizationFiles(ctx, d, proof); err != nil {
		return false, err
	}
	return true, nil
}

func (s *Server) inboxReviewDelivery(ctx context.Context, d *task.TaskDetailResponse) (*db.ProjectMessage, error) {
	proof, err := task.VerificationIntentForAttempt(ctx, s.taskSvc.Store(), &d.Task, d.LatestAttempt.ID)
	if err != nil {
		return nil, err
	}
	origin := d.LatestAttempt.ID
	if proof != nil && proof.FinalizeAttemptID != "" {
		valid := false
		for _, a := range d.Activities {
			if a != nil && a.AttemptID != nil && *a.AttemptID == d.LatestAttempt.ID && a.TaskID == d.ID && a.ActivityType == task.ActivityRunnerFinalization && a.Author == d.LatestAttempt.RunnerID && d.LatestAttempt.Status == task.AttemptStatusSucceeded {
				var report task.FinalizationReport
				if json.Unmarshal([]byte(a.Message), &report) == nil && report.OriginAttemptID == proof.FinalizeAttemptID && report.ManifestHash == proof.ArtifactManifestHash {
					valid = true
					break
				}
			}
		}
		if !valid {
			return nil, errors.New("finalization output provenance is unavailable")
		}
		origin = proof.FinalizeAttemptID
	}
	return s.projectStudioRepo.GetMessageByID(ctx, delegationMessageID("task-delivery:"+origin))
}
