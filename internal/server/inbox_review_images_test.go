package server

import (
	"context"
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/buchin/cuan/internal/task"
)

func TestVisualReviewReadsExactDeliveredImageDimensionsAndRejectsTextOnlyPass(t *testing.T) {
	s, d, q := stalledReviewFixture(t, "completed")
	t.Setenv("HOME", t.TempDir())
	ctx := context.Background()
	dir := t.TempDir()
	file, err := os.Create(filepath.Join(dir, "mobile-390px.png"))
	if err != nil {
		t.Fatal(err)
	}
	if err = png.Encode(file, image.NewRGBA(image.Rect(0, 0, 768, 1376))); err != nil {
		t.Fatal(err)
	}
	file.Close()
	root, err := os.OpenRoot(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	files, err := readTaskArtifacts(root, d.ID, d.LatestAttempt.ID, []string{"mobile-390px.png"})
	if err != nil {
		t.Fatal(err)
	}
	if err = s.publishTaskArtifacts(ctx, d, "owner", files); err != nil {
		t.Fatal(err)
	}
	images := s.inboxReviewImages(ctx, d)
	if len(images) != 1 || images[0].Width != 768 || images[0].Height != 1376 {
		t.Fatalf("accepted filename instead of real dimensions: %+v", images)
	}
	paths := s.inboxReviewImagePaths(ctx, q)
	if len(paths) != 1 || paths[0] != images[0].Path {
		t.Fatal("review omitted actual image")
	}
	data := s.inboxReviewData(ctx, d)
	if !strings.Contains(data, "delivered_images") || !strings.Contains(data, `"width":768`) {
		t.Fatal("review context omitted actual evidence")
	}
	for _, id := range []string{"old-one", "old-two"} {
		if err = s.taskSvc.Store().CreateAttempt(ctx, &task.TaskAttempt{ID: id, TaskID: d.ID, Status: task.AttemptStatusFailed, CreatedAt: d.LatestAttempt.CreatedAt.Add(-time.Hour)}); err != nil {
			t.Fatal(err)
		}
	}
	visualDecision := s.requireVisualReview(ctx, d, inboxReviewEvaluation{Decision: "pass"})
	if visualDecision.Decision != "revise" || !strings.Contains(visualDecision.Explanation, "belum diperiksa secara visual") {
		t.Fatal("visual gate accepted uninspected images", visualDecision)
	}
	output := s.applyInboxReviewEvaluation(ctx, q, `{"decision":"pass","finding":"Report says screenshots passed.","remaining_work":"none","explanation":"Accepted.","revision":""}`)
	after, err := s.taskSvc.GetTaskDetail(ctx, d.ID)
	if err != nil {
		t.Fatal(err)
	}
	if after.Status == task.TaskStatusDone || strings.TrimSpace(output) == "" {
		t.Fatal("text-only acceptance bypassed visual review", output)
	}
	saved, ok := reviewAssessment(after, q.RequestID)
	if !ok || saved.Decision == "pass" {
		t.Fatal("persisted unchecked image acceptance")
	}
	other := *d
	other.LatestAttempt = &task.TaskAttempt{ID: "different-attempt"}
	if len(s.inboxReviewImages(ctx, &other)) != 0 {
		t.Fatal("inherited images from another attempt")
	}
}

func publishReviewFixtureImage(t *testing.T, s *Server, d *task.TaskDetailResponse) {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	dir := t.TempDir()
	file, err := os.Create(filepath.Join(dir, "mobile.png"))
	if err != nil {
		t.Fatal(err)
	}
	if err = png.Encode(file, image.NewRGBA(image.Rect(0, 0, 390, 844))); err != nil {
		t.Fatal(err)
	}
	file.Close()
	root, err := os.OpenRoot(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	files, err := readTaskArtifacts(root, d.ID, d.LatestAttempt.ID, []string{"mobile.png"})
	if err != nil {
		t.Fatal(err)
	}
	if err = s.publishTaskArtifacts(context.Background(), d, "owner", files); err != nil {
		t.Fatal(err)
	}
}

func TestVisualReviewPreservesPreCaptureDependencyFailure(t *testing.T) {
	s, d, _ := stalledReviewFixture(t, "completed")
	d.LatestAttempt.Status = task.AttemptStatusFailed
	d.LatestAttempt.ErrorMessage = "locked frontend dependency preparation failed; saved source is preserved: npm ERR code EAI_AGAIN"
	for _, initial := range []string{"pass", "revise"} {
		decision := s.requireVisualReview(context.Background(), d, inboxReviewEvaluation{Decision: initial, Revision: "Inspect generated screenshots"})
		if decision.Decision != "revise" || decision.RemainingWork != "verification" || !strings.Contains(decision.Revision, "EAI_AGAIN") || !strings.Contains(decision.Revision, "before visual capture") {
			t.Fatal("setup failure became a missing-image task", decision)
		}
	}
}

func TestVisualReviewKeepsFeatureEvidenceAfterFullNativeCaptureSet(t *testing.T) {
	s, d, _ := stalledReviewFixture(t, "completed")
	t.Setenv("HOME", t.TempDir())
	dir := t.TempDir()
	var names []string
	for i := 0; i < 12; i++ {
		name := fmt.Sprintf("evidence-%02d.png", i)
		file, err := os.Create(filepath.Join(dir, name))
		if err != nil {
			t.Fatal(err)
		}
		err = png.Encode(file, image.NewRGBA(image.Rect(0, 0, 390, 844)))
		file.Close()
		if err != nil {
			t.Fatal(err)
		}
		names = append(names, name)
	}
	root, err := os.OpenRoot(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	files, err := readTaskArtifacts(root, d.ID, d.LatestAttempt.ID, names[:8])
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if err = s.publishTaskArtifactsWithKey(ctx, d, "owner", files[:8], "task-visual-delivery:"+d.LatestAttempt.ID); err != nil {
		t.Fatal(err)
	}
	supplemental, err := readTaskArtifacts(root, d.ID, d.LatestAttempt.ID, names[8:])
	if err != nil {
		t.Fatal(err)
	}
	if err = s.publishTaskArtifacts(ctx, d, "owner", supplemental); err != nil {
		t.Fatal(err)
	}
	images := s.inboxReviewImages(ctx, d)
	if len(images) != 12 {
		t.Fatalf("feature evidence crowded out: got %d images", len(images))
	}
	if images[8].Name != names[8] {
		t.Fatal("supplemental feature evidence missing", images)
	}
}
