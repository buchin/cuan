package server

import (
	"context"
	"encoding/json"
	"log"
	"strings"
	"sync"
	"time"
)

type inboxRequestTraceKey struct{}
type inboxProviderAccountTraceKey struct{}
type inboxRequestTrace struct{ RequestID, RoomID, AgentID string }

type inboxResponseInput struct {
	PromptBytes    int
	ContextBytes   int
	CandidateBytes int
}

type inboxResponseStageRecord struct {
	RequestID      string `json:"request_id"`
	RoomID         string `json:"room_id"`
	AgentID        string `json:"agent_id"`
	Stage          string `json:"stage"`
	Provider       string `json:"provider"`
	Release        string `json:"release"`
	Outcome        string `json:"outcome"`
	ElapsedMS      int64  `json:"elapsed_ms"`
	BudgetMS       int64  `json:"budget_ms,omitempty"`
	FailureKey     string `json:"failure_key,omitempty"`
	AccountID      string `json:"account_id,omitempty"`
	RequestedModel string `json:"requested_model"`
	PromptBytes    int    `json:"prompt_bytes"`
	ContextBytes   int    `json:"context_bytes"`
	CandidateBytes int    `json:"candidate_bytes,omitempty"`
}

// Aggregate counters survive the UI's bounded activity timeline. Field-presence
// counts describe the native contract without archiving private tool content.
func traceInboxRuntimeSummary(ctx context.Context, phase, release string, runtime *inboxRuntimeStream, reason string) {
	trace, ok := ctx.Value(inboxRequestTraceKey{}).(inboxRequestTrace)
	if !ok || trace.RequestID == "" {
		return
	}
	record := struct {
		RequestID    string `json:"request_id"`
		Stage        string `json:"stage"`
		Release      string `json:"release"`
		Reason       string `json:"reason,omitempty"`
		ToolCalls    int    `json:"tool_calls"`
		ToolFailures int    `json:"tool_failures"`
		DetailEvents int    `json:"tool_detail_events"`
		InputEvents  int    `json:"tool_input_events"`
		OutputEvents int    `json:"tool_output_events"`
		ErrorEvents  int    `json:"tool_error_events"`
		TraceLimited bool   `json:"trace_limited"`
		ExitSignal   int    `json:"exit_signal,omitempty"`
	}{trace.RequestID, phase, release, reason, runtime.toolCalls, runtime.toolFailures, runtime.toolDetailEvents, runtime.toolInputEvents, runtime.toolOutputEvents, runtime.toolErrorEvents, runtime.truncated, runtime.exitSignal}
	raw, _ := json.Marshal(record)
	log.Printf("inbox runtime summary %s", raw)
}

func inboxResponseStageOutcome(parent, stage context.Context, failed bool) string {
	if parent.Err() == context.DeadlineExceeded {
		return "request_deadline"
	}
	if parent.Err() != nil {
		return "request_canceled"
	}
	if stage.Err() == context.DeadlineExceeded {
		return "stage_deadline"
	}
	if stage.Err() != nil {
		return "stage_canceled"
	}
	if failed {
		return "failed"
	}
	return "completed"
}

// Two bounded records per provider stage, sharing the durable request identity.
// Never serialize the prompt, provider output, environment, profile or errors.
func traceInboxResponseStage(parent, stage context.Context, phase, provider, release string, input inboxResponseInput) func(bool) {
	trace, ok := parent.Value(inboxRequestTraceKey{}).(inboxRequestTrace)
	if !ok || trace.RequestID == "" {
		return func(bool) {}
	}
	started := time.Now()
	record := inboxResponseStageRecord{RequestID: trace.RequestID, RoomID: trace.RoomID, AgentID: trace.AgentID, Stage: phase, Provider: provider, Release: release, Outcome: "started"}
	record.AccountID, _ = parent.Value(inboxProviderAccountTraceKey{}).(string)
	record.RequestedModel, _ = parent.Value(inboxModelKey{}).(string)
	if record.RequestedModel = strings.TrimSpace(record.RequestedModel); record.RequestedModel == "" {
		record.RequestedModel = "auto"
	}
	record.PromptBytes = max(0, input.PromptBytes)
	record.ContextBytes = max(0, input.ContextBytes)
	record.CandidateBytes = max(0, input.CandidateBytes)
	if deadline, ok := stage.Deadline(); ok {
		record.BudgetMS = max(int64(0), time.Until(deadline).Milliseconds())
	}
	emit := func() { raw, _ := json.Marshal(record); log.Printf("inbox response stage %s", raw) }
	emit()
	var once sync.Once
	return func(failed bool) {
		once.Do(func() {
			record.Outcome = inboxResponseStageOutcome(parent, stage, failed)
			if record.Outcome != "completed" {
				record.FailureKey = "inbox_response:" + record.Provider + ":" + record.Stage + ":" + record.Outcome
			}
			record.ElapsedMS = time.Since(started).Milliseconds()
			emit()
		})
	}
}
