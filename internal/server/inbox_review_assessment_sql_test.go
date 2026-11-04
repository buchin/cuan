package server

import (
	"context"
	"database/sql"
	"testing"
)

func TestReviewAssessmentPostgres(t *testing.T) {
	withPhasePostgres(t, func(database *sql.DB) {
		s, q, d := phaseLineageSetup(t, database)
		ctx := context.Background()
		first := inboxReviewEvaluation{Decision: "revise", Finding: "Visual inspection evidence is missing.", Explanation: "Not accepted.", Revision: "Inspect the changed application at the required viewports."}
		if _, err := s.saveReviewAssessment(ctx, d, q, first); err != nil {
			t.Fatal(err)
		}
		current, err := s.taskSvc.GetTaskDetail(ctx, d.ID)
		if err != nil {
			t.Fatal(err)
		}
		second := inboxReviewEvaluation{Decision: "pass", Explanation: "Different answer", Finding: "Everything passed."}
		replay, err := s.saveReviewAssessment(ctx, current, q, second)
		if err != nil || replay != first {
			t.Fatal("SQL retry replaced durable assessment", replay, err)
		}
		var count int
		if err := database.QueryRowContext(ctx, `SELECT COUNT(*) FROM task_activities WHERE task_id=$1 AND activity_type=$2`, d.ID, reviewAssessmentActivity).Scan(&count); err != nil || count != 1 {
			t.Fatal("SQL receipt duplicated", count, err)
		}
		current.Instructions += "Changed scope"
		if _, ok := reviewAssessment(current, q.RequestID); ok {
			t.Fatal("SQL receipt authorized changed scope")
		}
	})
}
