package diagnostics

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestChatManageUsesExactOwnerTargetAndRequiresInactiveRequest(t *testing.T) {
	const room = "11111111-1111-4111-8111-111111111111"
	const request = "22222222-2222-4222-8222-222222222222"
	for _, scenario := range []string{"delete", "active", "cancel", "unavailable", "bad receipt", "bound"} {
		t.Run(scenario, func(t *testing.T) {
			t.Setenv("CUAN_TASK_CAPABILITY_FILE", "")
			t.Setenv("CUAN_TASK_CAPABILITY_JSON", "")
			if scenario == "bound" {
				t.Setenv("CUAN_TASK_CAPABILITY_JSON", "{}")
			}
			reads, writes := 0, 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Header.Get("Authorization") != "Bearer fixture" {
					t.Error("wrong identity")
				}
				if r.Method == "GET" {
					reads++
					if r.URL.Path != "/api/v1/chat/conversations/"+room+"/messages" {
						t.Error("wrong read")
					}
					if scenario == "unavailable" {
						w.WriteHeader(503)
						fmt.Fprint(w, "private diagnostic")
						return
					}
					state := "completed"
					if scenario == "active" {
						state = "running"
					}
					fmt.Fprintf(w, `{"messages":[],"requests":[{"request_id":"%s","state":"%s"}]}`, request, state)
					return
				}
				writes++
				if scenario == "cancel" {
					if r.Method != "POST" || r.URL.Path != "/api/v1/chat/conversations/"+room+"/requests/"+request+"/cancel" {
						t.Error("wrong cancellation target")
					}
					fmt.Fprint(w, `{"status":"reconciled"}`)
				} else {
					if r.Method != "DELETE" || r.URL.Path != "/api/v1/rooms/"+room {
						t.Error("wrong deletion target")
					}
					id := room
					if scenario == "bad receipt" {
						id = request
					}
					fmt.Fprintf(w, `{"status":"deleted","id":"%s"}`, id)
				}
			}))
			defer server.Close()
			old := globalCuanClient
			globalCuanClient = &CuanClient{BaseURL: server.URL, AuthToken: "fixture", HTTPClient: server.Client()}
			t.Cleanup(func() { globalCuanClient = old })
			args := []string{"delete", room}
			if scenario == "cancel" {
				args = []string{"cancel", room, request}
			}
			var out, errs bytes.Buffer
			code := RunChatCLI(args, &out, &errs)
			wantSuccess := scenario == "delete" || scenario == "cancel"
			if (code == 0) != wantSuccess {
				t.Fatal(code, errs.String())
			}
			if scenario == "active" || scenario == "unavailable" || scenario == "bound" {
				if writes != 0 {
					t.Fatal("unsafe write")
				}
			} else if writes != 1 {
				t.Fatal("missing or repeated write", writes)
			}
			if scenario == "bound" && reads != 0 {
				t.Fatal("worker used owner read")
			}
			if strings.Contains(out.String()+errs.String(), "private diagnostic") {
				t.Fatal("private response echoed")
			}
		})
	}
}
