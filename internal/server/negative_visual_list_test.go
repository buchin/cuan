package server

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/buchin/cuan/internal/runner"
	"github.com/buchin/cuan/internal/task"
)

func TestNegativeVisualListDoesNotPrepareBackendTools(t *testing.T) {
	for _, scope := range []string{"Batasan: tidak ada UI, browser/screenshot, email, scheduler baru.", "No UI, browser, screenshots or email.", "Tanpa UI/UX, browser screenshot, atau email."} {
		s, d, _ := stalledReviewFixture(t, "completed")
		d.Title, d.Instructions = "Backend snapshot retention", scope
		if s.taskRequiresVision(context.Background(), "owner", d, "", "") {
			t.Errorf("negative scope requested vision: %q", scope)
		}
		claim := &runner.ClaimTaskResponse{Task: &d.Task, Attempt: d.LatestAttempt, LiveInspection: &task.LiveInspectionIntent{TargetCommit: strings.Repeat("a", 40)}}
		if s.runnerClaimRequiresVision(context.Background(), claim, d, "") {
			t.Errorf("inspection invented visual scope: %q", scope)
		}
		root := t.TempDir()
		if err := os.Mkdir(filepath.Join(root, "scripts"), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, "scripts/verify-ui.sh"), []byte("#!/bin/sh\nexit 23\n"), 0700); err != nil {
			t.Fatal(err)
		}
		claim.RequiresVision = true
		// A real recipe and store exist; the original prohibition must prevent
		// binding it. The script is never executed by this test.
		if err := s.bindRunnerVisualRecipe(context.Background(), claim, root); err != nil || claim.VisualRecipe != nil {
			t.Fatal("capture bound despite original prohibition", err)
		}
		d.Instructions += "\nInspect the supplied customer.png."
		if !s.taskRequiresVision(context.Background(), "owner", d, "", "") {
			t.Fatal("real image reading hidden")
		}
	}
}
