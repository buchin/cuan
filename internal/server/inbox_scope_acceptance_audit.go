package server

import (
	"context"
	"strings"
	"time"

	"github.com/buchin/cuan/internal/db"
	"github.com/buchin/cuan/internal/git"
	"github.com/buchin/cuan/internal/task"
)

const legacyScopeAuditReason = "The legacy automatic evaluator demanded Cuan's web/src/features/landing/LandingPage.tsx in this different project, and the accepted branch diff names that path. Re-evaluate the original approved phase and its actual resulting files. Automatic feedback did not authorize unrelated Cuan product pages. Do not infer that a filename alone proves a file must be deleted: inspect the actual scope and changes, preserve unrelated work and reconcile only verified out-of-scope effects. Resource registration requires evidence of the resulting resource, not readiness instructions. Historical accepted status is not feature or scope evidence."

const scopeAuditProgress = "Cuan memeriksa kembali kesesuaian hasil dengan permintaan awal."

func scopeAcceptanceReviewData(d *task.TaskDetailResponse) map[string]any {
	if audit := scopeAcceptanceAudit(d); audit != nil {
		return map[string]any{"policy": "legacy-foreign-page-v1", "reason": legacyScopeAuditReason, "observed_at": audit.CreatedAt}
	}
	return nil
}

func scopeAcceptanceAudit(d *task.TaskDetailResponse) *task.TaskActivity {
	if d == nil || d.LatestAttempt == nil {
		return nil
	}
	for _, e := range d.Activities {
		if e != nil && e.TaskID == d.ID && e.AttemptID != nil && *e.AttemptID == d.LatestAttempt.ID && e.ActivityType == task.ActivityScopeAcceptanceAudit && e.Author == "coordinator" {
			return e
		}
	}
	return nil
}

func (s *Server) reconcileLegacyScopeAcceptances(ctx context.Context) {
	if s.cfg == nil || s.cfg.Environment != "production" || s.cfg.SourceRepositoryURL() == "" || s.taskSvc == nil || s.projectSvc == nil || s.projectStudioRepo == nil {
		return
	}
	now := time.Now().Unix()
	due := s.scopeAuditNext.Load()
	if due > now || !s.scopeAuditNext.CompareAndSwap(due, now+60) {
		return
	}
	reader, ok := s.taskSvc.Store().(task.ScopeAcceptanceAuditStore)
	if !ok {
		return
	}
	ids, err := reader.LegacyScopeAcceptanceTaskIDs(ctx, s.cfg.SourceRepositoryURL())
	if err != nil {
		s.scopeAuditNext.Store(now + 10)
		return
	}
	requests, ok := s.projectStudioRepo.(db.InboxRequestRepository)
	if !ok {
		return
	}
	for _, id := range ids {
		if ctx.Err() != nil {
			return
		}
		d, err := s.taskSvc.GetTaskDetail(ctx, id)
		if err != nil || d == nil || d.Status != task.TaskStatusDone || d.LatestAttempt == nil || scopeAcceptanceAudit(d) != nil {
			continue
		}
		p, err := s.projectSvc.Get(ctx, d.ProjectID)
		if err != nil || p == nil || p.IsArchived {
			continue
		}
		bound, err := s.projectSvc.TaskWorkspace(ctx, d.ProjectID, d.Instructions)
		if err != nil || bound == nil || bound.RepoURL != p.RepoURL || bound.DefaultBranch != p.DefaultBranch {
			continue
		}
		owner, name, err := git.ParseGitHubURL(p.RepoURL)
		if err != nil || strings.EqualFold(owner+"/"+name, s.cfg.SourceRepository) {
			continue
		}
		origins, err := requests.ListInboxRequests(ctx, d.SourceRoomID)
		if err != nil {
			continue
		}
		snapshot := *d
		snapshot.Status = task.TaskStatusReview
		authorized := false
		for _, origin := range origins {
			if origin.MessageID != d.SourceMessageID || origin.OwnerID == "" || origin.State == "cancelled" || origin.State == "canceled" || origin.State == "cancelling" || !s.sameRuntimeAgent(ctx, origin.AgentID, d.AssignedAgent) {
				continue
			}
			if s.reviewSnapshotRecoveryAuthorized(ctx, &snapshot, origin) {
				authorized = true
				break
			}
			if _, err := s.phaseApproval(ctx, d, origin.OwnerID, origin.RoomID); err == nil {
				authorized = true
				break
			}
		}
		if !authorized {
			continue
		}
		changed, err := reader.RequeueScopeAcceptance(ctx, d.ID, d.LatestAttempt.ID, d.UpdatedAt, scopeAuditProgress)
		if err == nil && changed {
			s.publishInbox(d.SourceRoomID)
		}
	}
}
