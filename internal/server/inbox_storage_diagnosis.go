package server

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"path/filepath"
	"time"

	"github.com/buchin/cuan/internal/agent"
	"github.com/buchin/cuan/internal/db"
	"github.com/buchin/cuan/internal/runner"
	"github.com/buchin/cuan/internal/task"
)

const storageDiagnosisActivity = "execution_diagnosis"
const storageDiagnosisAuthor = "cuan-diagnostics"

type storageDiagnosis struct {
	Kind             string    `json:"kind"`
	InstructionsHash string    `json:"instructions_hash"`
	EvidenceHash     string    `json:"evidence_hash,omitempty"`
	StartedAt        time.Time `json:"started_at"`
	FinishedAt       time.Time `json:"finished_at"`
}

func diagnosedStorageInterruption(d *task.TaskDetailResponse, a *task.TaskAttempt) bool {
	if d == nil || a == nil || a.Status != task.AttemptStatusFailed || a.ExitCode == nil || *a.ExitCode != 2 || a.StartedAt == nil || a.FinishedAt == nil {
		return false
	}
	for _, event := range d.Activities {
		if event == nil || event.TaskID != d.ID || event.AttemptID == nil || *event.AttemptID != a.ID || event.ActivityType != storageDiagnosisActivity || event.Author != storageDiagnosisAuthor {
			continue
		}
		var proof storageDiagnosis
		if json.Unmarshal([]byte(event.Message), &proof) != nil {
			continue
		}
		digest, err := hex.DecodeString(proof.EvidenceHash)
		if err == nil && len(digest) == 32 && proof.Kind == "storage_exhausted" && proof.InstructionsHash == task.LaunchInstructionsHash(d.Instructions) && proof.StartedAt.Equal(*a.StartedAt) && proof.FinishedAt.Equal(*a.FinishedAt) {
			return true
		}
	}
	return false
}

// Reconcile a legacy opaque exit only once. This records a diagnosis, never a
// replacement attempt outcome or permission to publish. Normal launch admission
// still revalidates owner approval, cancellation, concurrency and storage.
func (s *Server) diagnoseInboxStorageFailure(ctx context.Context, d *task.TaskDetailResponse, origin db.InboxRequest) {
	if s.cfg == nil || s.cfg.WorkspacesDir == "" || d == nil || d.LatestAttempt == nil || d.Status != task.TaskStatusBlocked || !s.reviewSnapshotRecoveryAuthorized(ctx, d, origin) {
		return
	}
	_, authorized, authorizationErr := s.authorizeSavedTaskLaunch(ctx, task.TaskLaunch{OwnerID: origin.OwnerID, RoomID: origin.RoomID, SourceRequestID: origin.RequestID, CreatedAt: time.Now().UTC()}, d)
	if authorizationErr != nil || !authorized {
		return
	}
	a := d.LatestAttempt
	if a.Status != task.AttemptStatusFailed || a.ExitCode == nil || *a.ExitCode != 2 || a.StartedAt == nil || a.FinishedAt == nil || time.Since(*a.FinishedAt) > 7*24*time.Hour || a.ID == "" || d.ID == "" || filepath.Base(d.ID) != d.ID {
		return
	}
	for _, event := range d.Activities {
		if event != nil && event.ID == "storage-diagnosis:"+a.ID {
			return
		}
	}
	accounts, err := agent.ListProviderAccounts(origin.OwnerID)
	if err != nil {
		return
	}
	proof := storageDiagnosis{Kind: "not_established", InstructionsHash: task.LaunchInstructionsHash(d.Instructions), StartedAt: *a.StartedAt, FinishedAt: *a.FinishedAt}
	for _, account := range accounts {
		if ctx.Err() != nil {
			return
		}
		if account.Legacy {
			continue
		}
		home, err := agent.ProviderAccountHome(origin.OwnerID, account.ID)
		if err != nil || home == "" {
			continue
		}
		if digest := runner.HistoricalProviderStorageEvidence(home, filepath.Join(s.cfg.WorkspacesDir, d.ID), *a.StartedAt, *a.FinishedAt); digest != "" {
			proof.Kind = "storage_exhausted"
			proof.EvidenceHash = digest
			break
		}
	}
	raw, _ := json.Marshal(proof)
	event := &task.TaskActivity{ID: "storage-diagnosis:" + a.ID, TaskID: d.ID, AttemptID: &a.ID, ActivityType: storageDiagnosisActivity, Author: storageDiagnosisAuthor, Message: string(raw), CreatedAt: time.Now().UTC()}
	if s.taskSvc.Store().CreateActivity(ctx, event) == nil {
		d.Activities = append(d.Activities, event)
	}
}
