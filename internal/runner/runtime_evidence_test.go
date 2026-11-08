package runner

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/buchin/cuan/internal/task"
)

// Separate service instances represent API and chat-worker processes. A restart
// must not erase fresh authenticated readiness; omission/expiry must erase it.
func TestRunnerRuntimeEvidenceCrossesServiceBoundaries(t *testing.T) {
	ctx := context.Background()
	store := NewMemoryStore(task.NewMemoryStore())
	api := NewService(store, store.taskStore)
	worker := NewService(store, store.taskStore)
	browser := &task.VerificationRuntime{Fingerprint: strings.Repeat("a", 64), Version: "fixture", CheckedAt: time.Now().UTC()}
	preparation := &task.DependencyPreparationRuntime{Policy: task.PHPPreparationPolicy, CheckedAt: time.Now().UTC()}
	for _, mode := range []string{"ready", "omitted", "unknown policy", "expired", "future"} {
		t.Run(mode, func(t *testing.T) {
			request := HeartbeatRequest{RunnerID: "runner", BrowserRuntime: browser, DependencyPreparation: preparation}
			b, p := *browser, *preparation
			request.BrowserRuntime, request.DependencyPreparation = &b, &p
			switch mode {
			case "omitted":
				request.BrowserRuntime = nil
				request.DependencyPreparation = nil
			case "unknown policy":
				p.Policy = "unknown"
				b.Fingerprint = "unknown"
			case "expired":
				p.CheckedAt = time.Now().Add(-time.Hour)
				b.CheckedAt = p.CheckedAt
			case "future":
				p.CheckedAt = time.Now().Add(time.Hour)
				b.CheckedAt = p.CheckedAt
			}
			if _, err := api.Heartbeat(ctx, request, time.Minute); err != nil {
				t.Fatal(err)
			}
			if (worker.BrowserRuntime(ctx, "runner") != nil) != (mode == "ready") || (worker.DependencyPreparationRuntime(ctx, "runner") != nil) != (mode == "ready") {
				t.Fatal("readiness lost across services or invalid evidence retained")
			}
		})
	}
}
