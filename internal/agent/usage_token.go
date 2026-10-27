package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// Public installed-application OAuth client used by CLI tools. These are
// application identifiers, not user credentials. Refresh tokens remain private.
const usageOAuthClientID = "1071006060591-tmhssin2h21lcre235vtolojh4g403ep.apps.googleusercontent.com"
const usageOAuthClientSecret = "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"

var refreshAttemptMu sync.Mutex
var refreshAttempts = map[string]time.Time{}

var usageOAuthURL = "https://oauth2.googleapis.com/token"

func accountUsageAccessToken(ctx context.Context, owner, id string, force bool) (string, error) {
	home, err := ProviderAccountHome(owner, id)
	if err != nil {
		return "", err
	}
	load := func() (string, []byte, map[string]any, map[string]any, error) {
		for _, file := range []string{AntigravityCredentialsFilePath(home), AntigravityLegacyCredentialsFilePath(home)} {
			raw, err := os.ReadFile(file)
			if err != nil {
				continue
			}
			var root map[string]any
			if json.Unmarshal(raw, &root) != nil {
				continue
			}
			tokenMap := root
			for _, key := range []string{"token", "tokens"} {
				if child, ok := root[key].(map[string]any); ok {
					tokenMap = child
					break
				}
			}
			return file, raw, root, tokenMap, nil
		}
		return "", nil, nil, nil, errors.New("Account credentials unavailable")
	}
	_, _, _, tokens, err := load()
	if err != nil {
		return "", err
	}
	expiry, _ := time.Parse(time.RFC3339Nano, usageString(tokens["expiry"]))
	token := usageAccessToken(tokens)
	needsRefresh := force || token == "" || (!expiry.IsZero() && time.Now().Add(time.Minute).After(expiry))
	if !needsRefresh {
		return token, nil
	}
	release, err := TryLeaseProviderAccountRefresh(owner, id)
	if err != nil {
		return "", errors.New("Account is busy; token refresh deferred")
	}
	defer release()
	// Read again under the execution lease; never overwrite a running CLI's tokens.
	file, before, root, tokens, err := load()
	if err != nil {
		return "", err
	}
	expiry, _ = time.Parse(time.RFC3339Nano, usageString(tokens["expiry"]))
	token = usageAccessToken(tokens)
	if !force && token != "" && !expiry.IsZero() && time.Now().Add(time.Minute).Before(expiry) {
		return token, nil
	}
	refresh := usageString(tokens["refresh_token"])
	if refresh == "" {
		return "", errors.New("Account token cannot be refreshed automatically")
	}
	// A rejected grant or repeated unauthorized response must not create a
	// refresh storm. This bound applies across concurrent usage requests too.
	refreshAttemptMu.Lock()
	key := owner + ":" + id
	if last := refreshAttempts[key]; !last.IsZero() && time.Since(last) < time.Minute {
		refreshAttemptMu.Unlock()
		return "", errors.New("Token refresh is cooling down. Try again later")
	}
	if len(refreshAttempts) >= 128 {
		for key, stamp := range refreshAttempts {
			if time.Since(stamp) > time.Minute {
				delete(refreshAttempts, key)
			}
		}
	}
	refreshAttempts[key] = time.Now()
	refreshAttemptMu.Unlock()
	form := url.Values{"client_id": {usageOAuthClientID}, "client_secret": {usageOAuthClientSecret}, "grant_type": {"refresh_token"}, "refresh_token": {refresh}}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, usageOAuthURL, strings.NewReader(form.Encode()))
	if err != nil {
		return "", errors.New("Token refresh unavailable")
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response, err := modelUsageClient.Do(req)
	if err != nil {
		return "", errors.New("Token refresh unavailable")
	}
	defer response.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(response.Body, 128*1024))
	if err != nil || response.StatusCode != 200 {
		return "", errors.New("Account token refresh was rejected or unavailable")
	}
	var updated struct {
		Access  string `json:"access_token"`
		Refresh string `json:"refresh_token"`
		Expires int64  `json:"expires_in"`
		Type    string `json:"token_type"`
	}
	if json.Unmarshal(raw, &updated) != nil || len(updated.Access) < 8 || updated.Expires <= 0 || updated.Expires > 365*24*3600 {
		return "", errors.New("Invalid token refresh response")
	}
	current, err := os.ReadFile(file)
	if err != nil || !bytes.Equal(before, current) {
		return "", errors.New("Account credentials changed during refresh; retry later")
	}
	tokens["access_token"] = updated.Access
	tokens["expiry"] = time.Now().UTC().Add(time.Duration(updated.Expires) * time.Second).Format(time.RFC3339Nano)
	if updated.Refresh != "" {
		tokens["refresh_token"] = updated.Refresh
	}
	if updated.Type != "" {
		tokens["token_type"] = updated.Type
	}
	data, err := json.MarshalIndent(root, "", "  ")
	if err != nil {
		return "", errors.New("Could not save refreshed account token")
	}
	// Keep atomic writes within this private profile. Do not follow a credential
	// symlink to another profile while refreshing an explicitly selected account.
	if info, err := os.Lstat(file); err != nil || !info.Mode().IsRegular() {
		return "", errors.New("Credential file cannot be refreshed safely")
	}
	temp, err := os.CreateTemp(filepath.Dir(file), ".token-refresh-*")
	if err != nil {
		return "", errors.New("Could not save refreshed account token")
	}
	defer os.Remove(temp.Name())
	if err = temp.Chmod(0600); err == nil {
		_, err = temp.Write(append(data, '\n'))
	}
	if err == nil {
		err = temp.Sync()
	}
	closeErr := temp.Close()
	if err == nil {
		err = closeErr
	}
	if err == nil {
		err = os.Rename(temp.Name(), file)
	}
	if err != nil {
		return "", errors.New("Could not save refreshed account token")
	}
	return updated.Access, nil
}
func usageString(v any) string { value, _ := v.(string); return strings.TrimSpace(value) }
