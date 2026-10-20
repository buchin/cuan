package agent

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"math"
	"net/http"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
)

// Same model quota RPC used by the installed Antigravity CLI and Fural.
// Reading usage never runs a model. Expired OAuth access tokens are refreshed
// atomically under the same exclusive lease used by execution.
var modelUsageClient = &http.Client{Timeout: 3 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
var modelUsageHosts = []string{"https://daily-cloudcode-pa.googleapis.com", "https://cloudcode-pa.googleapis.com"}
var modelIDPattern = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._-]{0,119}$`)

type ModelUsage struct {
	Model     string     `json:"model"`
	Remaining float64    `json:"remaining_percent"`
	ResetAt   *time.Time `json:"reset_at,omitempty"`
}
type AccountModelUsage struct {
	Available bool         `json:"available"`
	CheckedAt time.Time    `json:"checked_at"`
	Models    []ModelUsage `json:"models"`
	Message   string       `json:"message,omitempty"`
}

var usageCacheMu sync.Mutex
var usageCache = map[string]struct {
	at   time.Time
	data AccountModelUsage
}{}

func IsTextModel(id string) bool {
	lower := strings.ToLower(id)
	return modelIDPattern.MatchString(id) && (strings.HasPrefix(lower, "gemini-") || strings.HasPrefix(lower, "claude-")) && !strings.Contains(lower, "image") && !strings.Contains(lower, "embedding") && !strings.Contains(lower, "computer-use")
}
func FetchAccountModelUsage(ctx context.Context, owner, id string) AccountModelUsage {
	result := AccountModelUsage{CheckedAt: time.Now().UTC(), Models: []ModelUsage{}, Message: "Model usage is unavailable. No remaining quota is assumed."}
	a, err := GetProviderAccount(owner, id)
	if err != nil || a.Legacy || !a.CredentialsPresent {
		return result
	}
	usageCtx, cancelUsage := context.WithTimeout(ctx, 8*time.Second)
	defer cancelUsage()
	token, err := accountUsageAccessToken(usageCtx, owner, id, false)
	if err != nil {
		result.Message = "Account usage could not be checked. Token refresh may be pending or require attention."
		return result
	}
	hash := sha256.Sum256([]byte(token))
	key := owner + ":" + id + ":" + hex.EncodeToString(hash[:])
	usageCacheMu.Lock()
	cached, ok := usageCache[key]
	usageCacheMu.Unlock()
	if ok && time.Since(cached.at) < 30*time.Second {
		return cached.data
	}
	requestCtx, cancel := context.WithTimeout(usageCtx, 3*time.Second)
	defer cancel()
	for _, host := range modelUsageHosts {
		req, err := http.NewRequestWithContext(requestCtx, http.MethodPost, host+"/v1internal:fetchAvailableModels", bytes.NewBufferString(`{}`))
		if err != nil {
			continue
		}
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("User-Agent", "cuan/1.0 (Antigravity/0.0.0)")
		response, err := modelUsageClient.Do(req)
		if err != nil {
			continue
		}
		raw, err := io.ReadAll(io.LimitReader(response.Body, 1<<20))
		response.Body.Close()
		if response.StatusCode == 401 {
			fresh, refreshErr := accountUsageAccessToken(usageCtx, owner, id, true)
			if refreshErr != nil || fresh == token {
				break
			}
			token = fresh
			retryReq, err := http.NewRequestWithContext(usageCtx, http.MethodPost, host+"/v1internal:fetchAvailableModels", bytes.NewBufferString(`{}`))
			if err != nil {
				break
			}
			retryReq.Header = req.Header.Clone()
			retryReq.Header.Set("Authorization", "Bearer "+token)
			retry, err := modelUsageClient.Do(retryReq)
			if err != nil {
				break
			}
			raw, err = io.ReadAll(io.LimitReader(retry.Body, 1<<20))
			retry.Body.Close()
			if err != nil || retry.StatusCode != 200 {
				break
			}
			result.Models = parseAccountModelUsage(raw)
			if len(result.Models) > 0 {
				result.Available = true
				result.Message = ""
			}
			break
		}
		if response.StatusCode == 403 {
			break
		}
		if err != nil || response.StatusCode != 200 {
			continue
		}
		result.Models = parseAccountModelUsage(raw)
		if len(result.Models) > 0 {
			result.Available = true
			result.Message = ""
			break
		}
	}
	usageCacheMu.Lock()
	if len(usageCache) >= 128 {
		usageCache = map[string]struct {
			at   time.Time
			data AccountModelUsage
		}{}
	}
	usageCache[key] = struct {
		at   time.Time
		data AccountModelUsage
	}{time.Now(), result}
	usageCacheMu.Unlock()
	return result
}
func usageAccessToken(v map[string]any) string {
	for _, key := range []string{"access_token", "accessToken"} {
		if value, ok := v[key].(string); ok && len(value) >= 8 && !strings.ContainsAny(value, "\r\n") {
			return value
		}
	}
	for _, key := range []string{"token", "tokens"} {
		if child, ok := v[key].(map[string]any); ok {
			if token := usageAccessToken(child); token != "" {
				return token
			}
		}
	}
	return ""
}
func parseAccountModelUsage(raw []byte) []ModelUsage {
	var result struct {
		Models map[string]struct {
			QuotaInfo *struct {
				Remaining *float64 `json:"remainingFraction"`
				Reset     string   `json:"resetTime"`
			} `json:"quotaInfo"`
		} `json:"models"`
	}
	if json.Unmarshal(raw, &result) != nil {
		return nil
	}
	out := []ModelUsage{}
	for id, m := range result.Models {
		if !IsTextModel(id) || m.QuotaInfo == nil || m.QuotaInfo.Remaining == nil {
			continue
		}
		v := *m.QuotaInfo.Remaining
		if math.IsNaN(v) || math.IsInf(v, 0) || v < 0 || v > 1 {
			continue
		}
		item := ModelUsage{Model: id, Remaining: v * 100}
		if reset, err := time.Parse(time.RFC3339, m.QuotaInfo.Reset); err == nil {
			item.ResetAt = &reset
		}
		out = append(out, item)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Remaining != out[j].Remaining {
			return out[i].Remaining > out[j].Remaining
		}
		return out[i].Model < out[j].Model
	})
	return out
}
