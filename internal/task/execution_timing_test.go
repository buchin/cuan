package task

import (
	"encoding/json"
	"testing"
	"time"
)

func TestAttemptExecutionTimingRequiresCurrentRunnerReceipt(t *testing.T) {
	start := time.Now().UTC().Add(-10 * time.Second)
	finish := start.Add(9 * time.Second)
	agent, checks := int64(2000), int64(4000)
	for _, mode := range []string{"valid", "zero", "foreign author", "historical", "foreign task", "wrong id", "wrong type", "negative", "impossible sum", "missing start", "missing finish", "unknown"} {
		t.Run(mode, func(t *testing.T) {
			a := &TaskAttempt{ID: "current", TaskID: "work", RunnerID: "native", StartedAt: &start, FinishedAt: &finish}
			timing := &ExecutionTiming{AgentElapsedMS: &agent, ProjectChecksElapsedMS: &checks}
			event := &TaskActivity{ID: "verification:" + a.ID, TaskID: a.TaskID, AttemptID: &a.ID, Author: a.RunnerID, ActivityType: ActivityTypeRunnerVerification}
			switch mode {
			case "foreign author":
				event.Author = "worker"
			case "historical":
				id := "previous"
				event.AttemptID = &id
			case "foreign task":
				event.TaskID = "other"
			case "wrong id":
				event.ID = "other"
			case "wrong type":
				event.ActivityType = "agent_report"
			case "negative":
				n := int64(-1)
				timing.AgentElapsedMS = &n
			case "impossible sum":
				n := int64(9000)
				timing.AgentElapsedMS = &n
			case "missing start":
				a.StartedAt = nil
			case "missing finish":
				a.FinishedAt = nil
			case "unknown":
				timing = &ExecutionTiming{}
			case "zero":
				n := int64(0)
				timing.AgentElapsedMS = &n
			}
			raw, _ := json.Marshal(RunnerVerificationRecord{Summary: "Retained native result", Timing: timing})
			event.Message = string(raw)
			d := &TaskDetailResponse{Task: Task{ID: a.TaskID}, LatestAttempt: a, Activities: []*TaskActivity{event}}
			got := AttemptExecutionTiming(d)
			if mode == "valid" || mode == "zero" {
				if got == nil || *got.AgentElapsedMS != *timing.AgentElapsedMS || *got.ProjectChecksElapsedMS != checks {
					t.Fatal("lost measured stages", got)
				}
			} else if got != nil {
				t.Fatal("untrusted metrics accepted", got)
			}
		})
	}
}
