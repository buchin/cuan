package diagnostics

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestProjectSettingsGetAndUpdate(t *testing.T) {
	p := map[string]any{
		"id":             "proj-test-1",
		"name":           "TestApp",
		"repo_url":       "https://github.com/example/testapp",
		"default_branch": "main",
		"updated_at":     "2026-11-14T00:00:00Z",
		"metadata": map[string]any{
			"cuan_settings": map[string]any{
				"goal":          "Build great app",
				"instructions":  "Follow DoD",
				"environment":   "API_KEY=fixture-secret",
				"deploy_branch": "production",
				"deploy_script": "bash scripts/deploy.sh",
				"auto_deploy":   true,
			},
		},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-token" {
			t.Fatal("missing authorization")
		}
		if r.Method == http.MethodPatch {
			var body map[string]any
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatal("invalid json body", err)
			}
			if body["expected_updated_at"] != p["updated_at"] {
				t.Fatal("missing or incorrect expected_updated_at")
			}
			settings := p["metadata"].(map[string]any)["cuan_settings"].(map[string]any)
			if ds, ok := body["deploy_script"].(string); ok {
				settings["deploy_script"] = ds
			}
			if ad, ok := body["auto_deploy"].(bool); ok {
				settings["auto_deploy"] = ad
			}
			if db, ok := body["deploy_branch"].(string); ok {
				settings["deploy_branch"] = db
			}
			p["updated_at"] = "2026-11-14T00:00:01Z"
		}
		json.NewEncoder(w).Encode(p)
	}))
	defer server.Close()

	client := &CuanClient{BaseURL: server.URL, AuthToken: "test-token", HTTPClient: server.Client()}

	// Test Get
	settings, err := client.GetProjectSettings(context.Background(), "proj-test-1")
	if err != nil {
		t.Fatal("GetProjectSettings failed:", err)
	}
	if settings.DeployScript != "bash scripts/deploy.sh" || !settings.AutoDeploy || settings.DeployBranch != "production" {
		t.Fatalf("unexpected settings: %+v", settings)
	}

	// Ensure secret environment is not leaked
	raw, _ := json.Marshal(settings)
	if strings.Contains(string(raw), "fixture-secret") {
		t.Fatal("environment secret leaked in settings output")
	}

	// Test Update
	newScript := "echo updated deploy"
	newAutoDeploy := false
	updated, err := client.UpdateProjectSettings(context.Background(), "proj-test-1", &newScript, nil, &newAutoDeploy)
	if err != nil {
		t.Fatal("UpdateProjectSettings failed:", err)
	}
	if updated.DeployScript != newScript || updated.AutoDeploy != false {
		t.Fatalf("updated settings mismatch: %+v", updated)
	}
}

func TestProjectSettingsCLIRun(t *testing.T) {
	p := map[string]any{
		"id":             "proj-test-cli",
		"name":           "TestApp",
		"repo_url":       "https://github.com/example/testapp",
		"default_branch": "main",
		"updated_at":     "2026-11-14T00:00:00Z",
		"metadata": map[string]any{
			"cuan_settings": map[string]any{
				"deploy_script": "old script",
				"auto_deploy":   true,
			},
		},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPatch {
			var body map[string]any
			_ = json.NewDecoder(r.Body).Decode(&body)
			settings := p["metadata"].(map[string]any)["cuan_settings"].(map[string]any)
			if ds, ok := body["deploy_script"].(string); ok {
				settings["deploy_script"] = ds
			}
			if ad, ok := body["auto_deploy"].(bool); ok {
				settings["auto_deploy"] = ad
			}
			p["updated_at"] = "2026-11-14T00:00:02Z"
		}
		json.NewEncoder(w).Encode(p)
	}))
	defer server.Close()

	origClient := globalCuanClient
	globalCuanClient = &CuanClient{BaseURL: server.URL, AuthToken: "test-token", HTTPClient: server.Client()}
	defer func() { globalCuanClient = origClient }()

	var out, errOut bytes.Buffer
	code := runProjectSettingsCLI([]string{"show", "proj-test-cli"}, &out, &errOut)
	if code != 0 {
		t.Fatalf("cli show failed code=%d err=%s", code, errOut.String())
	}
	if !strings.Contains(out.String(), "old script") {
		t.Fatalf("expected old script in output: %s", out.String())
	}

	out.Reset()
	errOut.Reset()
	code = runProjectSettingsCLI([]string{"set", "proj-test-cli", "--deploy-script", "new script", "--auto-deploy", "false"}, &out, &errOut)
	if code != 0 {
		t.Fatalf("cli set failed code=%d err=%s", code, errOut.String())
	}
	if !strings.Contains(out.String(), "new script") || !strings.Contains(out.String(), `"auto_deploy":false`) {
		t.Fatalf("expected new script and auto_deploy false in output: %s", out.String())
	}
}
