package diagnostics

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

func environmentFixture(t *testing.T, status int) (*CuanClient, *int) {
	t.Helper()
	patches := 0
	p := map[string]any{"id": "cuan-id", "name": "Cuan", "repo_url": "https://github.com/buchin/cuan", "default_branch": "dev", "updated_at": "2026-11-03T00:00:00Z", "metadata": map[string]any{"cuan_settings": map[string]any{"goal": "keep goal", "instructions": "keep instructions", "auto_deploy": false, "deploy_branch": "release", "environment": "# keep comment\nKEEP=old-secret\nCHANGE=old\nCHANGE=duplicate\nREMOVE=gone"}}}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-token" {
			t.Error("missing authorization")
		}
		switch r.Method {
		case http.MethodGet:
			if r.URL.Path != "/api/v1/projects/cuan-id" {
				t.Errorf("unexpected get path: %s", r.URL.Path)
			}
			_ = json.NewEncoder(w).Encode(p)
		case http.MethodPatch:
			patches++
			if r.URL.Path != "/api/v1/projects/cuan-id/settings" {
				t.Errorf("unexpected patch path: %s", r.URL.Path)
			}
			var body map[string]any
			_ = json.NewDecoder(r.Body).Decode(&body)
			for key, want := range map[string]any{"name": "Cuan", "repo_url": "https://github.com/buchin/cuan", "default_branch": "dev", "goal": "keep goal", "instructions": "keep instructions", "auto_deploy": false, "deploy_branch": "release", "expected_updated_at": p["updated_at"]} {
				if body[key] != want {
					t.Errorf("setting %s was not preserved", key)
				}
			}
			if status != http.StatusOK {
				w.WriteHeader(status)
				_, _ = w.Write([]byte("echoed-new-secret"))
				return
			}
			settings := p["metadata"].(map[string]any)["cuan_settings"].(map[string]any)
			settings["environment"] = body["environment"]
			p["updated_at"] = "2026-11-03T00:01:00Z"
			_ = json.NewEncoder(w).Encode(p)
		default:
			t.Errorf("unexpected method: %s", r.Method)
		}
	}))
	t.Cleanup(server.Close)
	return &CuanClient{BaseURL: server.URL, AuthToken: "test-token", HTTPClient: server.Client()}, &patches
}

func TestProjectEnvironmentMergeAndRedaction(t *testing.T) {
	c, patches := environmentFixture(t, http.StatusOK)
	ctx := context.Background()
	got, err := c.UpdateProjectEnvironment(ctx, "cuan-id", map[string]string{"CHANGE": "new-secret#=$()", "EMPTY": ""}, []string{"REMOVE"})
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(got)
	if strings.Contains(string(raw), "secret") || !got.Redacted {
		t.Fatal("update leaked values")
	}
	if *patches != 1 {
		t.Fatal("expected one patch")
	}
	read, err := c.GetProjectEnvironment(ctx, "cuan-id", true)
	if err != nil {
		t.Fatal(err)
	}
	if read.Values["KEEP"] != "old-secret" || read.Values["CHANGE"] != "new-secret#=$()" {
		t.Fatal("merge lost values")
	}
	if _, ok := read.Values["REMOVE"]; ok {
		t.Fatal("unset failed")
	}
	if value, ok := read.Values["EMPTY"]; !ok || value != "" {
		t.Fatal("empty value lost")
	}
	redacted, err := c.GetProjectEnvironment(ctx, "cuan-id", false)
	if err != nil || redacted.Values["KEEP"] != "[REDACTED]" {
		t.Fatal("read must redact by default")
	}
}

func TestProjectEnvironmentFailureSafety(t *testing.T) {
	for _, status := range []int{http.StatusConflict, http.StatusForbidden, http.StatusUnauthorized, http.StatusBadRequest} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			c, patches := environmentFixture(t, status)
			_, err := c.UpdateProjectEnvironment(context.Background(), "cuan-id", map[string]string{"CHANGE": "new-secret"}, nil)
			if err == nil || strings.Contains(err.Error(), "new-secret") {
				t.Fatal("expected safe error")
			}
			if *patches != 1 {
				t.Fatal("must not retry conflicting writes")
			}
		})
	}
	c, patches := environmentFixture(t, http.StatusOK)
	for _, values := range []map[string]string{{"BAD-KEY": "secret"}, {"A": "secret\nINJECT=bad"}, {"A": " secret"}, {"A": "'quoted'"}} {
		if _, err := c.UpdateProjectEnvironment(context.Background(), "cuan-id", values, nil); err == nil {
			t.Fatal("invalid input accepted")
		}
	}
	if *patches != 0 {
		t.Fatal("invalid input must not mutate")
	}
}

