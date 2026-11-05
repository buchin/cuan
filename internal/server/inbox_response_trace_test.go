package server

import (
	"bytes"
	"context"
	"encoding/json"
	"log"
	"strings"
	"testing"
	"time"
)

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
	stage, cancel := inboxFormatRepairContext(ctx)
	defer cancel()
	finish := traceInboxResponseStage(ctx, stage, "format_repair", "agy", "release-fixture")
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
		if i == 0 && record.Outcome != "started" || i == 1 && (record.Outcome != "failed" || record.FailureKey != "inbox_response:agy:format_repair:failed") {
			t.Fatal("incorrect stage outcome", record)
		}
	}
}
