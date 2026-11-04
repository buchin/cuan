package diagnostics

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestResendCLIExplicitSecretImportAndRedaction(t *testing.T) {
	t.Setenv("RESEND_FIXTURE", "re_private_fixture")
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		if r.Header.Get("Authorization") != "Bearer owner_fixture" {
			t.Error("missing owner auth")
		}
		if r.Method == "POST" {
			var payload map[string]any
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				t.Fatal(err)
			}
			if payload["api_key"] != "re_private_fixture" || payload["from_email"] != "noreply@example.com" || payload["enabled"] != nil {
				t.Error("incorrect secret or policy changed")
			}
		}
		w.Write([]byte(`{"settings":{"has_api_key":true,"from_email":"noreply@example.com","api_key":"re_private_fixture","masked_api_key":"re_private_fixture","enabled":false}}`))
	}))
	defer server.Close()
	prev := globalCuanClient
	globalCuanClient = &CuanClient{BaseURL: server.URL, AuthToken: "owner_fixture", HTTPClient: server.Client()}
	t.Cleanup(func() { globalCuanClient = prev })
	var out, errs bytes.Buffer
	if code := RunIntegrationCLI([]string{"resend", "set", "--key-from-env", "RESEND_FIXTURE", "--from-email", "noreply@example.com"}, &out, &errs); code != 0 {
		t.Fatal(code, errs.String())
	}
	if strings.Contains(out.String()+errs.String(), "re_private_fixture") || !json.Valid(out.Bytes()) {
		t.Fatal("secret leaked")
	}
	t.Setenv("CUAN_TASK_CAPABILITY_JSON", "bound-fixture")
	out.Reset()
	errs.Reset()
	if code := RunIntegrationCLI([]string{"resend", "set", "--from-name", "Cuan"}, &out, &errs); code != 1 || requests != 1 {
		t.Fatal("bound task could mutate owner settings")
	}
}
func TestResendCLIRejectsUnsafeInputAndFailureBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
		w.Write([]byte(`re_private_fixture`))
	}))
	defer server.Close()
	prev := globalCuanClient
	globalCuanClient = &CuanClient{BaseURL: server.URL, AuthToken: "fixture", HTTPClient: server.Client()}
	t.Cleanup(func() { globalCuanClient = prev })
	for _, args := range [][]string{{"resend", "show"}, {"resend", "set", "--api-key", "re_private_fixture"}, {"resend", "set", "--key-from-env", "MISSING_FIXTURE"}} {
		var out, errs bytes.Buffer
		if code := RunIntegrationCLI(args, &out, &errs); code == 0 {
			t.Fatal("unexpected success")
		}
		if strings.Contains(out.String()+errs.String(), "re_private_fixture") {
			t.Fatal("secret leaked in failure")
		}
	}
}
