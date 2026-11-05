package task

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestCurrentOutcomeLaunchPreservesScopeAndSingleRequestIdentity(t *testing.T) {
	now := time.Now().UTC()
	task := &Task{ID: "task", ProjectID: "project", SourceRoomID: "room", Instructions: "Approved original requirements"}
	base := LiveInspectionIntent{Purpose: CurrentOutcomeInspection, SourceAttemptID: "failed-timeout", TargetCommit: strings.Repeat("a", 40), Repository: "buchin/cuan", OwnerID: "owner", RoomID: "room", SourceRequestID: "original", InstructionsHash: LaunchInstructionsHash(task.Instructions), ObservedAt: now}
	for _, name := range []string{"valid", "unknown purpose", "invented source", "inspection chain", "wrong scope", "foreign owner", "foreign room", "future observation", "changed target"} {
		t.Run(name, func(t *testing.T) {
			p := base
			launch := &TaskLaunch{TaskID: task.ID, ProjectID: task.ProjectID, OwnerID: p.OwnerID, RoomID: p.RoomID, SourceRequestID: p.SourceRequestID, InstructionsHash: p.InstructionsHash, CreatedAt: now, RequestID: p.RequestID(task.ID)}
			switch name {
			case "unknown purpose":
				p.Purpose = "other"
			case "invented source":
				p.SourceCommit = strings.Repeat("b", 40)
			case "inspection chain":
				p.PreviousAttemptID = "prior"
				p.PreviousCommit = strings.Repeat("b", 40)
			case "wrong scope":
				p.InstructionsHash = "different"
			case "foreign owner":
				p.OwnerID = "other"
			case "foreign room":
				p.RoomID = "other"
			case "future observation":
				p.ObservedAt = now.Add(time.Hour)
			case "changed target":
				p.TargetCommit = strings.Repeat("c", 40)
			}
			raw, _ := json.Marshal(p)
			launch.Revision = LiveInspectionMarker + string(raw)
			got, err := LiveInspectionForLaunch(task, launch)
			valid := name == "valid" || name == "changed target"
			if (err == nil && got != nil) != valid {
				t.Fatalf("unexpected binding acceptance: %v", err)
			}
			if valid && (got.SnapshotSourceCommit() != p.TargetCommit || got.RequestID(task.ID) != base.RequestID(task.ID)) {
				t.Fatal("release change reset budget or snapshot used unknown source")
			}
		})
	}
}

func TestCurrentOutcomeCheckFailureRequiresExactLatestFinalImplementationReport(t *testing.T) {
	attempt := "source"
	old := time.Now().Add(-time.Minute)
	report := &TaskActivity{ID: "report", TaskID: "task", AttemptID: &attempt, ActivityType: "agent_report", NewStatus: TaskStatusReview, CreatedAt: old}
	kind := &TaskActivity{ID: "kind", TaskID: "task", AttemptID: &attempt, ActivityType: "agent_result_kind", NewStatus: "implementation", CreatedAt: old}
	for _, scenario := range []string{"valid", "wrong attempt", "progress only", "newer progress", "other kind", "foreign task"} {
		t.Run(scenario, func(t *testing.T) {
			r, k := *report, *kind
			events := []*TaskActivity{&r, &k}
			switch scenario {
			case "wrong attempt":
				other := "other"
				r.AttemptID = &other
			case "progress only":
				r.NewStatus = "progress"
			case "newer progress":
				later := r
				later.ID = "newer"
				later.CreatedAt = time.Now()
				later.NewStatus = "progress"
				events = append(events, &later)
			case "other kind":
				k.NewStatus = "investigation"
			case "foreign task":
				r.TaskID = "foreign"
			}
			if liveImplementationFinalReport(events, "task", attempt) != (scenario == "valid") {
				t.Fatal("historical or unrelated report granted inspection eligibility")
			}
		})
	}
}
