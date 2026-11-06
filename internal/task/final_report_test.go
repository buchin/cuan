package task

import "testing"

func TestFinalAgentReportStateRequiresCurrentDurableEvidence(t *testing.T) {
	for _, scenario := range []string{"review", "done", "blocked", "progress", "missing-summary", "missing-evidence", "historical", "new-progress", "nil-activity", "missing-attempt", "no-report"} {
		t.Run(scenario, func(t *testing.T) {
			attempt := "current"
			state := scenario
			switch scenario {
			case "review", "done", "blocked", "progress":
			default:
				state = "review"
			}
			a := &TaskActivity{AttemptID: &attempt, ActivityType: "agent_report", NewStatus: state}
			d := &TaskDetailResponse{Task: Task{Summary: "Actual result", CompleteNote: "Observed evidence"}, Activities: []*TaskActivity{a}}
			want := state
			switch scenario {
			case "progress":
				want = ""
			case "missing-summary":
				d.Summary = " \n"
				want = ""
			case "missing-evidence":
				d.CompleteNote = " \n"
				want = ""
			case "historical":
				old := "previous"
				a.AttemptID = &old
				want = ""
			case "new-progress":
				d.Activities = append(d.Activities, &TaskActivity{AttemptID: &attempt, ActivityType: "agent_report", NewStatus: "progress"})
				want = ""
			case "nil-activity":
				d.Activities = append(d.Activities, nil)
			case "missing-attempt":
				a.AttemptID = nil
				want = ""
			case "no-report":
				a.ActivityType = "status_change"
				want = ""
			}
			if got := d.FinalAgentReportState(attempt); got != want {
				t.Fatalf("state=%q want=%q", got, want)
			}
			if d.FinalAgentReportState("") != "" {
				t.Fatal("unbound report accepted")
			}
		})
	}
	var empty *TaskDetailResponse
	if empty.FinalAgentReportState("attempt") != "" {
		t.Fatal("nil evidence accepted")
	}
}
