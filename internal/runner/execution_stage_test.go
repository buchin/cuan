package runner

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/buchin/cuan/internal/agent"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestAgentStageRecordsBudgetAndNativeOutcomeWithoutProviderText(t *testing.T) {
	deadline := time.Now().Add(time.Minute)
	ctx, cancel := context.WithDeadline(context.Background(), deadline)
	defer cancel()
	started := time.Now().Add(-time.Second)
	start := agentStageRecord(ctx, started, false, nil, nil)
	if start.State != "started" || !start.Deadline.Equal(deadline) || start.ElapsedMS != 0 {
		t.Fatal(start)
	}
	for _, test := range []struct {
		status string
		err    error
		want   string
	}{{"TIMEOUT", nil, "timeout"}, {"INTERRUPTED", nil, "interrupted"}, {"SUCCESS", nil, "returned"}, {"FAILED", nil, "failed"}, {"CANCELED", nil, "cancelled"}, {"FAILED", errors.New("secret-provider-text"), "error"}} {
		result := &agent.ExecutionResult{Status: test.status, Error: "secret-provider-text", Stdout: "secret-provider-text"}
		record := agentStageRecord(ctx, started, true, result, test.err)
		data, _ := json.Marshal(record)
		if record.Outcome != test.want || record.ElapsedMS < 1000 || strings.Contains(string(data), "secret-provider-text") {
			t.Fatal(string(data))
		}
	}
	expired, stop := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
	defer stop()
	if agentStageRecord(expired, started, true, nil, nil).Outcome != "timeout" {
		t.Fatal("lost cancelled-context timeout")
	}
	cancelled, stop2 := context.WithCancel(ctx)
	stop2()
	if agentStageRecord(cancelled, started, true, nil, nil).Outcome != "cancelled" {
		t.Fatal("lost cancellation")
	}
}

func TestAgentStageObservationSurvivesDeadlineWithoutExtendingExecution(t *testing.T) {
	var got StreamLogsRequest
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, q *http.Request) {
		if q.Method != "POST" || q.URL.Path != "/api/v1/runner/attempts/exact-attempt/logs" {
			t.Error("wrong stage route")
		}
		if err := json.NewDecoder(q.Body).Decode(&got); err != nil {
			t.Error(err)
		}
		w.Write([]byte(`{}`))
	}))
	defer api.Close()
	r := outboxRunner(t, api.URL)
	deadline := time.Now().Add(-time.Second)
	ctx, cancel := context.WithDeadline(context.Background(), deadline)
	defer cancel()
	r.logAgentStage(ctx, "exact-attempt", deadline.Add(-time.Minute), true, nil, context.DeadlineExceeded)
	if ctx.Err() != context.DeadlineExceeded || got.AttemptID != "exact-attempt" || len(got.Logs) != 1 || got.Logs[0].Stream != "system" {
		t.Fatal(ctx.Err(), got)
	}
	var record executionStageRecord
	if err := json.Unmarshal([]byte(got.Logs[0].Line), &record); err != nil {
		t.Fatal(err)
	}
	if record.Outcome != "timeout" || !record.Deadline.Equal(deadline) {
		t.Fatal(record)
	}
}
