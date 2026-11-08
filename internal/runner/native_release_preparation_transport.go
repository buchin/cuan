package runner

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"

	"github.com/buchin/cuan/internal/task"
)

func (r *Runner) requestNativeReleasePreparation(ctx context.Context, claim ClaimTaskResponse) (*NativeReleaseExecution, string) {
	if r.cfg.RunnerToken == "" || claim.Task == nil || claim.Attempt == nil || claim.NativeRelease == nil {
		return nil, ""
	}
	response, err := r.doRequest(ctx, http.MethodPost, "/api/v1/runner/attempts/"+url.PathEscape(claim.Attempt.ID)+"/release/prepare", map[string]string{"task_id": claim.Task.ID, "runner_id": r.cfg.RunnerID})
	if err != nil {
		return nil, ""
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		var failure struct {
			Code string `json:"code"`
		}
		decoder := json.NewDecoder(io.LimitReader(response.Body, 2048))
		if response.StatusCode == http.StatusConflict && decoder.Decode(&failure) == nil && decoder.Decode(new(any)) == io.EOF && failure.Code == "source_merge_conflict" {
			return nil, task.NativeReleaseMergeConflictMessage
		}
		return nil, ""
	}
	var preparation NativeReleaseExecution
	decoder := json.NewDecoder(io.LimitReader(response.Body, 256<<10))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&preparation) != nil || decoder.Decode(&struct{}{}) != io.EOF || preparation.Workspace == nil || preparation.Environment == nil || preparation.Workspace.SourceCommit != claim.NativeRelease.SourceCommit || preparation.Workspace.Branch != claim.NativeRelease.TargetBranch || preparation.Workspace.ManifestHash != claim.NativeRelease.ManifestHash {
		return nil, ""
	}
	return &preparation, ""
}
