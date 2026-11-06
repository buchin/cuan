package diagnostics

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestTaskDraftUsesOwnerAPIAndRequiresReceipt(t *testing.T) {
	for _, tc := range []struct {
		name    string
		status  int
		body    string
		success bool
	}{
		{"created", 201, `{"id":"server-task","status":"backlog","instructions":"approved"}`, true},
		{"unavailable", 503, `PRIVATE_SECRET`, false},
		{"missing identity", 201, `{}`, false},
		{"malformed", 201, `PRIVATE_SECRET`, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				if r.Method != "POST" || r.URL.Path != "/api/v1/tasks" || r.Header.Get("Authorization") != "Bearer fixture" {
					t.Error("wrong creation transport")
				}
				var req struct {
					ProjectID    string `json:"project_id"`
					Title        string `json:"title"`
					Instructions string `json:"instructions"`
					Checklist    []struct {
						Text      string `json:"text"`
						Completed bool   `json:"completed"`
					} `json:"dod_checklist"`
				}
				if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
					t.Fatal(err)
				}
				if req.ProjectID != "exact-project" || req.Title != "Work" || req.Instructions != "approved" || len(req.Checklist) != 1 || req.Checklist[0].Text != "Verified" || req.Checklist[0].Completed {
					t.Error("creation scope changed")
				}
				w.WriteHeader(tc.status)
				w.Write([]byte(tc.body))
			}))
			defer api.Close()
			oldClient, oldDB := globalCuanClient, platformDB
			globalCuanClient = &CuanClient{BaseURL: api.URL, AuthToken: "fixture", HTTPClient: api.Client()}
			platformDB = &sql.DB{} // Must never fall through to an ambient database.
			defer func() { globalCuanClient = oldClient; platformDB = oldDB }()
			result, err := CreateTaskDraft(context.Background(), "exact-project", "Work", "approved", "normal", []string{"Verified"})
			if calls != 1 {
				t.Fatalf("mutation repeated: %d", calls)
			}
			if tc.success {
				if err != nil || result.(map[string]any)["id"] != "server-task" {
					t.Fatal("native receipt lost", err)
				}
			} else if err == nil || result != nil || strings.Contains(err.Error(), "PRIVATE_SECRET") {
				t.Fatal("false success or leaked failure", err)
			}
		})
	}
}

func TestTaskDraftWithoutTransportCannotSucceed(t *testing.T) {
	oldClient, oldDB := globalCuanClient, platformDB
	globalCuanClient, platformDB = &CuanClient{}, nil
	defer func() { globalCuanClient = oldClient; platformDB = oldDB }()
	result, err := CreateTaskDraft(context.Background(), "project", "Work", "approved", "normal", nil)
	if err == nil || result != nil {
		t.Fatal("fabricated task creation", result)
	}
}
