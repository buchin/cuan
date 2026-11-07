package server

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/buchin/cuan/internal/task"
)

func TestHumanFeedbackLimitCannotBeBypassedWithExecutionMarker(t *testing.T) {
	s, _, call, _ := inboxFixture(t)
	item := workspaceTask(t, call)
	for _, prefix := range []string{"", task.LiveInspectionMarker, task.VerificationIntentMarker, task.NativeReleaseIntentMarker} {
		raw, _ := json.Marshal(map[string]string{"agent_id": "agent_lifecycle", "request_id": "unbound-request", "revision": prefix + strings.Repeat("x", task.MaxTaskFeedbackRunes+1)})
		response := call("POST", "/api/v1/tasks/"+item.ID+"/run-agent", string(raw))
		if response.Code != 400 {
			t.Fatal("unbound oversized feedback accepted", prefix, response.Code, response.Body.String())
		}
	}
	detail, err := s.taskSvc.GetTaskDetail(t.Context(), item.ID)
	if err != nil || detail.LatestAttempt != nil {
		t.Fatal("size exception created an unbound execution", err)
	}
}
