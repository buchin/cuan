package runner

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/buchin/cuan/internal/git"
)

func (r *Runner) publishTaskExecution(ctx context.Context, capability *TaskAgentCapability, result *CompleteAttemptRequest, branch string) error {
	if capability == nil || capability.API == "" || capability.Token == "" || result.CommitSHA == "" {
		return errors.New("The bound task publication channel or committed result is unavailable")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(capability.API, "/")+"/publish", nil)
	if err != nil {
		return errors.New("The bound task publication channel is invalid")
	}
	req.Header.Set("Authorization", "Bearer "+capability.Token)
	response, err := r.httpClient.Do(req)
	if err != nil {
		return errors.New("The task publication receipt was not received")
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("The project publication channel returned status %d", response.StatusCode)
	}
	var receipt struct {
		Commit string                 `json:"commit_sha"`
		Branch string                 `json:"branch"`
		Pushed bool                   `json:"pushed"`
		PR     *git.PullRequestResult `json:"pull_request"`
	}
	decoder := json.NewDecoder(io.LimitReader(response.Body, 64<<10))
	if decoder.Decode(&receipt) != nil || decoder.Decode(new(any)) != io.EOF || !receipt.Pushed || receipt.Commit != result.CommitSHA || receipt.Branch != branch {
		return errors.New("The publication receipt does not confirm this exact task branch and commit")
	}
	if receipt.PR != nil {
		result.PRURL, result.PRNumber = receipt.PR.HTMLURL, receipt.PR.Number
	}
	return nil
}
