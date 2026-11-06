package diagnostics

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
)

// An operator can verify an already-running old worker without restarting it.
// New native runners request the same observation automatically at completion.
func runTaskVerifyPublication(ctx context.Context, args []string, out, errOut io.Writer) int {
	if len(args) != 1 || strings.TrimSpace(args[0]) == "" {
		fmt.Fprintln(errOut, "Usage: cuan task verify-publication <task-id>")
		return 2
	}
	if taskSessionOwnerWriteError() != nil {
		fmt.Fprintln(errOut, "Publication verification uses the native runner or Owner connection.")
		return 1
	}
	client := GetCuanClient()
	if client == nil || !client.IsConfigured() {
		fmt.Fprintln(errOut, "Configure the Cuan connection first.")
		return 1
	}
	response, err := client.doRequest(ctx, http.MethodGet, "/api/v1/tasks/"+url.PathEscape(args[0]), nil)
	if err != nil {
		fmt.Fprintln(errOut, "Task state could not be confirmed.")
		return 1
	}
	defer response.Body.Close()
	if response.StatusCode != 200 {
		fmt.Fprintf(errOut, "Task state unavailable (HTTP %d).\n", response.StatusCode)
		return 1
	}
	var detail struct {
		ID      string `json:"id"`
		Commit  string `json:"commit_sha"`
		Attempt struct {
			ID     string `json:"id"`
			Runner string `json:"runner_id"`
			Status string `json:"status"`
		} `json:"latest_attempt"`
	}
	if json.NewDecoder(io.LimitReader(response.Body, 4<<20)).Decode(&detail) != nil || detail.ID != args[0] || detail.Attempt.Status != "running" || detail.Attempt.ID == "" || detail.Attempt.Runner == "" || !regexp.MustCompile(`^[0-9a-f]{40}$`).MatchString(detail.Commit) {
		fmt.Fprintln(errOut, "A running attempt with an exact result revision is required.")
		return 1
	}
	response, err = client.doRequest(ctx, http.MethodPost, "/api/v1/runner/attempts/"+url.PathEscape(detail.Attempt.ID)+"/publication/verify", map[string]string{"task_id": detail.ID, "runner_id": detail.Attempt.Runner, "commit_sha": detail.Commit})
	if err != nil {
		fmt.Fprintln(errOut, "Publication verification was not confirmed. Inspect the same task before retrying.")
		return 1
	}
	defer response.Body.Close()
	if response.StatusCode != 200 {
		fmt.Fprintf(errOut, "Publication not verified (HTTP %d); no deployment requested.\n", response.StatusCode)
		return 1
	}
	var result struct {
		Verified bool `json:"verified"`
		Receipt  struct {
			Task        string `json:"task_id"`
			Attempt     string `json:"attempt_id"`
			Commit      string `json:"commit_sha"`
			Release     string `json:"release_sha"`
			Health      string `json:"health"`
			Environment string `json:"environment"`
			Version     int    `json:"version"`
		} `json:"receipt"`
	}
	if json.NewDecoder(io.LimitReader(response.Body, 4096)).Decode(&result) != nil || !result.Verified || result.Receipt.Task != detail.ID || result.Receipt.Attempt != detail.Attempt.ID || result.Receipt.Commit != detail.Commit || result.Receipt.Release != detail.Commit || result.Receipt.Health != "healthy" || result.Receipt.Environment != "production" || result.Receipt.Version != 1 {
		fmt.Fprintln(errOut, "Invalid publication observation; task acceptance unchanged.")
		return 1
	}
	returnCode := json.NewEncoder(out).Encode(map[string]any{"verified": true, "task_id": detail.ID, "attempt_id": detail.Attempt.ID, "commit_sha": detail.Commit, "deployment_requested": false, "task_accepted": false})
	if returnCode != nil {
		return 1
	}
	return 0
}
