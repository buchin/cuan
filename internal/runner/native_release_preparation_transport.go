package runner

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
)

func (r *Runner) requestNativeReleasePreparation(ctx context.Context, claim ClaimTaskResponse) *NativeReleaseExecution {
	if r.cfg.RunnerToken == "" || claim.Task == nil || claim.Attempt == nil || claim.NativeRelease == nil {
		return nil
	}
	response, err := r.doRequest(ctx, http.MethodPost, "/api/v1/runner/attempts/"+url.PathEscape(claim.Attempt.ID)+"/release/prepare", map[string]string{"task_id": claim.Task.ID, "runner_id": r.cfg.RunnerID})
	if err != nil {
		return nil
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil
	}
	var preparation NativeReleaseExecution
	decoder := json.NewDecoder(io.LimitReader(response.Body, 256<<10))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&preparation) != nil || decoder.Decode(&struct{}{}) != io.EOF || preparation.Workspace == nil || preparation.Environment == nil || preparation.Workspace.SourceCommit != claim.NativeRelease.SourceCommit || preparation.Workspace.Branch != claim.NativeRelease.TargetBranch || preparation.Workspace.ManifestHash != claim.NativeRelease.ManifestHash {
		return nil
	}
	return &preparation
}
