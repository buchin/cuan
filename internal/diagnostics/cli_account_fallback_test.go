package diagnostics

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAccountFallbackCLIUsesNativeCatalogAndPreservesPrimary(t *testing.T) {
	patches := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer owner-fixture" {
			t.Error("missing Owner authentication")
		}
		switch r.URL.Path {
		case "/api/v1/provider-accounts":
			w.Write([]byte(`{"accounts":[{"id":"acct_fixture","label":"Brain","credentials_present":true,"model_id":"claude-sonnet-test"}]}`))
		case "/api/v1/provider-accounts/acct_fixture/models":
			w.Write([]byte(`{"models":[{"id":"claude-sonnet-test"},{"id":"gemini-test"}]}`))
		case "/api/v1/provider-accounts/acct_fixture/model-fallback":
			patches++
			var body struct {
				Enabled bool   `json:"enabled"`
				Model   string `json:"model_id"`
			}
			if r.Method != "PATCH" || json.NewDecoder(r.Body).Decode(&body) != nil || !body.Enabled || body.Model != "gemini-test" {
				t.Error("wrong policy operation")
			}
			w.Write([]byte(`{"account":{"id":"acct_fixture","label":"Brain","credentials_present":true,"model_id":"claude-sonnet-test","fallback_model_id":"gemini-test","allow_model_fallback":true,"access_token":"private"}}`))
		default:
			t.Error("unexpected endpoint", r.URL.Path)
			w.WriteHeader(404)
		}
	}))
	defer server.Close()
	client := &CuanClient{BaseURL: server.URL, AuthToken: "owner-fixture", HTTPClient: server.Client()}
	a, err := client.SetProviderAccountFallbackModel(context.Background(), "acct_fixture", "gemini-test")
	if err != nil || a.FallbackModelID != "gemini-test" || a.ModelID != "claude-sonnet-test" || patches != 1 {
		t.Fatal("fallback policy not confirmed", a, err)
	}
	for _, model := range []string{"unknown-model", "claude-sonnet-test"} {
		if _, err := client.SetProviderAccountFallbackModel(context.Background(), "acct_fixture", model); err == nil || patches != 1 {
			t.Fatal("invalid model changed policy")
		}
	}
	t.Setenv("CUAN_TASK_CAPABILITY_JSON", `{"task_id":"bound"}`)
	if _, err := client.SetProviderAccountFallbackModel(context.Background(), "acct_fixture", "gemini-test"); err == nil || patches != 1 {
		t.Fatal("task capability changed Owner routing policy")
	}
}
