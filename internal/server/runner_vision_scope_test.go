package server

import (
	"context"
	"github.com/buchin/cuan/internal/runner"
	"github.com/buchin/cuan/internal/task"
	"testing"
)

func TestInspectionVisionUsesOriginalScopeInsteadOfGenericTransport(t *testing.T) {
	s := &Server{}
	generic := "Inspect current behavior; inspect actual UI screenshots at original required viewports for UI work. Do not substitute old screenshots."
	for _, tc := range []struct {
		name, scope      string
		criteria         []task.DoDItem
		inspection, want bool
	}{
		{"database inspection", "Migrate PostgreSQL data to SQLite and reconcile row counts", nil, true, false},
		{"visual task without screenshot keyword", "Fix the task card UI layout", nil, true, true},
		{"original visual scope", "Verify actual mobile screenshots", nil, true, true},
		{"original visual criterion", "Improve task detail", []task.DoDItem{{Text: "Inspect screenshots at 390px and desktop"}}, true, true},
		{"ordinary explicit visual revision", "Improve task detail", nil, false, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			d := &task.TaskDetailResponse{Task: task.Task{Title: "Work", Instructions: tc.scope, DoDChecklist: tc.criteria}}
			claim := &runner.ClaimTaskResponse{}
			if tc.inspection {
				claim.LiveInspection = &task.LiveInspectionIntent{}
			}
			if got := s.runnerClaimRequiresVision(context.Background(), claim, d, generic); got != tc.want {
				t.Fatalf("vision=%v want %v", got, tc.want)
			}
		})
	}
}
