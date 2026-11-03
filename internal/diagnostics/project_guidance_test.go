package diagnostics

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestProjectGuidancePreservesOtherSettings(t *testing.T) {
	p := map[string]any{"id": "p", "name": "Cuan", "repo_url": "https://github.com/buchin/cuan", "default_branch": "dev", "updated_at": "2026-11-03T00:00:00Z", "metadata": map[string]any{"cuan_settings": map[string]any{"goal": "old", "instructions": "keep rules", "environment": "TOKEN=fixture-private", "deploy_branch": "release", "auto_deploy": false}}}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer fixture" {
			t.Fatal("missing authentication")
		}
		if r.Method == http.MethodPatch {
			var body map[string]any
			json.NewDecoder(r.Body).Decode(&body)
			if body["goal"] != "new goal" || body["instructions"] != "keep rules" || body["environment"] != "TOKEN=fixture-private" || body["auto_deploy"] != false || body["expected_updated_at"] != p["updated_at"] {
				t.Fatal("update changed unrelated settings or lost concurrency guard")
			}
			p["metadata"].(map[string]any)["cuan_settings"].(map[string]any)["goal"] = body["goal"]
		}
		json.NewEncoder(w).Encode(p)
	}))
	defer server.Close()
	client := &CuanClient{BaseURL: server.URL, AuthToken: "fixture", HTTPClient: server.Client()}
	goal := "new goal"
	result, err := client.UpdateProjectGuidance(context.Background(), "p", &goal, nil)
	if err != nil || result.Goal != goal {
		t.Fatal("goal not updated", err)
	}
	raw, _ := json.Marshal(result)
	if strings.Contains(string(raw), "fixture-private") {
		t.Fatal("environment leaked in guidance output")
	}
}
