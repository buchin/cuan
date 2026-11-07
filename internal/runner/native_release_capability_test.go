package runner

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/buchin/cuan/internal/config"
)

func TestPollAdvertisesNativeReleaseOnlyForOptedInWorker(t *testing.T) {
	for _, enabled := range []bool{false, true} {
		t.Run(map[bool]string{false: "default off", true: "enabled"}[enabled], func(t *testing.T) {
			calls := 0
			api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				if r.Method != http.MethodPost || r.URL.Path != "/api/v1/runner/claim" {
					t.Error("unexpected poll request")
				}
				var claim ClaimTaskRequest
				if err := json.NewDecoder(r.Body).Decode(&claim); err != nil {
					t.Error(err)
				}
				if claim.SupportsNativeRelease != enabled || claim.RunnerID != "worker" || len(claim.SupportedAgents) != 1 || claim.SupportedAgents[0] != "fixture" {
					t.Error("worker capability or identity changed", claim)
				}
				w.Header().Set("Content-Type", "application/json")
				w.Write([]byte(`{"claimed":false}`))
			}))
			defer api.Close()
			cfg := &config.Config{RunnerID: "worker", RunnerServerURL: api.URL, RunnerNativeRelease: enabled, RunnerWorkerRegistryDir: t.TempDir()}
			worker := NewRunner(cfg, &detachedTestAdapter{name: "fixture"}, api.Client())
			worker.pollAndExecute(context.Background())
			if calls != 1 {
				t.Fatal("worker did not poll")
			}
		})
	}
}
