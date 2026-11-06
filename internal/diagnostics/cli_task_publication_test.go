package diagnostics

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestTaskVerifyPublicationUsesSavedActiveBindingWithoutDeployOrAcceptance(t *testing.T) {
	for _, scenario := range []string{"verified", "inactive", "unavailable", "wrong-attempt", "forged-health"} {
		t.Run(scenario, func(t *testing.T) {
			calls := 0
			commit := strings.Repeat("a", 40)
			api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				if r.Header.Get("Authorization") != "Bearer fixture" {
					t.Error("unauthed request")
				}
				if calls == 1 {
					if r.Method != "GET" || r.URL.Path != "/api/v1/tasks/task" {
						t.Error("unexpected task read")
					}
					state := "running"
					if scenario == "inactive" {
						state = "failed"
					}
					json.NewEncoder(w).Encode(map[string]any{"id": "task", "commit_sha": commit, "latest_attempt": map[string]string{"id": "attempt", "runner_id": "runner", "status": state}})
					return
				}
				if r.Method != "POST" || r.URL.Path != "/api/v1/runner/attempts/attempt/publication/verify" {
					t.Error("nonverification operation")
				}
				var body map[string]string
				json.NewDecoder(r.Body).Decode(&body)
				if len(body) != 3 || body["task_id"] != "task" || body["runner_id"] != "runner" || body["commit_sha"] != commit {
					t.Error("unbound observation")
				}
				if scenario == "unavailable" {
					w.WriteHeader(503)
					w.Write([]byte(`{"private":"do-not-echo"}`))
					return
				}
				attempt := "attempt"
				if scenario == "wrong-attempt" {
					attempt = "old"
				}
				health := "healthy"
				if scenario == "forged-health" {
					health = "failed"
				}
				json.NewEncoder(w).Encode(map[string]any{"verified": true, "receipt": map[string]any{"version": 1, "task_id": "task", "attempt_id": attempt, "commit_sha": commit, "release_sha": commit, "health": health, "environment": "production"}})
			}))
			defer api.Close()
			previous := globalCuanClient
			globalCuanClient = &CuanClient{BaseURL: api.URL, AuthToken: "fixture", HTTPClient: api.Client()}
			t.Cleanup(func() { globalCuanClient = previous })
			var out, errOut bytes.Buffer
			code := RunTaskCLI([]string{"verify-publication", "task"}, &out, &errOut)
			if (code == 0) != (scenario == "verified") {
				t.Fatal("unexpected acceptance", code, errOut.String())
			}
			wanted := 2
			if scenario == "inactive" {
				wanted = 1
			}
			if calls != wanted {
				t.Fatal("verification retried or mutated work")
			}
			if strings.Contains(out.String()+errOut.String(), "do-not-echo") {
				t.Fatal("private error leaked")
			}
		})
	}
}
