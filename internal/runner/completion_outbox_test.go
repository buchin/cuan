package runner

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/buchin/cuan/internal/config"
	"github.com/buchin/cuan/internal/task"
)

func outboxRunner(t *testing.T, url string) *Runner {
	t.Helper()
	base := t.TempDir()
	return NewRunner(&config.Config{RunnerID: "native", RunnerServerURL: url, WorkspacesDir: filepath.Join(base, "workspaces"), RunnerWorkerRegistryDir: filepath.Join(base, "workers")}, &detachedTestAdapter{})
}
func outboxPayload() CompleteAttemptRequest {
	return CompleteAttemptRequest{RunnerID: "native", Status: task.AttemptStatusSucceeded, ExitCode: 0, TestSummary: "Project verification: registered PHPUnit targets (exit 0)\nOriginal checks passed"}
}
func ackOutbox(w http.ResponseWriter) {
	_ = json.NewEncoder(w).Encode(task.TaskAttempt{ID: "attempt", TaskID: "task", RunnerID: "native", Status: task.AttemptStatusSucceeded})
}

func TestCompletionOutboxPersistsBeforeRPCAndReplaysAfterRestart(t *testing.T) {
	ctx := context.Background()
	var calls atomic.Int32
	var bodies [][]byte
	var runner *Runner
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, q *http.Request) {
		b, _ := io.ReadAll(q.Body)
		bodies = append(bodies, b)
		if _, err := os.Stat(filepath.Join(runner.completionDirectory(), "attempt.json")); err != nil {
			t.Error("native result not durable before handoff", err)
		}
		if calls.Add(1) == 1 {
			w.WriteHeader(503)
			return
		}
		ackOutbox(w)
	}))
	defer server.Close()
	runner = outboxRunner(t, server.URL)
	payload := outboxPayload()
	if runner.deliverCompletion(ctx, "attempt", "task", payload) == nil {
		t.Fatal("503 treated as completed handoff")
	}
	root, err := runner.openCompletionOutbox()
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	record, err := readCompletion(root, "attempt.json")
	if err != nil {
		t.Fatal(err)
	}
	if record.DeliveryAttempts != 1 || record.PayloadHash == "" || len(runner.pendingCompletionIDs()) != 1 {
		t.Fatal("pending result lost", record.DeliveryAttempts)
	}
	record.NextAttemptAt = time.Now().Add(-time.Second)
	if err := writeCompletion(root, "attempt.json", record); err != nil {
		t.Fatal(err)
	}
	restarted := NewRunner(runner.cfg, &detachedTestAdapter{})
	restarted.replayCompletions(ctx)
	if calls.Load() != 2 || !bytes.Equal(bodies[0], bodies[1]) {
		t.Fatal("recovery changed native payload or repeated agent work")
	}
	saved, err := readCompletion(root, "acknowledged/attempt.json")
	if err != nil || saved.AcknowledgedState != task.AttemptStatusSucceeded || saved.Ignored {
		t.Fatal("missing exact server receipt", err)
	}
	if len(restarted.pendingCompletionIDs()) != 0 {
		t.Fatal("acknowledged task still renews a lease")
	}
	if err := restarted.deliverCompletion(ctx, "attempt", "task", payload); err != nil || calls.Load() != 2 {
		t.Fatal("acknowledged result resent", err)
	}
	payload.TestSummary = "Different result"
	if restarted.deliverCompletion(ctx, "attempt", "task", payload) == nil {
		t.Fatal("different result overwrote original")
	}
}

func TestCompletionOutboxKeepsRejectionsAndRejectsFalseAcknowledgements(t *testing.T) {
	for _, scenario := range []string{"conflict", "bad request", "revoked", "wrong attempt", "wrong runner", "invalid body", "unknown state", "ignored terminal"} {
		t.Run(scenario, func(t *testing.T) {
			var calls atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, q *http.Request) {
				calls.Add(1)
				switch scenario {
				case "conflict":
					w.WriteHeader(409)
				case "bad request":
					w.WriteHeader(400)
				case "revoked":
					w.WriteHeader(401)
				case "wrong attempt":
					_ = json.NewEncoder(w).Encode(task.TaskAttempt{ID: "other", TaskID: "task", RunnerID: "native", Status: task.AttemptStatusSucceeded})
				case "wrong runner":
					_ = json.NewEncoder(w).Encode(task.TaskAttempt{ID: "attempt", TaskID: "task", RunnerID: "other", Status: task.AttemptStatusSucceeded})
				case "invalid body":
					_, _ = w.Write([]byte("ok"))
				case "unknown state":
					_ = json.NewEncoder(w).Encode(task.TaskAttempt{ID: "attempt", TaskID: "task", RunnerID: "native", Status: "unknown"})
				case "ignored terminal":
					_ = json.NewEncoder(w).Encode(map[string]any{"ignored": true, "attempt": task.TaskAttempt{ID: "attempt", TaskID: "task", RunnerID: "native", Status: task.AttemptStatusInterrupted}})
				}
			}))
			defer server.Close()
			r := outboxRunner(t, server.URL)
			err := r.deliverCompletion(context.Background(), "attempt", "task", outboxPayload())
			root, openErr := r.openCompletionOutbox()
			if openErr != nil {
				t.Fatal(openErr)
			}
			defer root.Close()
			if scenario == "ignored terminal" {
				saved, readErr := readCompletion(root, "acknowledged/attempt.json")
				if err != nil || readErr != nil || !saved.Ignored || saved.AcknowledgedState != task.AttemptStatusInterrupted {
					t.Fatal("ignored terminal misrepresented as accepted success", err, readErr)
				}
			} else if err == nil {
				t.Fatal("unacknowledged native result discarded")
			}
			if scenario == "conflict" || scenario == "bad request" || scenario == "revoked" {
				saved, readErr := readCompletion(root, "rejected/attempt.json")
				if readErr != nil || saved.HTTPStatus < 400 || saved.PayloadHash == "" {
					t.Fatal("rejected result not retained", readErr)
				}
				r.replayCompletions(context.Background())
				if calls.Load() != 1 {
					t.Fatal("permanent rejection blindly retried")
				}
			}
			if scenario == "wrong attempt" || scenario == "wrong runner" || scenario == "invalid body" || scenario == "unknown state" {
				if _, readErr := readCompletion(root, "attempt.json"); readErr != nil {
					t.Fatal("missing pending result", readErr)
				}
			}
		})
	}
}

