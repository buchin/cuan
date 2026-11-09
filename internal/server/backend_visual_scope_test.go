package server

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/buchin/cuan/internal/runner"
	"github.com/buchin/cuan/internal/task"
)

func TestOriginalBackendScopeDoesNotAuthorizeBrowserCapture(t *testing.T) {
	raw, err := os.ReadFile("../executionpolicy/testdata/backend_verifier_publication_scope.txt")
	if err != nil {
		t.Fatal(err)
	}
	s := &Server{}
	d := &task.TaskDetailResponse{Task: task.Task{Title: "Backend dependency optimization", Instructions: string(raw), DoDChecklist: []task.DoDItem{{Text: "1 task backend terfokus, tanpa modifikasi UI, browser screenshot, atau email."}}}}
	if s.taskRequiresVision(context.Background(), "owner", d, "", "") {
		t.Error("negative backend screenshot scope requested image capability")
	}
	for _, scope := range []string{string(raw), "Jangan browser/screenshot.", "Tanpa modifikasi UI, browser screenshot, atau email.", "No UI changes, browser screenshots, or email."} {
		claim := &runner.ClaimTaskResponse{RequiresVision: true, Task: &task.Task{Instructions: scope}, LiveInspection: &task.LiveInspectionIntent{TargetCommit: strings.Repeat("a", 40)}}
		if err := s.bindRunnerVisualRecipe(context.Background(), claim, "unavailable-source"); err != nil || claim.VisualRecipe != nil {
			t.Errorf("capture prohibition scheduled a recipe for %q: %v", scope, err)
		}
	}
	d.Instructions += "\nInspect the customer-supplied screenshot.png."
	if !s.taskRequiresVision(context.Background(), "owner", d, "", "") {
		t.Fatal("capture prohibition hid a genuine image-reading requirement")
	}
}

func TestDeliveredArtifactsDoNotBecomeOwnerImageInputs(t *testing.T) {
	for _, scenario := range []string{"delivered output", "owner input", "foreign output", "wrong author", "missing attempt"} {
		t.Run(scenario, func(t *testing.T) {
			s := &Server{}
			attempt := "attempt"
			d := &task.TaskDetailResponse{Task: task.Task{ID: "task", Instructions: "Backend tests only. No screenshots."}, Attachments: []any{map[string]any{"id": "picture", "name": "native.png", "type": "image/png"}}}
			if scenario != "owner input" {
				event := &task.TaskActivity{TaskID: "task", AttemptID: &attempt, ActivityType: "work_files", Author: "Cuan", Message: `{"attachments":[{"id":"picture","name":"native.png","type":"image/png"}]}`}
				if scenario == "foreign output" {
					event.TaskID = "other"
				}
				if scenario == "wrong author" {
					event.Author = "agent"
				}
				if scenario == "missing attempt" {
					event.AttemptID = nil
				}
				d.Activities = []*task.TaskActivity{event}
			}
			if got := s.taskRequiresVision(context.Background(), "owner", d, "", ""); got != (scenario != "delivered output") {
				t.Fatalf("vision=%v for %s", got, scenario)
			}
		})
	}
}

func TestOwnerCanSupplyAnEarlierDeliveredImageAsOriginalInput(t *testing.T) {
	s, d, _ := stalledReviewFixture(t, "completed")
	publishReviewFixtureImage(t, s, d)
	ctx := context.Background()
	d, err := s.taskSvc.GetTaskDetail(ctx, d.ID)
	if err != nil {
		t.Fatal(err)
	}
	images := s.inboxReviewImages(ctx, d)
	if len(images) != 1 {
		t.Fatal("missing delivered fixture")
	}
	d.Title, d.Instructions, d.DoDChecklist = "Backend task", "Backend only. No screenshots.", nil
	d.Attachments = []any{map[string]any{"id": images[0].ID, "name": "mobile.png", "type": "image/png"}}
	// Owner task reads project the server-delivered message as a work_files
	// activity alongside its attachment, unlike the raw task store above.
	d.Activities = append(d.Activities, &task.TaskActivity{ID: "files:fixture", TaskID: d.ID, AttemptID: &d.LatestAttempt.ID, ActivityType: "work_files", Author: "Cuan", Message: `{"attachments":[{"id":"` + images[0].ID + `"}]}`})
	if s.taskRequiresVision(ctx, "owner", d, "", "") {
		t.Fatal("delivered output became an owner input")
	}
	message, err := s.projectStudioRepo.GetMessageByID(ctx, d.SourceMessageID)
	if err != nil || message == nil {
		t.Fatal("missing original input", err)
	}
	message.Content = encodeAttachmentMessage("Inspect this existing image", []string{images[0].ID})
	if err := s.projectStudioRepo.UpdateMessage(ctx, message); err != nil {
		t.Fatal(err)
	}
	if !s.taskRequiresVision(ctx, "owner", d, "", "") {
		t.Fatal("delivered-image filtering hid an explicit original image input")
	}
}
