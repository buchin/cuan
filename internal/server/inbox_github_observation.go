package server

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"strings"
	"time"

	"github.com/buchin/cuan/internal/db"
	"github.com/buchin/cuan/internal/git"
	"github.com/buchin/cuan/internal/task"
)

const githubObservationActivity = "github_connection_observed"

type githubConnectionObservation struct {
	State                    string `json:"state"`
	Account                  string `json:"account,omitempty"`
	ConnectionFingerprint    string `json:"connection_fingerprint,omitempty"`
	InstructionsHash         string `json:"instructions_hash"`
	RepositoryAccessVerified bool   `json:"repository_access_verified"`
}

func githubObservation(d *task.TaskDetailResponse) (*task.TaskActivity, *githubConnectionObservation) {
	if d == nil || d.LatestAttempt == nil {
		return nil, nil
	}
	var latest *task.TaskActivity
	var observed *githubConnectionObservation
	for _, event := range d.Activities {
		if event == nil || event.TaskID != d.ID || event.AttemptID == nil || *event.AttemptID != d.LatestAttempt.ID || event.ActivityType != githubObservationActivity || event.Author != storageDiagnosisAuthor {
			continue
		}
		var proof githubConnectionObservation
		if json.Unmarshal([]byte(event.Message), &proof) != nil || proof.InstructionsHash != task.LaunchInstructionsHash(d.Instructions) || (proof.State != "connected" && proof.State != "disconnected" && proof.State != "unavailable") {
			continue
		}
		if latest == nil || event.CreatedAt.After(latest.CreatedAt) || (event.CreatedAt.Equal(latest.CreatedAt) && event.ID > latest.ID) {
			latest, observed = event, &proof
		}
	}
	return latest, observed
}

func githubReviewObservation(d *task.TaskDetailResponse) map[string]any {
	_, proof := githubObservation(d)
	if proof == nil {
		return nil
	}
	return map[string]any{"state": proof.State, "account": proof.Account, "repository_access_verified": false, "provenance": "saved_owner_connection"}
}

func (s *Server) registeredReviewProject(ctx context.Context, d *task.TaskDetailResponse) map[string]any {
	if s.projectSvc == nil || d == nil {
		return nil
	}
	p, err := s.projectSvc.Get(ctx, d.ProjectID)
	if err != nil || p == nil {
		return nil
	}
	return map[string]any{"id": p.ID, "name": p.Name, "repository": p.RepoURL, "archived": p.IsArchived}
}

// Only a previously reviewed GitHub prerequisite qualifies. This observes the
// registered owner's connection, never host gh auth, and cannot launch work.
func (s *Server) observeInboxGitHubConnection(ctx context.Context, d *task.TaskDetailResponse, origin db.InboxRequest) {
	if s.cfg == nil || d == nil || d.Status != task.TaskStatusBlocked || d.LatestAttempt == nil || !s.reviewSnapshotRecoveryAuthorized(ctx, d, origin) {
		return
	}
	claimed := false
	for _, event := range d.Activities {
		if event == nil || event.AttemptID == nil || *event.AttemptID != d.LatestAttempt.ID || event.ActivityType != reviewAssessmentActivity || event.Author != "coordinator" {
			continue
		}
		var record reviewAssessmentRecord
		if json.Unmarshal([]byte(event.Message), &record) == nil && record.InstructionsHash == task.LaunchInstructionsHash(d.Instructions) && record.Assessment.Decision == "blocked" && (record.Assessment.RemainingWork == "external" || record.Assessment.RemainingWork == "") && strings.Contains(strings.ToLower(record.Assessment.Finding), "github") {
			claimed = true
		}
	}
	if !claimed {
		return
	}
	_, authorized, err := s.authorizeSavedTaskLaunch(ctx, task.TaskLaunch{OwnerID: origin.OwnerID, RoomID: origin.RoomID, SourceRequestID: origin.RequestID, CreatedAt: time.Now().UTC()}, d)
	if err != nil || !authorized {
		return
	}
	path, err := githubConnectionPath(origin.OwnerID)
	if err != nil {
		return
	}
	githubConnectionMu.Lock()
	account, token, err := git.LoadConnection(path, s.cfg.AuthSecret)
	githubConnectionMu.Unlock()
	proof := githubConnectionObservation{State: "connected", InstructionsHash: task.LaunchInstructionsHash(d.Instructions)}
	if os.IsNotExist(err) {
		proof.State = "disconnected"
	} else if err != nil {
		proof.State = "unavailable"
	} else {
		proof.Account = account.Login
		hash := sha256.Sum256([]byte("cuan-owner-connection\x00" + origin.OwnerID + "\x00" + token))
		proof.ConnectionFingerprint = hex.EncodeToString(hash[:])
	}
	previous, saved := githubObservation(d)
	if saved != nil && *saved == proof {
		return
	}
	raw, _ := json.Marshal(proof)
	predecessor := ""
	if previous != nil {
		predecessor = previous.ID
	}
	event := &task.TaskActivity{ID: delegationMessageID("github-observation:" + d.LatestAttempt.ID + ":" + predecessor + ":" + string(raw)), TaskID: d.ID, AttemptID: &d.LatestAttempt.ID, ActivityType: githubObservationActivity, Author: storageDiagnosisAuthor, Message: string(raw), CreatedAt: time.Now().UTC()}
	if s.taskSvc.Store().CreateActivity(ctx, event) == nil {
		d.Activities = append(d.Activities, event)
	}
}
