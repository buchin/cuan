package task

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestLaunchPresentationUsesBoundPhaseWithoutReturningRevision(t *testing.T) {
	for _, item := range []struct{ revision, want string }{{"Ordinary task", ""}, {NativeReleaseIntentMarker + `{"private":"PRIVATE_PROFILE"}`, "native_release"}, {VerificationIntentMarker + `{}`, "verification"}, {LiveInspectionMarker + `{}`, "inspection"}, {NativeReleaseIntentMarker + VerificationIntentMarker, ""}, {LiveInspectionMarker + LiveInspectionMarker, ""}} {
		if got := launchPresentationKind(item.revision); got != item.want {
			t.Fatalf("phase %q want %q", got, item.want)
		}
		store := NewMemoryStore()
		now := time.Now().UTC()
		if err := store.CreateTask(context.Background(), &Task{ID: "work", ProjectID: "project", Status: TaskStatusReview, CreatedAt: now, UpdatedAt: now}); err != nil {
			t.Fatal(err)
		}
		_, err := store.SaveLaunch(context.Background(), TaskLaunch{TaskID: "work", ProjectID: "project", RequestID: "request", OwnerID: "owner", RoomID: "room", SourceRequestID: "source", AgentID: "agent", Revision: item.revision, InstructionsHash: "scope", CreatedAt: now, AvailableAt: now, ExpiresAt: now.Add(time.Hour)})
		if err != nil {
			t.Fatal(err)
		}
		infos, err := store.LatestLaunchInfos(context.Background(), []string{"work"})
		if err != nil {
			t.Fatal(err)
		}
		if infos["work"].Kind != item.want {
			t.Fatal("task list lost native phase", infos)
		}
		raw, _ := json.Marshal(infos)
		if strings.Contains(string(raw), "PRIVATE_PROFILE") || strings.Contains(string(raw), "revision") {
			t.Fatal("phase projection exposed execution payload")
		}
	}
}
