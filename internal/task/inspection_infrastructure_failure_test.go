package task

import (
	"testing"
	"time"
)

func TestInspectionInfrastructureFailureRequiresSavedFailureWithoutResult(t *testing.T) {
	for _, mode := range []string{"launch", "setup", "stale release", "missing transition", "foreign author", "foreign task", "cancelled", "missing exit", "wrong exit", "running", "interrupted", "unfinished", "final report", "blocked report", "check failure", "repository observation", "unrecognized error", "empty setup diagnostic", "future transition", "setup checks ran", "setup missing checks"} {
		t.Run(mode, func(t *testing.T) {
			started := time.Now().UTC().Add(-time.Second)
			finished := started.Add(time.Second)
			exit := 1
			a := &TaskAttempt{ID: "attempt", TaskID: "task", RunnerID: "worker", Status: AttemptStatusFailed, ExitCode: &exit, StartedAt: &started, FinishedAt: &finished, ErrorMessage: "Could not save task status before starting."}
			transition := &TaskActivity{TaskID: a.TaskID, AttemptID: &a.ID, ActivityType: ActivityTypeAttempt, Author: "runner", OldStatus: AttemptStatusRunning, NewStatus: AttemptStatusFailed, Message: "[Message: " + a.ErrorMessage + "]", CreatedAt: finished}
			events := []*TaskActivity{transition}
			if mode == "stale release" {
				a.ErrorMessage = StaleInspectionReleaseMessage
				transition.Author, transition.OldStatus, transition.NewStatus = a.RunnerID, TaskStatusDoing, TaskStatusBlocked
				transition.Message = "Attempt attempt selesai dengan status 'failed' (exit code: 1, peak RSS: 0 bytes)."
				events = append(events, &TaskActivity{ID: "verification:" + a.ID, TaskID: a.TaskID, AttemptID: &a.ID, ActivityType: ActivityTypeRunnerVerification, Author: a.RunnerID, Message: `{"summary":""}`})
			}
			if mode == "setup" || mode == "setup checks ran" || mode == "setup missing checks" || mode == "empty setup diagnostic" {
				a.ErrorMessage = InspectionDependencyFailurePrefix + ": npm ERR! ENOSPC"
				transition.Author, transition.OldStatus, transition.NewStatus = a.RunnerID, TaskStatusDoing, TaskStatusBlocked
				transition.Message = "Attempt attempt selesai dengan status 'failed' (exit code: 1, peak RSS: 0 bytes)."
				if mode != "setup missing checks" {
					events = append(events, &TaskActivity{ID: "verification:" + a.ID, TaskID: a.TaskID, AttemptID: &a.ID, ActivityType: ActivityTypeRunnerVerification, Author: a.RunnerID, Message: `{"summary":""}`})
				}
			}
			switch mode {
			case "missing transition":
				events = nil
			case "foreign author":
				transition.Author = "owner"
			case "foreign task":
				transition.TaskID = "other"
			case "cancelled":
				a.CancelRequested = true
			case "missing exit":
				a.ExitCode = nil
			case "wrong exit":
				exit = 124
			case "running":
				a.Status = AttemptStatusRunning
			case "interrupted":
				a.Status = AttemptStatusInterrupted
			case "unfinished":
				a.FinishedAt = nil
			case "unrecognized error":
				a.ErrorMessage = "Agent reported execution failure"
			case "empty setup diagnostic":
				a.ErrorMessage = InspectionDependencyFailurePrefix
			case "future transition":
				transition.CreatedAt = finished.Add(time.Second)
			case "setup checks ran":
				events[1].Message = `{"summary":"Project verification: assertion failed"}`
			case "final report", "blocked report", "check failure", "repository observation":
				e := &TaskActivity{TaskID: a.TaskID, AttemptID: &a.ID, Author: a.RunnerID}
				switch mode {
				case "final report":
					e.ActivityType, e.NewStatus = "agent_report", TaskStatusReview
				case "blocked report":
					e.ActivityType, e.NewStatus = "agent_report", TaskStatusBlocked
				case "check failure":
					e.ActivityType = ActivityTypeRunnerVerification
				case "repository observation":
					e.ActivityType = "repository_scope_observed"
				}
				events = append(events, e)
			}
			if got, want := InspectionInfrastructureFailure(a, events), mode == "launch" || mode == "setup" || mode == "stale release"; got != want {
				t.Fatalf("failure classified=%v want=%v", got, want)
			}
		})
	}
}
