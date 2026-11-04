package runner

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/buchin/cuan/internal/task"
)

type verificationFinalizationClaimKey struct{}

// No adapter, AI account, project command, workspace preparation, publication or
// agent-report fabrication is involved in this bounded integrity check.
func (r *Runner) runVerificationFinalizer(ctx context.Context, claim ClaimTaskResponse, root string, sourceErr error) {
	proof := claim.Verification
	req := CompleteAttemptRequest{RunnerID: r.cfg.RunnerID, Status: task.AttemptStatusFailed, ExitCode: 1, RoomID: claim.Task.SourceRoomID}
	if sourceErr == nil {
		final, err := repositoryFingerprint(ctx, root)
		req.RepositoryVerification = &RepositoryVerification{Baseline: proof.RepositoryBaseline, Final: final}
		if err == nil && req.RepositoryVerification.Unchanged() {
			req.Status = task.AttemptStatusSucceeded
			req.ExitCode = 0
			req.VerifiedSourceCommit = proof.SourceCommit
			req.FinalizationManifestHash = proof.ArtifactManifestHash
			req.TestSummary = "Saved verification output finalization: exact source commit and unchanged repository fingerprint verified. Original implementation tests remain bound to their source attempt; original timeout and acceptance requirements are preserved."
		} else {
			req.ErrorMessage = "Saved output finalization could not verify unchanged repository state; existing work was preserved"
		}
	} else {
		req.ErrorMessage = "Saved output finalization source commit or clean repository check failed; existing work was preserved"
	}
	submit, cancel := context.WithTimeout(context.WithoutCancel(ctx), 15*time.Second)
	defer cancel()
	response, err := r.doRequest(submit, http.MethodPost, fmt.Sprintf("/api/v1/runner/attempts/%s/complete", claim.Attempt.ID), req)
	if err != nil {
		log.Printf("[Runner %s] Saved-output finalization completion could not be delivered; server state remains authoritative", r.cfg.RunnerID)
		return
	}
	response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		log.Printf("[Runner %s] Saved-output finalization completion was not accepted (HTTP %d); server state remains authoritative", r.cfg.RunnerID, response.StatusCode)
	}
}
