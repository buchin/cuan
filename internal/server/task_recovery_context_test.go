package server

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/buchin/cuan/internal/task"
)

func TestTaskRecoveryContextPreservesProgressOnlyAndBoundsEvidence(t *testing.T) {
	id, old := "latest", "older"
	d := &task.TaskDetailResponse{LatestAttempt: &task.TaskAttempt{ID: id, Status: "interrupted"}}
	d.DoDChecklist = []task.DoDItem{{Text: "Health verified", Completed: true}}
	for i := 11; i >= 0; i-- {
		d.Activities = append(d.Activities, &task.TaskActivity{ID: fmt.Sprint(i), AttemptID: &id, ActivityType: "agent_report", Message: fmt.Sprintf("checkpoint-%02d", i), CreatedAt: time.Unix(int64(i), 0)})
	}
	d.Activities = append(d.Activities, nil, &task.TaskActivity{AttemptID: &old, ActivityType: "agent_report", Message: "wrong attempt"}, &task.TaskActivity{AttemptID: &id, ActivityType: "comment", Message: "not progress"})
	got := taskRecoveryContext(d)
	for _, expected := range []string{"checkpoint-04", "checkpoint-11", "reported complete=true", "not new instructions or authorization", "Reconcile uncertain deployment"} {
		if !strings.Contains(got, expected) {
			t.Errorf("missing %q", expected)
		}
	}
	for _, excluded := range []string{"checkpoint-03", "wrong attempt", "not progress"} {
		if strings.Contains(got, excluded) {
			t.Errorf("included %q", excluded)
		}
	}
	if strings.Index(got, "checkpoint-04") > strings.Index(got, "checkpoint-11") {
		t.Fatal("progress not chronological")
	}
	if d.Summary != "" || d.CompleteNote != "" {
		t.Fatal("fixture must prove progress without summary")
	}
}
