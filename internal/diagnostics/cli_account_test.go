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

func TestAccountDiagnosticsAuthorizationAndRedaction(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer fixture-token" {
			t.Error("missing authentication")
		}
		switch r.URL.Path {
		case "/api/v1/provider-accounts":
			if r.Method != "GET" {
				t.Error("list mutated state")
			}
			w.Write([]byte(`{"accounts":[{"id":"acct_fixture","state":"unavailable","credentials_present":true,"credentials":"private-token","home":"/private/profile"}]}`))
		case "/api/v1/provider-accounts/acct_fixture/usage":
			if r.Method != "GET" {
				t.Error("usage mutated state")
			}
			w.Write([]byte(`{"available":true,"models":[{"model":"gemini-test","remaining_percent":88}],"access_token":"private-token"}`))
		case "/api/v1/provider-accounts/acct_fixture/verify":
			if r.Method != "POST" || r.Body != nil && r.ContentLength != 0 {
				t.Error("verify must not accept terms/change configuration")
			}
			w.Write([]byte(`{"account":{"id":"acct_fixture","state":"verified","allow_model_fallback":false,"access_token":"private-token"}}`))
		default:
			t.Error("unexpected path", r.URL.Path)
			w.WriteHeader(404)
		}
	}))
	defer server.Close()
	client := &CuanClient{BaseURL: server.URL, AuthToken: "fixture-token", HTTPClient: server.Client()}
	previous := globalCuanClient
	globalCuanClient = client
	t.Cleanup(func() { globalCuanClient = previous })
	for _, args := range [][]string{{"list"}, {"usage", "acct_fixture"}, {"verify", "acct_fixture"}} {
		var out, errors bytes.Buffer
		if code := RunAccountCLI(args, &out, &errors); code != 0 {
			t.Fatal(code, errors.String())
		}
		if !json.Valid(out.Bytes()) || strings.Contains(out.String(), "private-token") || strings.Contains(out.String(), "/private/profile") {
			t.Fatal("unsafe or invalid response", out.String())
		}
	}
	for _, id := range []string{"", "../other", "acct?query", "acct/usage"} {
		if _, err := client.GetProviderAccountUsage(context.Background(), id); err == nil {
			t.Fatal("invalid id accepted", id)
		}
	}
}
func TestAccountDiagnosticsNeverEchoFailureBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		w.Write([]byte("private-token"))
	}))
	defer server.Close()
	client := &CuanClient{BaseURL: server.URL, AuthToken: "fixture-token", HTTPClient: server.Client()}
	_, err := client.ListProviderAccounts(context.Background())
	if err == nil || strings.Contains(err.Error(), "private-token") {
		t.Fatal("error leaked response body", err)
	}
}

func TestAccountModelSelectionUsesNativeCatalogAndPreservesSettings(t *testing.T) {
	patches := 0
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		if r.Header.Get("Authorization") != "Bearer fixture-token" {
			t.Error("missing owner authentication")
		}
		switch {
		case r.URL.Path == "/api/v1/provider-accounts":
			w.Write([]byte(`{"accounts":[{"id":"acct_fixture","label":"Existing label","credentials_present":true,"allow_failover":true}]}`))
		case r.URL.Path == "/api/v1/provider-accounts/acct_fixture/models":
			w.Write([]byte(`{"models":[{"id":"claude-sonnet-fixture"}]}`))
		case r.URL.Path == "/api/v1/provider-accounts/acct_fixture" && r.Method == http.MethodPatch:
			patches++
			var body map[string]string
			if json.NewDecoder(r.Body).Decode(&body) != nil || len(body) != 2 || body["label"] != "Existing label" || body["model_id"] != "claude-sonnet-fixture" {
				t.Error("model selection expanded mutation", body)
			}
			w.Write([]byte(`{"account":{"id":"acct_fixture","label":"Existing label","model_id":"claude-sonnet-fixture","credentials_present":true,"allow_failover":true}}`))
		default:
			t.Error("unexpected model operation", r.Method, r.URL.Path)
			w.WriteHeader(404)
		}
	}))
	defer server.Close()
	client := &CuanClient{BaseURL: server.URL, AuthToken: "fixture-token", HTTPClient: server.Client()}
	account, err := client.SetProviderAccountModel(context.Background(), "acct_fixture", "claude-sonnet-fixture")
	if err != nil || account.ModelID != "claude-sonnet-fixture" || !account.AllowFailover || patches != 1 {
		t.Fatal("model change not confirmed", account, err, patches)
	}
	if _, err := client.SetProviderAccountModel(context.Background(), "acct_fixture", "invented-model"); err == nil || patches != 1 {
		t.Fatal("invented model was applied")
	}
	before := requests
	t.Setenv("CUAN_TASK_CAPABILITY_JSON", `{"task_id":"worker"}`)
	if _, err := client.SetProviderAccountModel(context.Background(), "acct_fixture", "claude-sonnet-fixture"); err == nil || requests != before {
		t.Fatal("bound worker used Owner account settings")
	}
}
