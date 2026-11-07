package server

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"github.com/buchin/cuan/internal/task"
)

type countedReviewEvidenceStore struct {
	task.Store
	reader task.CurrentAttemptActivityStore
	calls  int
	fail   bool
}

func (s *countedReviewEvidenceStore) CurrentAttemptActivities(ctx context.Context, ids, kinds []string) (map[string][]*task.TaskActivity, error) {
	s.calls++
	if s.fail {
		return nil, errors.New("unavailable")
	}
	return s.reader.CurrentAttemptActivities(ctx, ids, kinds)
}
func (s *countedReviewEvidenceStore) ListActivities(context.Context, string) ([]*task.TaskActivity, error) {
	panic("list projection performed a per-task evidence read")
}

func TestTaskReviewFindingProjection(t *testing.T) { reviewFindingProjectionContract(t, nil) }
func TestTaskReviewFindingProjectionPostgres(t *testing.T) {
	withPhasePostgres(t, func(database *sql.DB) { reviewFindingProjectionContract(t, database) })
}
func reviewFindingProjectionContract(t *testing.T, database *sql.DB) {
	t.Helper()
	ctx := context.Background()
	var databases []*sql.DB
	if database != nil {
		databases = append(databases, database)
	}
	s, q, d := phaseLineageSetup(t, databases...)
	d.Status = task.TaskStatusBlocked
	if err := s.taskSvc.Store().UpdateTask(ctx, &d.Task); err != nil {
		t.Fatal(err)
	}
	finding := "Navigasi dari kartu percakapan ke obrolan yang tepat belum terverifikasi pada ponsel dan desktop."
	if _, err := s.saveReviewAssessment(ctx, d, q, inboxReviewEvaluation{Decision: "revise", RemainingWork: "verification", Finding: finding, Explanation: finding, Revision: "Verify real navigation"}); err != nil {
		t.Fatal(err)
	}
	if got := currentTaskReviewFinding(d); got != finding {
		t.Fatal("detail lost exact saved finding", got)
	}
	base := s.taskSvc.Store()
	counted := &countedReviewEvidenceStore{Store: base, reader: base.(task.CurrentAttemptActivityStore)}
	s.taskSvc = task.NewService(counted, s.projectSvc)
	for _, scenario := range []string{"current", "changed instructions", "changed evidence", "new attempt", "cancel requested", "cancelled", "running", "done", "unavailable"} {
		t.Run(scenario, func(t *testing.T) {
			item := &task.TaskItemResponse{Task: d.Task, LatestAttempt: d.LatestAttempt, ReviewFinding: "stale"}
			a := *d.LatestAttempt
			item.LatestAttempt = &a
			switch scenario {
			case "changed instructions":
				item.Instructions += " Changed scope"
			case "changed evidence":
				item.TestSummary = "New result"
			case "new attempt":
				a.ID = "newer"
			case "cancel requested":
				a.CancelRequested = true
			case "cancelled":
				a.Status = task.AttemptStatusCanceled
			case "running":
				a.Status = task.AttemptStatusRunning
			case "done":
				item.Status = task.TaskStatusDone
			}
			counted.fail = scenario == "unavailable"
			before := counted.calls
			other := &task.TaskItemResponse{Task: task.Task{ID: "unrelated", Status: task.TaskStatusBlocked}, LatestAttempt: &task.TaskAttempt{ID: "unrelated-attempt"}}
			err := s.projectTaskReviewFindings(ctx, []*task.TaskItemResponse{item, other})
			if counted.calls-before != 1 {
				t.Fatal("projection did not use one batch")
			}
			if (err != nil) != counted.fail {
				t.Fatal("storage failure was hidden", err)
			}
			want := ""
			if scenario == "current" {
				want = finding
			}
			if item.ReviewFinding != want || other.ReviewFinding != "" {
				t.Fatalf("stale or cross-task finding: %q / %q", item.ReviewFinding, other.ReviewFinding)
			}
		})
	}
	// A worker cannot impersonate the coordinator by supplying a review record.
	altered := *d
	altered.Activities = make([]*task.TaskActivity, len(d.Activities))
	for i, a := range d.Activities {
		cp := *a
		if cp.ActivityType == reviewAssessmentActivity {
			cp.Author = "worker"
		}
		altered.Activities[i] = &cp
	}
	if currentTaskReviewFinding(&altered) != "" {
		t.Fatal("worker claim displayed as coordinator finding")
	}
}
