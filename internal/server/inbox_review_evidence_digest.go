package server

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/buchin/cuan/internal/executionpolicy"
	"github.com/buchin/cuan/internal/task"
)

// Evidence revisions, unlike task status or timestamps, justify one new review.
func inboxReviewEvidenceDigest(d *task.TaskDetailResponse) string {
	return inboxReviewEvidenceDigestForPublicationPolicy(d, inboxPublicationReviewPolicy)
}

func inboxReviewEvidenceDigests(d *task.TaskDetailResponse) []string {
	if d == nil {
		return nil
	}
	seen := make(map[string]bool)
	var result []string
	add := func(digest string) {
		if digest != "" && !seen[digest] {
			seen[digest] = true
			result = append(result, digest)
		}
	}

	add(inboxReviewEvidenceDigest(d))
	add(inboxReviewEvidenceDigestForPublicationPolicy(d, previousPublicationReviewPolicy))

	if d.LatestAttempt != nil {
		var pubObs []*task.TaskActivity
		var otherActivities []*task.TaskActivity
		for _, a := range d.Activities {
			if a != nil && a.TaskID == d.ID && a.AttemptID != nil && *a.AttemptID == d.LatestAttempt.ID && a.ActivityType == publicationConfigurationActivity && a.Author == "coordinator" {
				pubObs = append(pubObs, a)
			} else {
				otherActivities = append(otherActivities, a)
			}
		}
		if len(pubObs) > 0 {
			copyNoPub := *d
			copyNoPub.Activities = otherActivities
			add(inboxReviewEvidenceDigest(&copyNoPub))
			add(inboxReviewEvidenceDigestForPublicationPolicy(&copyNoPub, previousPublicationReviewPolicy))

			for _, obs := range pubObs {
				copyObs := *d
				copyObs.Activities = append(append([]*task.TaskActivity{}, otherActivities...), obs)
				add(inboxReviewEvidenceDigest(&copyObs))
				add(inboxReviewEvidenceDigestForPublicationPolicy(&copyObs, previousPublicationReviewPolicy))
			}
		}
	}

	return result
}

func inboxReviewEvidenceDigestMatches(d *task.TaskDetailResponse, digest string) bool {
	for _, candidate := range inboxReviewEvidenceDigests(d) {
		if candidate == digest {
			return true
		}
	}
	return false
}

func inboxReviewEvidenceDigestForPublicationPolicy(d *task.TaskDetailResponse, publicationPolicy string) string {
	parts := []any{"owner-continuation-budget-v2", d.Summary, d.CompleteNote, d.DoDChecklist, d.TestSummary}
	if d.ExecutionCategory == task.ExecutionCategoryCode {
		parts = append(parts, inboxIndependentCodeReviewPolicy, publicationPolicy)
	}
	if nativePHPPrerequisiteFailure(d) != "" {
		parts = append(parts, inboxPrerequisiteReviewPolicy)
	}
	if nativeProjectCheckFailure(d) != "" {
		parts = append(parts, inboxProjectCheckFailurePolicy)
	}
	parts = append(parts, task.AttemptChangeEvidence(d))
	if audit := scopeAcceptanceReviewData(d); audit != nil {
		parts = append(parts, audit)
	}
	if observed := publicationConfigurationObservation(d); observed != nil {
		parts = append(parts, observed.Message)
	}
	if event, _ := githubObservation(d); event != nil {
		parts = append(parts, event.Message)
	}
	if observation := failedRepositoryObservation(d, true); observation != nil {
		parts = append(parts, observation)
	}
	// Acceptance may replace the user-facing summary. Machine evidence remains
	// immutable; that presentation change must not invalidate its visual review.
	if evidence := inboxReportEvidence(d); evidence.Provenance == "current_runner_finalization" {
		parts[1], parts[2] = evidence.Summary, evidence.Verification
	}
	if d.LatestAttempt != nil {
		for _, a := range d.Activities {
			if a != nil && a.TaskID == d.ID && a.AttemptID != nil && *a.AttemptID == d.LatestAttempt.ID && a.ActivityType == recoveredVerificationArtifactsActivity && a.Author == "coordinator" {
				parts = append(parts, a.Message)
			}
		}
	}
	raw, _ := json.Marshal(parts)
	return fmt.Sprintf("%x", sha256.Sum256(raw))
}

func inboxReviewEvidenceChanged(d *task.TaskDetailResponse, previous string) bool {
	const marker = "Review evidence digest: "
	if i := strings.Index(previous, marker); i >= 0 {
		saved := strings.Fields(previous[i+len(marker):])
		return len(saved) > 0 && !inboxReviewEvidenceDigestMatches(d, saved[0])
	}
	// Legacy reviews persisted the bounded report directly, without a digest.
	return strings.TrimSpace(d.CompleteNote) != "" && !strings.Contains(previous, truncateProjectText(d.CompleteNote, 6000))
}

// A prose report or a marker from an older attempt cannot replace runner evidence.
func (s *Server) inboxReviewRepositoryScope(ctx context.Context, d *task.TaskDetailResponse) (executionpolicy.PublicationPolicy, bool, error) {
	policy, err := s.taskPublicationPolicy(ctx, d)
	if err != nil {
		return policy, false, err
	}
	if !policy.ReadOnlyRepository {
		return policy, true, nil
	}
	if d.LatestAttempt == nil {
		return policy, false, nil
	}
	for _, act := range d.Activities {
		if act != nil && act.ActivityType == "repository_scope_verified" && act.NewStatus == "read_only" && act.AttemptID != nil && *act.AttemptID == d.LatestAttempt.ID && act.Author == d.LatestAttempt.RunnerID {
			return policy, true, nil
		}
	}
	return policy, false, nil
}
