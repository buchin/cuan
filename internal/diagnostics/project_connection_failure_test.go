package diagnostics

import (
	"context"
	"database/sql"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestProjectReadsDoNotInventMissingProjectsOnOwnerFailure(t *testing.T) {
	for _, scenario := range []string{"list", "get"} {
		t.Run(scenario, func(t *testing.T) {
			calls := 0
			api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				if r.Method != "GET" || r.Header.Get("Authorization") != "Bearer fixture" {
					t.Error("wrong project connection")
				}
				w.WriteHeader(503)
				w.Write([]byte("PRIVATE_UPSTREAM_ERROR"))
			}))
			defer api.Close()
			previousClient, previousDB := globalCuanClient, platformDB
			globalCuanClient = &CuanClient{BaseURL: api.URL, AuthToken: "fixture", HTTPClient: api.Client()}
			platformDB = &sql.DB{}
			defer func() { globalCuanClient = previousClient; platformDB = previousDB }()
			var result any
			var err error
			if scenario == "list" {
				result, err = ListProjects(context.Background(), 5)
			} else {
				result, err = GetProject(context.Background(), "registered-project")
			}
			if calls != 1 || result != nil || err == nil || !strings.Contains(err.Error(), "could not be confirmed") || strings.Contains(err.Error(), "PRIVATE_") {
				t.Fatal("Owner failure became fabricated project state", calls, result, err)
			}
		})
	}
}