func TestCompletionOutboxDoesNotOverlapSurvivingWorker(t *testing.T) {
	started, release, done := make(chan struct{}), make(chan struct{}), make(chan error, 1)
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, q *http.Request) { calls.Add(1); close(started); <-release; ackOutbox(w) }))
	defer server.Close()
	r := outboxRunner(t, server.URL)
	go func() { done <- r.deliverCompletion(context.Background(), "attempt", "task", outboxPayload()) }()
	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("handoff did not start")
	}
	restarted := NewRunner(r.cfg, &detachedTestAdapter{})
	restarted.replayCompletions(context.Background())
	if calls.Load() != 1 {
		t.Fatal("second supervisor overlapped native handoff")
	}
	close(release)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}

func TestCompletionOutboxScopesConnectionAndBoundsLeaseRenewal(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, q *http.Request) { w.WriteHeader(503) }))
	defer server.Close()
	r := outboxRunner(t, server.URL)
	_ = r.deliverCompletion(context.Background(), "attempt", "task", outboxPayload())
	root, _ := r.openCompletionOutbox()
	defer root.Close()
	record, err := readCompletion(root, "attempt.json")
	if err != nil {
		t.Fatal(err)
	}
	record.CreatedAt = time.Now().Add(-3 * time.Minute)
	if err := writeCompletion(root, "attempt.json", record); err != nil {
		t.Fatal(err)
	}
	if len(r.pendingCompletionIDs()) != 0 {
		t.Fatal("expired handoff kept execution alive forever")
	}
	cfg := *r.cfg
	cfg.RunnerServerURL = "https://different.invalid"
	moved := NewRunner(&cfg, &detachedTestAdapter{})
	if len(moved.pendingCompletionIDs()) != 0 || moved.completionDirectory() == r.completionDirectory() {
		t.Fatal("results migrated to a different connection")
	}
	record.Payload.TestSummary = "Tampered"
	if err := writeCompletion(root, "attempt.json", record); err != nil {
		t.Fatal(err)
	}
	if _, err := readCompletion(root, "attempt.json"); err == nil {
		t.Fatal("changed saved payload accepted")
	}
}

func TestCompletionOutboxRenewsOnlyPendingDeliveryInHeartbeat(t *testing.T) {
	requests := make(chan HeartbeatRequest, 2)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, q *http.Request) {
		if q.URL.Path == "/api/v1/runner/heartbeat" {
			var request HeartbeatRequest
			_ = json.NewDecoder(q.Body).Decode(&request)
			requests <- request
			_ = json.NewEncoder(w).Encode(HeartbeatResponse{Status: "ok", LeaseDurationSec: 30})
			return
		}
		w.WriteHeader(503)
	}))
	defer server.Close()
	r := outboxRunner(t, server.URL)
	r.browserNextProbe = time.Now().Add(time.Hour)
	_ = r.deliverCompletion(context.Background(), "attempt", "task", outboxPayload())
	r.sendHeartbeat(context.Background(), "fixture")
	first := <-requests
	if len(first.ActiveAttemptIDs) != 1 || first.ActiveAttemptIDs[0] != "attempt" || first.CurrentAttemptID != "attempt" {
		t.Fatal("handoff lease lost before retry")
	}
	root, _ := r.openCompletionOutbox()
	defer root.Close()
	record, _ := readCompletion(root, "attempt.json")
	record.CreatedAt = time.Now().Add(-3 * time.Minute)
	if err := writeCompletion(root, "attempt.json", record); err != nil {
		t.Fatal(err)
	}
	r.sendHeartbeat(context.Background(), "fixture")
	second := <-requests
	if len(second.ActiveAttemptIDs) != 0 || second.CurrentAttemptID != "" {
		t.Fatal("saved file became an indefinitely active agent")
	}
}
