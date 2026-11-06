package runner

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"

	"github.com/buchin/cuan/internal/task"
)

// This asks the native server to observe the registered target independently.
// No target, credential, shell command or agent-supplied receipt is forwarded.
func (r *Runner) verifyRegisteredPublication(ctx context.Context, claim ClaimTaskResponse, commit string) bool {
	if r.cfg.RunnerToken == "" || claim.Task == nil || claim.Attempt == nil {
		return false
	}
	response, err := r.doRequest(ctx, http.MethodPost, "/api/v1/runner/attempts/"+url.PathEscape(claim.Attempt.ID)+"/publication/verify", map[string]string{"task_id": claim.Task.ID, "runner_id": r.cfg.RunnerID, "commit_sha": commit})
	if err != nil {
		return false
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return false
	}
	var result struct {
		Verified bool                   `json:"verified"`
		Receipt  task.DeploymentReceipt `json:"receipt"`
	}
	if json.NewDecoder(io.LimitReader(response.Body, 4097)).Decode(&result) != nil {
		return false
	}
	receipt := result.Receipt
	return result.Verified && receipt.Version == 1 && receipt.TaskID == claim.Task.ID && receipt.AttemptID == claim.Attempt.ID && receipt.CommitSHA == commit && receipt.ReleaseSHA == commit && receipt.Environment == "production" && receipt.Health == "healthy"
}
