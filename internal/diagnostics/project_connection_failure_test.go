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

func TestProjectReadsPreserveAuthoritativeEmptyAndNotFoundResults(t *testing.T) {
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/projects" {
			w.Write([]byte(`{"projects":[]}`))
			return
		}
		w.WriteHeader(404)
		w.Write([]byte("PRIVATE_ERROR_BODY"))
	}))
	defer api.Close()
	previous := globalCuanClient
	globalCuanClient = &CuanClient{BaseURL: api.URL, AuthToken: "fixture", HTTPClient: api.Client()}
	defer func() { globalCuanClient = previous }()
	listed, err := ListProjects(context.Background(), 5)
	if err != nil {
		t.Fatal("successful empty list rejected", err)
	}
	list := listed.(map[string]any)
	if list["total"] != 0 {
		t.Fatal("empty authoritative list changed", list)
	}
	missing, err := GetProject(context.Background(), "missing-project")
	if err != nil {
		t.Fatal("authoritative absence lost", err)
	}
	project := missing.(map[string]any)
	if project["error"] != "project_not_found" || project["project_id"] != "missing-project" || len(project) != 2 {
		t.Fatal("wrong or private absence data", project)
	}
}
