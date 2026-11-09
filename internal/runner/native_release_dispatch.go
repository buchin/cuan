package runner

import (
	"context"
	"errors"
	"log"
	"time"

	"github.com/buchin/cuan/internal/project"
	"github.com/buchin/cuan/internal/task"
)

// NativeReleaseExecution is supplied by native backend preparation, never by a
// model. Environment is explicit owner/project execution configuration.
type NativeReleaseExecution struct {
	Workspace   *project.PreparedNativeReleaseWorkspace `json:"workspace"`
	Environment []string                                `json:"environment"`
}

func (r *Runner) runNativeReleaseAttempt(ctx context.Context, claim ClaimTaskResponse) {
	if claim.Attempt == nil || claim.Task == nil {
		return
	}
	req := CompleteAttemptRequest{RunnerID: r.cfg.RunnerID, Status: task.AttemptStatusFailed, ExitCode: 1, ErrorMessage: "Native release workspace preparation is unavailable; no model or release execution was started"}
	if claim.NativeReleaseExecution == nil {
		preparationCtx, cancel := context.WithTimeout(ctx, 45*time.Second)
		var failure string
		claim.NativeReleaseExecution, failure = r.requestNativeReleasePreparation(preparationCtx, claim)
		if failure != "" {
			req.ErrorMessage = failure
		}
		cancel()
	}
	if claim.NativeReleaseExecution != nil {
		environment, closeCaches, cacheErr := prepareNativeReleaseBuildCaches(ctx, claim.NativeReleaseExecution, r.cfg.WorkspacesDir, claim.OwnerID)
		if cacheErr != nil {
			req.ErrorMessage = "Managed release build cache preparation failed; no release process started"
			claim.NativeReleaseExecution = nil
		} else {
			defer closeCaches()
			execution := *claim.NativeReleaseExecution
			execution.Environment = environment
			claim.NativeReleaseExecution = &execution
		}
	}
	if claim.NativeReleaseExecution != nil {
		executeCtx, cancel := context.WithDeadline(ctx, task.AttemptDeadline(claim.Task, claim.Attempt))
		result, err := r.executeNativeReleaseOnce(executeCtx, claim)
		if errors.Is(err, errNativeReleaseActive) {
			cancel()
			log.Printf("Native release remains owned by its active worker; duplicate dispatch stopped")
			return
		}
		if err != nil {
			req.ErrorMessage = "Native release execution was not confirmed; reconcile saved process and current release before retrying"
		} else {
			req.NativeReleaseProcess = &result
			req.ExitCode = result.ExitCode
			if result.ExitCode == 0 && result.Error == "" && result.SourceUnchanged {
				if r.verifyRegisteredPublication(executeCtx, claim, result.Commit, result.FinishedAt) {
					req.Status = task.AttemptStatusSucceeded
					req.LiveDeploymentConfirmed = true
					req.ErrorMessage = ""
				} else {
					req.ErrorMessage = "Native release process finished; independent production observation is still missing"
				}
			} else {
				req.ErrorMessage = "Native release process did not finish successfully; current release must be reconciled before retrying"
			}
		}
		cancel()
	}
	completionCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 15*time.Second)
	defer cancel()
	if err := r.deliverCompletion(completionCtx, claim.Attempt.ID, claim.Task.ID, req); err != nil {
		log.Printf("Native release completion retained for delivery: %v", err)
	}
}
