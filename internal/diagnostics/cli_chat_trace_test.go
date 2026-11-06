package diagnostics

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestChatTraceReadsExplicitCursorWithoutMutation(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.Method != "GET" || r.URL.Path != "/api/v1/chat/conversations/room/requests/request/steps" || r.URL.Query().Get("after_seq") != "100" || r.Header.Get("Authorization") != "Bearer fixture" {
			t.Error("unexpected request")
		}
		w.Write([]byte(`{"steps":[{"seq":101,"kind":"context","text":"saved stage","at":"2026-11-06T00:00:00Z"}],"next_seq":101,"has_more":true}`))
	}))
	defer server.Close()
	previous := globalCuanClient
	globalCuanClient = &CuanClient{BaseURL: server.URL, AuthToken: "fixture", HTTPClient: server.Client()}
	defer func() { globalCuanClient = previous }()
	var out, errs bytes.Buffer
	code := RunChatCLI([]string{"trace", "room", "request", "--after-seq", "100", "--limit", "7"}, &out, &errs)
	if code != 0 || calls != 1 || !strings.Contains(out.String(), `"next_seq":101`) {
		t.Fatal(code, calls, out.String(), errs.String())
	}
	if code := RunChatCLI([]string{"trace", "room", "request", "--limit", "999"}, &out, &errs); code != 2 || calls != 1 {
		t.Fatal("bad limit reached network")
	}
}
