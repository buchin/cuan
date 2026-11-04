package diagnostics

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestChatRetryUsesOriginalRequestWithoutResendingMessage(t *testing.T) {
	for _, status := range []int{202, 200, 409, 503} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			calls := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				if r.Method != "POST" || r.URL.Path != "/api/v1/chat/conversations/room/requests/source/retry" || r.Header.Get("Authorization") != "Bearer fixture" {
					t.Error("unexpected request")
				}
				w.WriteHeader(status)
				w.Write([]byte(`{"request":{"request_id":"child","state":"queued","private":"must-not-echo"}}`))
			}))
			defer server.Close()
			previous := globalCuanClient
			globalCuanClient = &CuanClient{BaseURL: server.URL, AuthToken: "fixture", HTTPClient: server.Client()}
			t.Cleanup(func() { globalCuanClient = previous })
			var out, errs bytes.Buffer
			code := RunChatCLI([]string{"retry", "room", "source"}, &out, &errs)
			if calls != 1 || (status < 300) != (code == 0) {
				t.Fatal(calls, code, errs.String())
			}
			if strings.Contains(out.String()+errs.String(), "must-not-echo") {
				t.Fatal("private response leaked")
			}
		})
	}
}
