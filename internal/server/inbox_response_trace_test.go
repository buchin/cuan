package server

import (
	"bytes"
	"context"
	"encoding/json"
	"log"
	"strings"
	"testing"
	"time"

	"github.com/buchin/cuan/internal/db"
)

func TestDurableResponseInputContainsOnlySizesAndKnownStage(t *testing.T) {
	var kind, value string
	traceInboxResponseInput(func(k, v string) { kind, value = k, v }, "PRIVATE_MODEL_TEXT", inboxResponseInput{PromptBytes: 1200, ContextBytes: 400, CandidateBytes: -2})
	if kind != "response_input" || !strings.Contains(value, "prompt 1200 bytes; context 400 bytes; repair candidate 0 bytes") || !strings.Contains(value, "Stage unknown") || strings.Contains(value, "PRIVATE") {
		t.Fatal("private text or incorrect counters in durable trace", kind, value)
	}
	traceInboxResponseInput(nil, "task_review", inboxResponseInput{})
}

func TestReviewContractTraceDistinguishesFallbackWithoutPrivateEvidence(t *testing.T) {
	var output bytes.Buffer
	previous := log.Writer()
	log.SetOutput(&output)
	defer log.SetOutput(previous)
	decision := inboxReviewEvaluation{VisualInspection: "PRIVATE_IMAGE_DESCRIPTION", Explanation: "PRIVATE_CHAT_CONTENT", Decision: "revise", RemainingWork: "implementation", RequestedOutcome: "unfinished", ManagerAction: "replan", SourceReview: "PRIVATE_INVALID_ENUM"}
	traceInboxReviewContract(db.InboxRequest{}, 100, true, true, decision)
	if output.Len() != 0 {
		t.Fatal("uncorrelated review trace")
	}
	traceInboxReviewContract(db.InboxRequest{RequestID: "review-fixture", RoomID: "room-fixture"}, 100, false, true, decision)
	line := output.String()
	if strings.Contains(line, "PRIVATE_") {
		t.Fatal("private review evidence logged")
	}
	var record map[string]any
	if err := json.Unmarshal([]byte(line[strings.IndexByte(line, '{'):]), &record); err != nil {
		t.Fatal(err)
	}
	if record["request_id"] != "review-fixture" || record["room_id"] != "room-fixture" || record["structured_valid"] != false || record["parsed"] != true || record["visual_inspection_present"] != true || record["response_bytes"] != float64(100) {
		t.Fatal("lost contract diagnosis", record)
	}
	if record["decision"] != "revise" || record["remaining_work"] != "implementation" || record["requested_outcome"] != "unfinished" || record["manager_action"] != "replan" || record["source_review"] != "unknown" {
		t.Fatal("lost pre-guard classification or leaked malformed enum", record)
	}
}

func TestInboxResponseStageDistinguishesParentAndFormattingDeadline(t *testing.T) {
	parent := context.Background()
	stage, cancel := context.WithDeadline(parent, time.Now().Add(-time.Second))
	defer cancel()
	if got := inboxResponseStageOutcome(parent, stage, true); got != "stage_deadline" {
		t.Fatal(got)
	}
	if got := inboxResponseStageOutcome(stage, stage, true); got != "request_deadline" {
		t.Fatal(got)
	}
	canceled, stop := context.WithCancel(parent)
	stop()
	if got := inboxResponseStageOutcome(canceled, canceled, true); got != "request_canceled" {
		t.Fatal(got)
	}
	if got := inboxResponseStageOutcome(parent, parent, false); got != "completed" {
		t.Fatal(got)
	}
	if got := inboxResponseStageOutcome(parent, parent, true); got != "failed" {
		t.Fatal(got)
	}
}

func TestInboxResponseStageCarriesCorrelationWithoutPrivateContext(t *testing.T) {
	var output bytes.Buffer
	previous := log.Writer()
	log.SetOutput(&output)
	defer log.SetOutput(previous)
	ctx := context.WithValue(context.Background(), inboxRequestTraceKey{}, inboxRequestTrace{RequestID: "request-fixture", RoomID: "room-fixture", AgentID: "agent-fixture"})
	type privateKey struct{}
	ctx = context.WithValue(ctx, privateKey{}, "PRIVATE_PROMPT_AND_PROFILE")
	ctx = context.WithValue(ctx, inboxProviderAccountTraceKey{}, "acct-fixture")
	ctx = context.WithValue(ctx, inboxModelKey{}, "model-fixture")
	stage, cancel := inboxFormatRepairContext(ctx)
	defer cancel()
	finish := traceInboxResponseStage(ctx, stage, "format_repair", "agy", "release-fixture", inboxResponseInput{PromptBytes: 2048, CandidateBytes: 100})
	finish(true)
	finish(false)
	lines := strings.Split(strings.TrimSpace(output.String()), "\n")
	if len(lines) != 2 || strings.Contains(output.String(), "PRIVATE_PROMPT_AND_PROFILE") {
		t.Fatal("duplicate or private trace", output.String())
	}
	for i, line := range lines {
		var record inboxResponseStageRecord
		if err := json.Unmarshal([]byte(line[strings.IndexByte(line, '{'):]), &record); err != nil {
			t.Fatal(err)
		}
		if record.RequestID != "request-fixture" || record.RoomID != "room-fixture" || record.AgentID != "agent-fixture" || record.Release != "release-fixture" || record.Stage != "format_repair" || record.BudgetMS > 45000 {
			t.Fatal("lost correlation", record)
		}
		if record.AccountID != "acct-fixture" || record.RequestedModel != "model-fixture" || record.PromptBytes != 2048 || record.ContextBytes != 0 || record.CandidateBytes != 100 {
			t.Fatal("lost route or context metrics", record)
		}
		if i == 0 && record.Outcome != "started" || i == 1 && (record.Outcome != "failed" || record.FailureKey != "inbox_response:agy:format_repair:failed") {
			t.Fatal("incorrect stage outcome", record)
		}
	}
}

func TestInboxRuntimeSummaryContainsOnlyCorrelatedCounters(t *testing.T) {
	var output bytes.Buffer
	previous := log.Writer()
	log.SetOutput(&output)
	defer log.SetOutput(previous)
	ctx := context.WithValue(context.Background(), inboxRequestTraceKey{}, inboxRequestTrace{RequestID: "request-fixture"})
	runtime := &inboxRuntimeStream{pending: []byte("PRIVATE_CONTENT"), toolCalls: 80, toolFailures: 3, toolInputEvents: 4, toolOutputEvents: 2, truncated: true, exitSignal: 10}
	traceInboxRuntimeSummary(context.Background(), "analysis", "release-fixture", runtime, "")
	if output.Len() != 0 {
		t.Fatal("uncorrelated summary")
	}
	traceInboxRuntimeSummary(ctx, "analysis", "release-fixture", runtime, "runtime_missing_result")
	line := output.String()
	var record map[string]any
	if err := json.Unmarshal([]byte(line[strings.IndexByte(line, '{'):]), &record); err != nil {
		t.Fatal(err)
	}
	if record["request_id"] != "request-fixture" || record["tool_calls"] != float64(80) || record["tool_failures"] != float64(3) || record["trace_limited"] != true || record["exit_signal"] != float64(10) || record["reason"] != "runtime_missing_result" || strings.Contains(line, "PRIVATE") {
		t.Fatal("missing counters or private content in summary", line)
	}
}
