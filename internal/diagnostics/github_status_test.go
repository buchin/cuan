package diagnostics

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestGitHubConnectionCLIUsesOnlyAuthenticatedRedactedRead(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		if r.Method != "GET" || r.URL.Path != "/api/v1/integrations/github" || r.Header.Get("Authorization") != "Bearer cuan_owner_fixture" {
			t.Error("wrong owner request")
		}
		w.Write([]byte(`{"connected":true,"token":"provider_fixture_secret","account":{"login":"connected-owner","id":42,"token":"provider_fixture_secret"}}`))
	}))
	defer server.Close()
	previous := globalCuanClient
	globalCuanClient = &CuanClient{BaseURL: server.URL, AuthToken: "cuan_owner_fixture", HTTPClient: server.Client()}
	t.Cleanup(func() { globalCuanClient = previous })
	var out, errs bytes.Buffer
	if code := RunIntegrationCLI([]string{"github", "show"}, &out, &errs); code != 0 {
		t.Fatal(code, errs.String())
	}
	if !strings.Contains(out.String(), `"login":"connected-owner"`) || strings.Contains(out.String()+errs.String(), "provider_fixture_secret") {
		t.Fatal("connection missing or secret exposed")
	}
	for _, args := range [][]string{{"github", "connect"}, {"github", "show", "--token", "provider_fixture_secret"}} {
		out.Reset()
		errs.Reset()
		if code := RunIntegrationCLI(args, &out, &errs); code != 2 {
			t.Fatal("unsupported mutation or secret argument accepted")
		}
		if strings.Contains(out.String()+errs.String(), "provider_fixture_secret") {
			t.Fatal("secret input echoed")
		}
	}
	if requests != 1 {
		t.Fatal("unexpected integration mutation")
	}
	out.Reset()
	errs.Reset()
	if code := RunMCP(strings.NewReader("{\"jsonrpc\":\"2.0\",\"id\":1,\"method\":\"tools/call\",\"params\":{\"name\":\"get_github_connection\",\"arguments\":{}}}\n"), &out, &errs); code != 0 {
		t.Fatal(code, errs.String())
	}
	if !strings.Contains(out.String(), "connected-owner") || strings.Contains(out.String()+errs.String(), "provider_fixture_secret") || requests != 2 {
		t.Fatal("MCP read failed or exposed credentials")
	}
}

func TestGitHubConnectionStatusDoesNotEchoFailureBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(503)
		w.Write([]byte("provider_fixture_secret"))
	}))
	defer server.Close()
	client := &CuanClient{BaseURL: server.URL, AuthToken: "owner", HTTPClient: server.Client()}
	_, err := client.GitHubConnection(context.Background())
	if err == nil || strings.Contains(err.Error(), "provider_fixture_secret") {
		t.Fatal("failed response accepted or echoed")
	}
}
