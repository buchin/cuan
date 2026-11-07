package server

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/buchin/cuan/internal/agent"
	"github.com/buchin/cuan/internal/db"
)

func TestNativeTaskAdmissionUsesSelectedModelPool(t *testing.T) {
	for _, mode := range []string{"cooldown", "slots full", "ready"} {
		t.Run(mode, func(t *testing.T) {
			s, token, _, primary, backup, _, _ := setupFailoverFixture(t)
			t.Setenv("CUAN_ACCOUNT_MAX_SESSIONS", "2")
			t.Setenv("CUAN_ACCOUNT_CHAT_RESERVED_SESSIONS", "1")
			ctx := context.Background()
			facade := &db.Agent{ID: db.AgentIDCuan, Name: "@cuan", Provider: "agy", Harness: "agy", Enabled: true, Status: db.AgentStatusActive, FallbackModelID: "gemini-test"}
			if _, err := s.agentRepo.GetAgentByID(ctx, facade.ID); err == nil {
				if err := s.agentRepo.UpdateAgent(ctx, facade); err != nil {
					t.Fatal(err)
				}
			} else if err := s.agentRepo.CreateAgent(ctx, facade); err != nil {
				t.Fatal(err)
			}
			for _, profile := range []*agent.ProviderAccount{primary, backup} {
				if _, err := agent.UpdateProviderAccountSettings(primary.OwnerID, profile.ID, profile.Label, "claude-sonnet-test"); err != nil {
					t.Fatal(err)
				}
				if mode == "cooldown" {
					if err := agent.RecordProviderModelQuotaWithDuration(primary.OwnerID, profile.ID, "claude-sonnet-test", time.Hour); err != nil {
						t.Fatal(err)
					}
				}
				if mode == "slots full" {
					release, err := agent.TryLeaseProviderAccountForTask(primary.OwnerID, profile.ID)
					if err != nil {
						t.Fatal(err)
					}
					defer release()
				}
			}
			// Native tasks must use the same selected-model/task-slot pool as the
			// worker, even when the public agent retains an old chat fallback.
			oldFetch := fetchInboxModelUsage
			defer func() { fetchInboxModelUsage = oldFetch }()
			probes := 0
			fetchInboxModelUsage = func(context.Context, string, string) agent.AccountModelUsage {
				probes++
				return agent.AccountModelUsage{}
			}
			call := func(method, path, body string) *httptest.ResponseRecorder {
				r := httptest.NewRequest(method, path, strings.NewReader(body))
				r.Header.Set("Authorization", "Bearer "+token)
				w := httptest.NewRecorder()
				s.Router().ServeHTTP(w, r)
				return w
			}
			item := workspaceTask(t, call)
			response := call("POST", "/api/v1/tasks/"+item.ID+"/run-agent", `{"agent_id":"agent_cuan","runner_id":"cuan-runner"}`)
			if mode == "ready" {
				if response.Code != 200 {
					t.Fatal("ready task did not queue", response.Code, response.Body.String())
				}
				_, selected, release, err := agent.LeaseHealthyBrainForTask(primary.OwnerID, nil)
				if err != nil {
					t.Fatal("preflight retained worker capacity", err)
				}
				release()
				if selected.ModelID != "claude-sonnet-test" {
					t.Fatal("admission changed selected model")
				}
			} else {
				if response.Code != 409 || !strings.Contains(response.Body.String(), `"code":"provider_capacity"`) {
					t.Fatal("unavailable native capacity admitted", response.Code, response.Body.String())
				}
				attempts, err := s.taskSvc.Store().ListAttempts(ctx, item.ID)
				if err != nil || len(attempts) != 0 {
					t.Fatal("capacity wait created a failed/claimable attempt", err)
				}
			}
			if probes != 0 {
				t.Fatal("native task admission used a different chat routing pool")
			}
		})
	}
}
