package agent

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

// Resolve identity only within this account's explicit private provider profile.
// Failure to read identity must not prevent execution or invent an email address.
func lookupGoogleAccountEmail(ctx context.Context, home string, client *http.Client) string {
	if home == "" {
		return ""
	}
	var access string
	for _, path := range []string{AntigravityCredentialsFilePath(home), AntigravityLegacyCredentialsFilePath(home)} {
		raw, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		var value struct {
			AccessToken string `json:"access_token"`
			Token       struct {
				AccessToken string `json:"access_token"`
			} `json:"token"`
		}
		if json.Unmarshal(raw, &value) == nil {
			access = value.AccessToken
			if access == "" {
				access = value.Token.AccessToken
			}
			if access != "" {
				break
			}
		}
	}
	if access == "" || strings.ContainsAny(access, "\r\n") {
		return ""
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://openidconnect.googleapis.com/v1/userinfo", nil)
	if err != nil {
		return ""
	}
	req.Header.Set("Authorization", "Bearer "+access)
	// Never forward an account token across a redirect, including with injected clients.
	safe := *client
	safe.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	response, err := safe.Do(req)
	if err != nil {
		return ""
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return ""
	}
	var identity struct {
		Email    string `json:"email"`
		Verified bool   `json:"email_verified"`
	}
	if json.NewDecoder(io.LimitReader(response.Body, 16384)).Decode(&identity) != nil || !identity.Verified || !strings.Contains(identity.Email, "@") || strings.ContainsAny(identity.Email, "\r\n") {
		return ""
	}
	return identity.Email
}
