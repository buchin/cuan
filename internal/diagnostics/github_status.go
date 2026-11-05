package diagnostics

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/buchin/cuan/internal/taskcli"
	"io"
	"net/http"
)

// The saved Cuan connection is distinct from a host's gh login and from actual
// repository permission. Never inspect ambient provider credentials here.
type GitHubConnectionStatus struct {
	Connected bool `json:"connected"`
	Account   *struct {
		ID    int64  `json:"id,omitempty"`
		Login string `json:"login"`
		Name  string `json:"name,omitempty"`
	} `json:"account,omitempty"`
}

func (c *CuanClient) GitHubConnection(ctx context.Context) (*GitHubConnectionStatus, error) {
	if boundTaskSession() {
		var out, errs bytes.Buffer
		if taskcli.Run([]string{"get"}, &out, &errs) != 0 {
			return nil, fmt.Errorf("could not read GitHub connection through this task capability")
		}
		var current struct {
			GitHub struct {
				State   string          `json:"state"`
				Account json.RawMessage `json:"account"`
			} `json:"github_connection"`
		}
		if json.Unmarshal(out.Bytes(), &current) != nil || (current.GitHub.State != "connected" && current.GitHub.State != "disconnected") {
			return nil, fmt.Errorf("this task's GitHub connection status is unavailable; no Owner credential fallback was used")
		}
		result := GitHubConnectionStatus{Connected: current.GitHub.State == "connected"}
		if result.Connected && (json.Unmarshal(current.GitHub.Account, &result.Account) != nil || result.Account == nil || result.Account.Login == "") {
			return nil, fmt.Errorf("invalid scoped GitHub connection status")
		}
		return &result, nil
	}
	if !c.IsConfigured() {
		return nil, fmt.Errorf("configure Cuan API URL and auth token first")
	}
	response, err := c.doRequest(ctx, http.MethodGet, "/api/v1/integrations/github", nil)
	if err != nil {
		return nil, fmt.Errorf("GitHub connection status could not be read")
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub connection status request failed (HTTP %d)", response.StatusCode)
	}
	var wire struct {
		Connected *bool           `json:"connected"`
		Account   json.RawMessage `json:"account"`
	}
	if json.NewDecoder(io.LimitReader(response.Body, 65536)).Decode(&wire) != nil || wire.Connected == nil {
		return nil, fmt.Errorf("invalid GitHub connection status response")
	}
	result := GitHubConnectionStatus{Connected: *wire.Connected}
	if result.Connected && (json.Unmarshal(wire.Account, &result.Account) != nil || result.Account == nil || result.Account.Login == "") {
		return nil, fmt.Errorf("invalid GitHub connection status response")
	}
	if !result.Connected {
		result.Account = nil
	}
	return &result, nil
}
