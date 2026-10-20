package server

import (
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"github.com/buchin/cuan/internal/git"
	"io"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"
)

type githubOAuthFlow struct {
	Owner, ReturnTo, CookieHash string
	Expires                     time.Time
}

var githubOAuthFlows = struct {
	sync.Mutex
	flows map[string]githubOAuthFlow
}{flows: map[string]githubOAuthFlow{}}
var githubOAuthHTTP = &http.Client{Timeout: 20 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
var githubOAuthTokenURL = "https://github.com/login/oauth/access_token"
var githubOAuthAPIBase = "https://api.github.com"

func githubOAuthConfig() (string, string, string, bool) {
	client := os.Getenv("CUAN_GITHUB_CLIENT_ID")
	secret := os.Getenv("CUAN_GITHUB_CLIENT_SECRET")
	callback := os.Getenv("CUAN_GITHUB_REDIRECT_URL")
	u, err := url.Parse(callback)
	valid := err == nil && u.Host != "" && u.User == nil && u.RawQuery == "" && u.Fragment == "" && u.Path == "/api/v1/integrations/github/oauth/callback" && (u.Scheme == "https" || (u.Scheme == "http" && (u.Hostname() == "localhost" || u.Hostname() == "127.0.0.1")))
	return client, secret, callback, valid && client != "" && secret != ""
}
func githubTaskReturn(value string) string {
	if regexp.MustCompile(`^/tasks/[A-Za-z0-9_-]+(?:/sessions/[A-Za-z0-9_-]+)?$`).MatchString(value) {
		return value
	}
	return ""
}
func oauthRandom() (string, error) {
	var data [32]byte
	_, err := rand.Read(data[:])
	return hex.EncodeToString(data[:]), err
}
func oauthHash(value string) string {
	digest := sha256.Sum256([]byte(value))
	return hex.EncodeToString(digest[:])
}
func (s *Server) handleGitHubOAuth(w http.ResponseWriter, r *http.Request) {
	owner := s.accountOwner(w, r)
	if owner == "" {
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	client, _, callback, configured := githubOAuthConfig()
	if r.Method == "GET" {
		respondJSON(w, 200, map[string]any{"configured": configured})
		return
	}
	if !configured {
		respondJSONError(w, 503, "GitHub sign-in is not configured for this installation.")
		return
	}
	var body struct {
		ReturnTo string `json:"return_to"`
	}
	if json.NewDecoder(http.MaxBytesReader(w, r.Body, 2048)).Decode(&body) != nil {
		respondJSONError(w, 400, "Invalid GitHub connection request.")
		return
	}
	state, err := oauthRandom()
	if err != nil {
		respondJSONError(w, 503, "Could not start GitHub connection.")
		return
	}
	cookie, err := oauthRandom()
	if err != nil {
		respondJSONError(w, 503, "Could not start GitHub connection.")
		return
	}
	githubOAuthFlows.Lock()
	for key, flow := range githubOAuthFlows.flows {
		if time.Now().After(flow.Expires) {
			delete(githubOAuthFlows.flows, key)
		}
	}
	if len(githubOAuthFlows.flows) >= 100 {
		githubOAuthFlows.Unlock()
		respondJSONError(w, 429, "Too many pending GitHub connections. Try again later.")
		return
	}
	githubOAuthFlows.flows[oauthHash(state)] = githubOAuthFlow{Owner: owner, ReturnTo: githubTaskReturn(body.ReturnTo), CookieHash: oauthHash(cookie), Expires: time.Now().Add(10 * time.Minute)}
	githubOAuthFlows.Unlock()
	http.SetCookie(w, &http.Cookie{Name: "cuan_github_oauth", Value: cookie, Path: "/api/v1/integrations/github/oauth", HttpOnly: true, Secure: strings.HasPrefix(callback, "https://"), SameSite: http.SameSiteLaxMode, MaxAge: 600})
	values := url.Values{"client_id": {client}, "redirect_uri": {callback}, "state": {state}, "scope": {"repo"}}
	respondJSON(w, 200, map[string]string{"authorize_url": "https://github.com/login/oauth/authorize?" + values.Encode()})
}
func (s *Server) handleGitHubOAuthCallback(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Referrer-Policy", "no-referrer")
	state := r.URL.Query().Get("state")
	cookie, err := r.Cookie("cuan_github_oauth")
	githubOAuthFlows.Lock()
	flow, exists := githubOAuthFlows.flows[oauthHash(state)]
	valid := exists && len(state) == 64 && err == nil && time.Now().Before(flow.Expires)
	if valid {
		valid = subtle.ConstantTimeCompare([]byte(flow.CookieHash), []byte(oauthHash(cookie.Value))) == 1
	}
	if valid {
		delete(githubOAuthFlows.flows, oauthHash(state))
	}
	githubOAuthFlows.Unlock()
	if !valid {
		http.Redirect(w, r, "/settings/integrations/github?github_status=expired", http.StatusSeeOther)
		return
	}
	redirect := func(status string) {
		values := url.Values{"github_status": {status}}
		if flow.ReturnTo != "" {
			values.Set("return_to", flow.ReturnTo)
		}
		http.Redirect(w, r, "/settings/integrations/github?"+values.Encode(), http.StatusSeeOther)
	}
	client, secret, callback, configured := githubOAuthConfig()
	if !configured {
		redirect("unavailable")
		return
	}
	http.SetCookie(w, &http.Cookie{Name: "cuan_github_oauth", Value: "", Path: "/api/v1/integrations/github/oauth", HttpOnly: true, Secure: strings.HasPrefix(callback, "https://"), SameSite: http.SameSiteLaxMode, MaxAge: -1})
	code := r.URL.Query().Get("code")
	if r.URL.Query().Get("error") != "" || code == "" {
		redirect("cancelled")
		return
	}
	// The browser identity and owner are bound before code exchange; no credential
	// or provider error body is included in a redirect or application response.
	current, err := s.authSvc.OwnerID(r.Context())
	if err != nil || current != flow.Owner {
		redirect("expired")
		return
	}
	encoded, _ := json.Marshal(map[string]string{"client_id": client, "client_secret": secret, "code": code, "redirect_uri": callback})
	request, err := http.NewRequestWithContext(r.Context(), "POST", githubOAuthTokenURL, bytes.NewReader(encoded))
	if err != nil {
		redirect("failed")
		return
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "application/json")
	response, err := githubOAuthHTTP.Do(request)
	if err != nil {
		redirect("failed")
		return
	}
	defer response.Body.Close()
	var result struct {
		Token string `json:"access_token"`
		Error string `json:"error"`
	}
	if response.StatusCode != 200 || json.NewDecoder(io.LimitReader(response.Body, 64<<10)).Decode(&result) != nil || result.Token == "" || result.Error != "" {
		redirect("failed")
		return
	}
	api := git.NewClient()
	api.SetAPIBaseURL(githubOAuthAPIBase)
	account, err := api.Account(r.Context(), result.Token)
	if err != nil {
		redirect("failed")
		return
	}
	path, err := githubConnectionPath(flow.Owner)
	if err != nil {
		redirect("failed")
		return
	}
	githubConnectionMu.Lock()
	err = git.SaveConnection(path, s.cfg.AuthSecret, result.Token, account)
	githubConnectionMu.Unlock()
	if err != nil {
		redirect("failed")
		return
	}
	redirect("connected")
}