func TestProjectEnvironmentCLIAndMCP(t *testing.T) {
	client, _ := environmentFixture(t, http.StatusOK)
	old := globalCuanClient
	globalCuanClient = client
	t.Cleanup(func() { globalCuanClient = old })
	t.Setenv("CHANGE", "process-secret")
	var out, errOut bytes.Buffer
	code := RunProjectCLI([]string{"env", "set", "cuan-id", "--from-env", "CHANGE"}, strings.NewReader(""), &out, &errOut)
	if code != 0 {
		t.Fatal(errOut.String())
	}
	if strings.Contains(out.String(), "process-secret") {
		t.Fatal("CLI leaked secret")
	}
	out.Reset()
	code = RunProjectCLI([]string{"env", "set", "cuan-id", "--file", "-"}, strings.NewReader("# comment\nNEW='stdin-secret'\n"), &out, &errOut)
	if code != 0 || strings.Contains(out.String(), "stdin-secret") {
		t.Fatal("stdin import failed or leaked")
	}
	for _, tool := range []string{"get_project_environment", "update_project_environment"} {
		out.Reset()
		request := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"` + tool + `","arguments":{"project_id":"cuan-id","from_env":["CHANGE"]}}}` + "\n"
		if RunMCP(strings.NewReader(request), &out, &errOut) != 0 {
			t.Fatal(errOut.String())
		}
		if strings.Contains(out.String(), "process-secret") || strings.Contains(out.String(), `"isError":true`) {
			t.Fatalf("unsafe or failed MCP: %s", out.String())
		}
	}
	out.Reset()
	code = RunProjectCLI([]string{"env", "unset", "cuan-id", "--key", "NEW"}, strings.NewReader(""), &out, &errOut)
	if code != 0 {
		t.Fatal(errOut.String())
	}
}

func TestProjectEnvironmentInputErrors(t *testing.T) {
	for _, raw := range []string{"SECRET LINE WITHOUT EQUALS", "A=one\nA=two", "BAD-KEY=value"} {
		if _, err := parseEnvironmentInput(raw); err == nil || strings.Contains(err.Error(), raw) {
			t.Fatal("expected safe parse error")
		}
	}
	t.Setenv("PRESENT", "secret")
	values := map[string]string{}
	if err := environmentFromProcess([]string{"PRESENT"}, values); err != nil {
		t.Fatal(err)
	}
	if err := environmentFromProcess([]string{"PRESENT"}, values); err == nil {
		t.Fatal("duplicates accepted")
	}
	t.Setenv("MISSING_ENV_TEST", "")
	// Missing must differ from explicitly empty.
	if err := environmentFromProcess([]string{"MISSING_ENV_TEST"}, map[string]string{}); err != nil {
		t.Fatal(err)
	}
}

func TestProjectEnvironmentGeneralInspectionRedacts(t *testing.T) {
	p := map[string]any{"id": "project", "metadata": map[string]any{"cuan_settings": map[string]any{"goal": "keep", "environment": "PASSWORD=private-secret"}}}
	redactProjectEnvironments([]map[string]any{p})
	raw, _ := json.Marshal(p)
	if strings.Contains(string(raw), "private-secret") {
		t.Fatal("general project inspection leaked environment")
	}
	settings := p["metadata"].(map[string]any)["cuan_settings"].(map[string]any)
	if settings["goal"] != "keep" {
		t.Fatal("unrelated project setting changed")
	}
}

func TestProjectEnvironmentMissingInput(t *testing.T) {
	const missing = "CUAN_TEST_UNSET_ENV_3B814"
	t.Setenv(missing, "temporary")
	if err := os.Unsetenv(missing); err != nil {
		t.Fatal(err)
	}
	if err := environmentFromProcess([]string{missing}, map[string]string{}); err == nil {
		t.Fatal("missing environment accepted")
	}
	client, patches := environmentFixture(t, http.StatusOK)
	old := globalCuanClient
	globalCuanClient = client
	t.Cleanup(func() { globalCuanClient = old })
	for _, args := range [][]string{{"env", "set", "cuan-id"}, {"env", "unset", "cuan-id"}, {"env", "set", "cuan-id", "--from-env", missing}, {"env", "set", "cuan-id", "literal-secret"}} {
		var out, errOut bytes.Buffer
		if RunProjectCLI(args, strings.NewReader(""), &out, &errOut) == 0 {
			t.Fatal("empty/invalid operation succeeded")
		}
		if strings.Contains(out.String()+errOut.String(), "literal-secret") {
			t.Fatal("input echoed in diagnostic")
		}
	}
	if *patches != 0 {
		t.Fatal("invalid CLI operation mutated settings")
	}
}
