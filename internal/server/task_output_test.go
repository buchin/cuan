package server

import (
	"context"
	"strings"
	"testing"

	"github.com/buchin/cuan/internal/task"
)

func TestTaskOutputRetainsLateRedactedDiagnostics(t *testing.T) {
	svc := task.NewService(task.NewMemoryStore(), nil)
	recorder := newTaskOutputRecorder(svc, "run")
	recorder.secrets = append(recorder.secrets, "fixture-private-value")
	for i := 0; i < 1200; i++ {
		recorder.append([]byte("ordinary progress\n"))
	}
	recorder.append([]byte("Individual quota reached; fixture-private-"))
	recorder.append([]byte("value\n"))
	recorder.flush()
	logs, err := svc.GetAttemptLogs(context.Background(), "run", 0, 1000)
	if err != nil || len(logs) != 1000 {
		t.Fatal("retained logs", len(logs), err)
	}
	last := logs[len(logs)-1].Line
	if !strings.Contains(last, "Individual quota reached") || strings.Contains(last, "fixture-private") {
		t.Fatal("late diagnosis or redaction failed", last)
	}
}
