package server

import (
	"encoding/json"
	"github.com/buchin/cuan/internal/git"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
)

func TestGitHubOAuthBrowserBoundExchange(t *testing.T) {
	s, token := setupTestChatServer(t)
	t.Setenv("HOME", t.TempDir())
	t.Setenv("CUAN_GITHUB_CLIENT_ID", "fixture-client")
	t.Setenv("CUAN_GITHUB_CLIENT_SECRET", "fixture-secret")
	t.Setenv("CUAN_GITHUB_REDIRECT_URL", "https://cuan.example/api/v1/integrations/github/oauth/callback")
	exchanges := 0
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/exchange" {
			exchanges++
			var body map[string]string
			json.NewDecoder(r.Body).Decode(&body)
			if body["client_secret"] != "fixture-secret" || body["code"] != "fixture-code" {
				t.Error("bad code exchange")
			}
			w.Write([]byte(`{"access_token":"fixture-oauth-token"}`))
			return
		}
		if r.URL.Path == "/user" && r.Header.Get("Authorization") == "Bearer fixture-oauth-token" {
			w.Write([]byte(`{"login":"fixture-owner"}`))
			return
		}
		w.WriteHeader(401)
	}))
	defer provider.Close()
	oldToken, oldBase := githubOAuthTokenURL, githubOAuthAPIBase
	githubOAuthTokenURL = provider.URL + "/exchange"
	githubOAuthAPIBase = provider.URL
	defer func() { githubOAuthTokenURL = oldToken; githubOAuthAPIBase = oldBase }()
	begin := httptest.NewRequest("POST", "/api/v1/integrations/github/oauth", strings.NewReader(`{"return_to":"/tasks/task-one"}`))
	begin.Header.Set("Authorization", "Bearer "+token)
	start := httptest.NewRecorder()
	s.Router().ServeHTTP(start, begin)
	if start.Code != 200 {
		t.Fatal(start.Code, start.Body.String())
	}
	var response map[string]string
	json.Unmarshal(start.Body.Bytes(), &response)
	authorize, _ := url.Parse(response["authorize_url"])
	if authorize.Host != "github.com" || strings.Contains(response["authorize_url"], "fixture-secret") {
		t.Fatal("invalid authorization transport")
	}
	cookies := start.Result().Cookies()
	if len(cookies) != 1 || !cookies[0].HttpOnly || !cookies[0].Secure {
		t.Fatal("browser state cookie not private")
	}
	callback := "/api/v1/integrations/github/oauth/callback?state=" + authorize.Query().Get("state") + "&code=fixture-code"
	wrong := httptest.NewRecorder()
	s.Router().ServeHTTP(wrong, httptest.NewRequest("GET", callback, nil))
	if exchanges != 0 || !strings.Contains(wrong.Header().Get("Location"), "expired") {
		t.Fatal("code exchange accepted without browser cookie")
	}
	request := httptest.NewRequest("GET", callback, nil)
	request.AddCookie(cookies[0])
	done := httptest.NewRecorder()
	s.Router().ServeHTTP(done, request)
	destination, _ := url.Parse(done.Header().Get("Location"))
	if done.Code != 303 || destination.Query().Get("github_status") != "connected" || destination.Query().Get("return_to") != "/tasks/task-one" {
		t.Fatal(done.Code, done.Header())
	}
	owner, _ := s.authSvc.OwnerID(request.Context())
	path, _ := githubConnectionPath(owner)
	account, saved, e := git.LoadConnection(path, s.cfg.AuthSecret)
	if e != nil || account.Login != "fixture-owner" || saved != "fixture-oauth-token" {
		t.Fatal("OAuth credential not saved")
	}
	raw, _ := os.ReadFile(path)
	if strings.Contains(string(raw), saved) {
		t.Fatal("plaintext persisted")
	}
	replay := httptest.NewRequest("GET", callback, nil)
	replay.AddCookie(cookies[0])
	s.Router().ServeHTTP(httptest.NewRecorder(), replay)
	if exchanges != 1 {
		t.Fatal("callback replay exchanged code twice")
	}
}
func TestGitHubOAuthUnavailableAndSafeReturn(t *testing.T) {
	s, token := setupTestChatServer(t)
	t.Setenv("CUAN_GITHUB_CLIENT_ID", "")
	request := httptest.NewRequest("POST", "/api/v1/integrations/github/oauth", strings.NewReader(`{}`))
	request.Header.Set("Authorization", "Bearer "+token)
	response := httptest.NewRecorder()
	s.Router().ServeHTTP(response, request)
	if response.Code != 503 {
		t.Fatal("unconfigured OAuth advertised")
	}
	for _, path := range []string{"https://example.com", "//example.com", "/tasks/x?redirect=external", "/settings"} {
		if githubTaskReturn(path) != "" {
			t.Fatal("unsafe task return accepted", path)
		}
	}
}
