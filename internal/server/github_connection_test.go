package server

import (
	"net/http/httptest"
	"strings"
	"testing"
)

func TestGitHubConnectionOwnerEndpoints(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	s, token := setupTestChatServer(t)
	call := func(method, path, body string, authenticated bool) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		if authenticated {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		response := httptest.NewRecorder()
		s.Router().ServeHTTP(response, req)
		return response
	}
	for _, method := range []string{"GET", "PUT", "DELETE"} {
		if got := call(method, "/api/v1/integrations/github", "", false); got.Code != 401 {
			t.Fatal("unprotected connection", method, got.Code)
		}
	}
	status := call("GET", "/api/v1/integrations/github", "", true)
	if status.Code != 200 || !strings.Contains(status.Body.String(), `"connected":false`) {
		t.Fatal(status.Code, status.Body.String())
	}
	if got := call("PUT", "/api/v1/integrations/github", `{"unexpected":"value"}`, true); got.Code != 400 {
		t.Fatal("unknown input accepted", got.Code)
	}
	if got := call("GET", "/api/v1/integrations/github/repositories?page=-1", "", true); got.Code != 400 {
		t.Fatal("invalid page accepted", got.Code)
	}
	if got := call("GET", "/api/v1/integrations/github/repositories", "", true); got.Code != 409 {
		t.Fatal("missing connection accepted", got.Code)
	}
	if got := call("DELETE", "/api/v1/integrations/github", "", true); got.Code != 200 {
		t.Fatal("idempotent disconnect failed", got.Code)
	}
}
