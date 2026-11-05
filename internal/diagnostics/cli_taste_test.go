package diagnostics

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestTasteCLIUsesAuthenticatedMemberAndRejectsBoundMutation(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.Header.Get("Authorization") != "Bearer fixture" || r.URL.Path != "/api/v1/taste/items" || r.Method != "POST" {
			t.Error("incorrect authenticated preference request")
		}
		var payload map[string]string
		if json.NewDecoder(r.Body).Decode(&payload) != nil || payload["content"] != "Use ane and ente" || payload["owner_id"] != "" {
			t.Error("wrong preference or supplied identity")
		}
		w.WriteHeader(201)
		w.Write([]byte(`{"item":{"id":"personal"}}`))
	}))
	defer server.Close()
	prev := globalCuanClient
	globalCuanClient = &CuanClient{BaseURL: server.URL, AuthToken: "fixture", HTTPClient: server.Client()}
	t.Cleanup(func() { globalCuanClient = prev })
	var out, errs bytes.Buffer
	args := []string{"add", "--topic", "Voice", "--content", "Use ane and ente"}
	if code := RunTasteCLI(args, &out, &errs); code != 0 || calls != 1 {
		t.Fatal(code, errs.String())
	}
	t.Setenv("CUAN_TASK_CAPABILITY_JSON", "bound-fixture")
	if code := RunTasteCLI(args, &out, &errs); code != 1 || calls != 1 {
		t.Fatal("bound task mutated private taste", code, calls)
	}
}
