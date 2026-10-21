package server

import (
	"context"
	"strings"
	"testing"
	"time"

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

func TestTaskOutputRecognizesSplitNativeQuotaWarningOnce(t *testing.T) {
	svc := task.NewService(task.NewMemoryStore(), nil)
	r := newTaskOutputRecorder(svc, "quota")
	seen := make(chan struct{}, 2)
	r.onQuota = func() { seen <- struct{}{} }
	r.append([]byte("Discussion of quota is not a provider warning.\n"))
	select {
	case <-seen:
		t.Fatal("ordinary text stopped work")
	default:
	}
	r.append([]byte("\x1b[33m⚠ Individual quota rea"))
	r.append([]byte("ched. Please upgrade your subscription to increase your\r\nlimits. Resets in 63h.\x1b[0m\n"))
	select {
	case <-seen:
	case <-time.After(time.Second):
		t.Fatal("native quota not detected")
	}
	r.append([]byte("⚠ Individual quota reached. Please upgrade your subscription\n"))
	if !strings.Contains(r.failureNote(), "usage limit") {
		t.Fatal("quota outcome missing")
	}
	select {
	case <-seen:
		t.Fatal("duplicate stop callback")
	case <-time.After(20 * time.Millisecond):
	}
}

func TestTaskOutputRecognizesNativeLocationRestriction(t *testing.T) {
	svc := task.NewService(task.NewMemoryStore(), nil)
	r := newTaskOutputRecorder(svc, "region")
	seen := make(chan struct{}, 1)
	r.onRegion = func() { seen <- struct{}{} }
	r.append([]byte("⚠ FAILED_PRECONDITION (code 400): User location is not supported for the API\r\n"))
	r.append([]byte("use.\n"))
	select {
	case <-seen:
	case <-time.After(time.Second):
		t.Fatal("native location warning not detected")
	}
	if !strings.Contains(r.failureNote(), "location restriction") {
		t.Fatal("wrong recovery reason")
	}
	if r.quotaDetected {
		t.Fatal("regional rejection labelled as quota")
	}
}
