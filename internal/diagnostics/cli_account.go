package diagnostics

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/buchin/cuan/internal/agent"
)

// Account diagnostics use the same authenticated owner boundary as Settings.
// Typed DTOs omit credential contents, profile paths and unknown server fields.
func (c *CuanClient) ListProviderAccounts(ctx context.Context) ([]*agent.ProviderAccount, error) {
	var data struct {
		Accounts []*agent.ProviderAccount `json:"accounts"`
	}
	if err := c.readAccountJSON(ctx, "/api/v1/provider-accounts", &data); err != nil {
		return nil, err
	}
	if data.Accounts == nil {
		data.Accounts = []*agent.ProviderAccount{}
	}
	return data.Accounts, nil
}
func (c *CuanClient) GetProviderAccountUsage(ctx context.Context, id string) (agent.AccountModelUsage, error) {
	var data agent.AccountModelUsage
	if strings.TrimSpace(id) == "" || strings.ContainsAny(id, "/\\?#") {
		return data, fmt.Errorf("a valid account ID is required")
	}
	err := c.readAccountJSON(ctx, "/api/v1/provider-accounts/"+url.PathEscape(id)+"/usage", &data)
	return data, err
}

func (c *CuanClient) ListProviderAccountModels(ctx context.Context, id string) ([]agent.ModelInfo, error) {
	var data struct {
		Models []agent.ModelInfo `json:"models"`
	}
	if strings.TrimSpace(id) == "" || strings.ContainsAny(id, "/\\?#") {
		return nil, fmt.Errorf("a valid account ID is required")
	}
	err := c.readAccountJSON(ctx, "/api/v1/provider-accounts/"+url.PathEscape(id)+"/models", &data)
	return data.Models, err
}

// SetProviderAccountModel changes only the saved model selection, preserving
// the account label and its separate readiness/fallback policies.
func (c *CuanClient) SetProviderAccountModel(ctx context.Context, id, model string) (*agent.ProviderAccount, error) {
	if err := taskSessionOwnerWriteError(); err != nil {
		return nil, err
	}
	accounts, err := c.ListProviderAccounts(ctx)
	if err != nil {
		return nil, err
	}
	var current *agent.ProviderAccount
	for _, account := range accounts {
		if account.ID == id {
			current = account
			break
		}
	}
	if current == nil || !current.CredentialsPresent || current.Legacy {
		return nil, fmt.Errorf("choose an owned connected account")
	}
	if model != "" {
		models, err := c.ListProviderAccountModels(ctx, id)
		if err != nil {
			return nil, err
		}
		found := false
		for _, candidate := range models {
			found = found || candidate.ID == model
		}
		if !found {
			return nil, fmt.Errorf("choose a model from this account's native catalog")
		}
	}
	resp, err := c.doRequest(ctx, http.MethodPatch, "/api/v1/provider-accounts/"+url.PathEscape(id), map[string]string{"label": current.Label, "model_id": model})
	if err != nil {
		return nil, fmt.Errorf("could not update account model")
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("account model update failed (HTTP %d)", resp.StatusCode)
	}
	var data struct {
		Account *agent.ProviderAccount `json:"account"`
	}
	if json.NewDecoder(io.LimitReader(resp.Body, 2<<20)).Decode(&data) != nil || data.Account == nil || data.Account.ID != id || data.Account.ModelID != model || data.Account.Label != current.Label {
		return nil, fmt.Errorf("account model update could not be confirmed; read account settings before retrying")
	}
	return data.Account, nil
}

// Verify probes the existing sign-in and updates readiness, never accepting terms or changing policy.
func (c *CuanClient) VerifyProviderAccount(ctx context.Context, id string) (*agent.ProviderAccount, error) {
	if strings.TrimSpace(id) == "" || strings.ContainsAny(id, "/\\?#") {
		return nil, fmt.Errorf("a valid account ID is required")
	}
	var data struct {
		Account *agent.ProviderAccount `json:"account"`
	}
	err := c.accountJSON(ctx, http.MethodPost, "/api/v1/provider-accounts/"+url.PathEscape(id)+"/verify", &data)
	return data.Account, err
}
func (c *CuanClient) readAccountJSON(ctx context.Context, endpoint string, out any) error {
	return c.accountJSON(ctx, http.MethodGet, endpoint, out)
}
func (c *CuanClient) accountJSON(ctx context.Context, method, endpoint string, out any) error {
	if !c.IsConfigured() {
		return fmt.Errorf("configure Cuan API URL and auth token first")
	}
	resp, err := c.doRequest(ctx, method, endpoint, nil)
	if err != nil {
		return fmt.Errorf("account diagnostics request failed")
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("account diagnostics failed (HTTP %d)", resp.StatusCode)
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 2<<20)).Decode(out); err != nil {
		return fmt.Errorf("invalid account diagnostics response")
	}
	return nil
}
func RunAccountCLI(args []string, out, errOut io.Writer) int {
	const help = `Cuan account diagnostics (authenticated as the configured user)
Usage:
  cuan account list
  cuan account usage ACCOUNT_ID
  cuan account models ACCOUNT_ID
  cuan account model ACCOUNT_ID MODEL_ID|auto
  cuan account verify ACCOUNT_ID
Output is JSON metadata only. Credentials and native profile paths are never returned.
Verify probes an existing sign-in and updates readiness without accepting terms.
Usage reads the provider's availability evidence; it does not change model/fallback policy.`
	if len(args) == 0 || args[0] == "--help" || args[0] == "help" {
		fmt.Fprintln(out, help)
		return 0
	}
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	var result any
	var err error
	switch {
	case args[0] == "list" && len(args) == 1:
		var accounts []*agent.ProviderAccount
		accounts, err = GetCuanClient().ListProviderAccounts(ctx)
		result = map[string]any{"accounts": accounts}
	case args[0] == "usage" && len(args) == 2:
		result, err = GetCuanClient().GetProviderAccountUsage(ctx, args[1])
	case args[0] == "models" && len(args) == 2:
		result, err = GetCuanClient().ListProviderAccountModels(ctx, args[1])
	case args[0] == "model" && len(args) == 3:
		model := args[2]
		if model == "auto" {
			model = ""
		}
		result, err = GetCuanClient().SetProviderAccountModel(ctx, args[1], model)
	case args[0] == "verify" && len(args) == 2:
		result, err = GetCuanClient().VerifyProviderAccount(ctx, args[1])
	default:
		fmt.Fprintln(errOut, help)
		return 2
	}
	if err != nil {
		fmt.Fprintln(errOut, err)
		return 1
	}
	enc := json.NewEncoder(out)
	enc.SetIndent("", "  ")
	if err := enc.Encode(result); err != nil {
		fmt.Fprintln(errOut, "could not write account diagnostics")
		return 1
	}
	return 0
}
