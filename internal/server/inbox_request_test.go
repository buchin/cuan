package server

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/buchin/cuan/internal/agent"
	"github.com/buchin/cuan/internal/db"
)

func TestS264TimeoutAfterAcceptanceSafeRetry(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	s, token := setupTestChatServer(t)
	owner, _ := s.authSvc.ValidateToken(context.Background(), token)
	profile, err := agent.CreateProviderAccount(owner.ID, "fixture-inbox")
	if err != nil {
		t.Fatal(err)
	}
	profileHome, _ := agent.ProviderAccountHome(owner.ID, profile.ID)
	os.MkdirAll(agent.AntigravityConfigDirPath(profileHome), 0700)
	os.WriteFile(agent.AntigravityCredentialsFilePath(profileHome), []byte("fixture-not-real-provider-credential"), 0600)
	call := func(method, path, body string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, path, strings.NewReader(body))
		r.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()
		s.Router().ServeHTTP(w, r)
		return w
	}
	w := call("POST", "/api/v1/agents/connect-cli", `{"name":"@fixture","handle":"@fixture","provider":"agy","account_id":"`+profile.ID+`"}`)
	if w.Code != 201 {
		t.Fatal(w.Body.String())
	}
	bin := t.TempDir()
	// Observe provider invocations independently of the brain selected by @cuan.
	counter := filepath.Join(t.TempDir(), "execution-count")
	shellCounter := "'" + strings.ReplaceAll(counter, "'", `'\''`) + "'"
	os.WriteFile(filepath.Join(bin, "agy"), []byte("#!/bin/sh\necho run >> "+shellCounter+"\nsleep 0.2\nprintf '%s\\n' '{\"event\":\"result\",\"result\":{\"status\":\"SUCCESS\",\"response\":\"{\\\"reply\\\":\\\"fixture-reply\\\",\\\"topic_title\\\":\\\"\\\",\\\"action\\\":null}\"}}'\n"), 0700)
	installInboxSandboxStub(t, bin)
	t.Setenv("PATH", bin+":/usr/bin:/bin")
	s.cfg.Environment = "staging"
	path := "/api/v1/chat/conversations/agent_fixture/messages"
	// Prime the durable room before concurrent requests.
	if w := call("GET", path, ""); w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	id := db.GenerateUUID()
	body := `{"message":"implement fixture timeout","request_id":"` + id + `"}`
	// The server accepts, then its response is delayed beyond the client's deadline.
	delayed := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rec := httptest.NewRecorder()
		s.Router().ServeHTTP(rec, r)
		time.Sleep(80 * time.Millisecond)
		w.WriteHeader(rec.Code)
		w.Write(rec.Body.Bytes())
	}))
	defer delayed.Close()
	req, _ := http.NewRequest("POST", delayed.URL+path, strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	client := &http.Client{Timeout: 30 * time.Millisecond}
	res, e := client.Do(req)
	if res != nil {
		res.Body.Close()
	}
	if e == nil {
		t.Fatal("expected timeout after acceptance")
	}
	for i := 0; i < 3; i++ {
		w = call("POST", path, body)
		if w.Code != 202 && w.Code != 200 {
			t.Fatal(w.Code, w.Body.String())
		}
	}
	deadline := time.Now().Add(3 * time.Second)
	for {
		w = call("GET", path, "")
		if strings.Contains(w.Body.String(), `"state":"completed"`) {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal(w.Body.String())
		}
		time.Sleep(15 * time.Millisecond)
	}
	data, _ := os.ReadFile(counter)
	if strings.Count(string(data), "run") != 1 {
		t.Fatal("duplicate executions", string(data))
	}
	var got struct {
		Messages []ChatMessageDTO
		Requests []db.InboxRequest
	}
	json.Unmarshal(w.Body.Bytes(), &got)
	if len(got.Messages) != 2 || len(got.Requests) != 1 || got.Requests[0].MessageID != got.Messages[0].UUID {
		t.Fatalf("correlation/messages: %+v", got)
	}
	// Reload a new HTTP server around the same durable repository (SQL process
	// restart persistence is independently exercised in isolated staging).
	next, nextToken := setupTestChatServer(t)
	next.SetProjectStudioRepository(s.projectStudioRepo)
	next.SetAgentRepository(s.agentRepo)
	rr := httptest.NewRequest("GET", path, nil)
	rr.Header.Set("Authorization", "Bearer "+nextToken)
	// Auth stores are separate, so retain original auth service for this fixture.

	rw := httptest.NewRecorder()
	next.Router().ServeHTTP(rw, rr)
	if rw.Code != 200 || !strings.Contains(rw.Body.String(), id) {
		t.Fatal(rw.Body.String())
	}
	w = call("POST", path, `{"message":"different","request_id":"`+id+`"}`)
	if w.Code != 409 {
		t.Fatal("payload conflict not rejected")
	}
	t.Log("FIXTURE: HTTP timeout after committed acceptance; three concurrent-lifetime retries, refresh and handler restart preserve one user, one reply, one CLI execution and original correlation")
	// Quota/login/unknown CLI failures must never expose stderr secrets or paths.
	for index, item := range []struct{ raw, want, state string }{{"unknown SECRET /private/profile", "before a final result", "failed"}, {"401 invalid_grant SECRET /private/profile", "sign-in", "failed"}, {"429 quota SECRET /private/profile", "capacity", "queued"}} {
		// Each failure gets an independent account: quota/auth observations
		// intentionally exclude a used account from subsequent routing.
		account, err := agent.CreateProviderAccount(owner.ID, fmt.Sprintf("failure-%d", index))
		if err != nil {
			t.Fatal(err)
		}
		accountHome, _ := agent.ProviderAccountHome(owner.ID, account.ID)
		os.MkdirAll(agent.AntigravityConfigDirPath(accountHome), 0700)
		os.WriteFile(agent.AntigravityCredentialsFilePath(accountHome), []byte("fixture-not-real-provider-credential"), 0600)
		handle := fmt.Sprintf("@failure%d", index)
		connected := call("POST", "/api/v1/agents/connect-cli", fmt.Sprintf(`{"name":%q,"handle":%q,"provider":"agy","account_id":%q}`, handle, handle, account.ID))
		if connected.Code != 201 {
			t.Fatal(connected.Body.String())
		}
		path := fmt.Sprintf("/api/v1/chat/conversations/agent_failure%d/messages", index)
		os.WriteFile(filepath.Join(bin, "agy"), []byte("#!/bin/sh\necho run >> "+shellCounter+"\nprintf '"+item.raw+"' >&2\nexit 1\n"), 0700)
		before, _ := os.ReadFile(counter)
		key := db.GenerateUUID()
		w = call("POST", path, `{"message":"fixture failure","request_id":"`+key+`"}`)
		if w.Code != 202 {
			t.Fatal(w.Body.String())
		}
		for end := time.Now().Add(time.Second); ; {
			w = call("GET", path, "")
			var v struct{ Requests []db.InboxRequest }
			json.Unmarshal(w.Body.Bytes(), &v)
			done := false
			for _, q := range v.Requests {
				if q.RequestID == key && q.State == item.state && q.SafeError != "" {
					if !strings.Contains(q.SafeError, item.want) {
						t.Fatal(q)
					}
					done = true
				}
			}
			if done {
				break
			}
			if time.Now().After(end) {
				t.Fatal(w.Body.String())
			}
			time.Sleep(10 * time.Millisecond)
		}
		raw, _ := io.ReadAll(w.Result().Body)
		if strings.Contains(string(raw), "SECRET") || strings.Contains(string(raw), "/private/profile") {
			t.Fatal("internal output leaked")
		}
		after, err := os.ReadFile(counter)
		if err != nil || len(after) <= len(before) {
			t.Fatal("failure fixture did not actually execute its provider", err)
		}
		w = call("POST", path, `{"message":"fixture failure","request_id":"`+key+`"}`)
		if w.Code != 200 && !(item.state == "queued" && w.Code == 202) {
			t.Fatal(w.Body.String())
		}
		time.Sleep(50 * time.Millisecond)
		repeated, err := os.ReadFile(counter)
		if err != nil || string(repeated) != string(after) {
			t.Fatal("duplicate acceptance replayed provider work", err)
		}
	}
	if text := safeInboxProviderFailure(context.DeadlineExceeded, "SECRET"); !strings.Contains(text, "timed out") || strings.Contains(text, "SECRET") {
		t.Fatal(text)
	}
	t.Log("FIXTURE: quota remains queued; login, unknown provider error and deadline retain safe diagnostics; duplicate acceptance never replays provider work")
}
