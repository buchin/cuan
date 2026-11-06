package diagnostics

import (
	"bytes"
	"database/sql"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestTaskReadsDoNotReplaceOwnerFailureWithEmptyOrLocalData(t *testing.T) {
	for _, command := range [][]string{{"list", "--status", "doing"}, {"get", "task-one", "--json"}} {
		t.Run(command[0], func(t *testing.T) {
			calls := 0
			api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				if r.Header.Get("Authorization") != "Bearer fixture" {
					t.Error("owner identity missing")
				}
				w.WriteHeader(503)
				w.Write([]byte("PRIVATE_PROVIDER_ERROR"))
			}))
			defer api.Close()
			previousClient, previousDB := globalCuanClient, platformDB
			globalCuanClient = &CuanClient{BaseURL: api.URL, AuthToken: "fixture", HTTPClient: api.Client()}
			// An ambient DB must not be queried when the Owner endpoint is selected.
			platformDB = &sql.DB{}
			defer func() { globalCuanClient = previousClient; platformDB = previousDB }()
			var out, errOut bytes.Buffer
			if code := RunTaskCLI(command, &out, &errOut); code != 1 || calls != 1 {
				t.Fatal("failed Owner read became successful fallback", code, calls)
			}
			if out.Len() != 0 || !strings.Contains(errOut.String(), "could not be confirmed") || strings.Contains(errOut.String(), "PRIVATE_") {
				t.Fatal("misleading or private failure output")
			}
		})
	}
}
