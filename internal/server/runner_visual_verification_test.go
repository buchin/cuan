package server

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"image"
	"image/png"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/buchin/cuan/internal/db"
	"github.com/buchin/cuan/internal/runner"
	"github.com/buchin/cuan/internal/task"
	"github.com/go-chi/chi/v5"
)

func TestNativeVisualCaptureLifecycle(t *testing.T) {
	for _, scenario := range []string{"valid", "missing record", "changed receipt", "wrong runner", "wrong recipe", "wrong dimensions", "owner token", "task token", "escape", "corrupted delivery"} {
		t.Run(scenario, func(t *testing.T) {
			s, q, d := phaseLineageSetup(t)
			liveInspectionContract(t, s, q, d, "current outcome unpublished valid native "+scenario, nil)
		})
	}
}

// Tiny images are protocol fixtures only; real application proof is exercised
// separately through scripts/verify-ui.sh against the real React components.
func nativeVisualLifecycleRecord(t *testing.T, s *Server, d *task.TaskDetailResponse, claim *runner.ClaimTaskResponse, scenario string) *task.VisualVerification {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	s.cfg.RunnerToken = "native-test-runner-token"
	ctx := context.Background()
	id := claim.Attempt.ID
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "scripts"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, task.VisualVerificationScript), []byte("#!/bin/sh\nexit 0\n"), 0700); err != nil {
		t.Fatal(err)
	}
	claim.RequiresVision = true
	if err := s.bindRunnerVisualRecipe(ctx, claim, root); err != nil {
		t.Fatal(err)
	}
	if claim.VisualRecipe == nil {
		t.Fatal("recipe was not bound before capture")
	}
	recipe := *claim.VisualRecipe
	var data bytes.Buffer
	if err := png.Encode(&data, image.NewRGBA(image.Rect(0, 0, 3, 4))); err != nil {
		t.Fatal(err)
	}
	hash := sha256.Sum256(data.Bytes())
	receipt := &task.VisualVerification{Recipe: recipe, Baseline: strings.Repeat("c", 64), Components: []string{"web/Component.tsx"}, Captures: []task.VisualCapture{{Path: "native-visual/capture.png", SHA256: hex.EncodeToString(hash[:]), Width: 3, Height: 4}}}
	request := runner.VisualVerificationRequest{RunnerID: "live-runner", Verification: receipt, Files: [][]byte{data.Bytes()}}
	token := s.cfg.RunnerToken
	expected := 200
	switch scenario {
	case "missing record":
		return receipt
	case "wrong runner":
		request.RunnerID = "foreign"
		expected = 409
	case "wrong recipe":
		receipt.Recipe.ScriptHash = strings.Repeat("d", 64)
		expected = 409
	case "wrong dimensions":
		receipt.Captures[0].Width = 7
		expected = 400
	case "owner token":
		token = s.cfg.AuthSecret
		expected = 401
	case "task token":
		token = "task-session-token"
		expected = 401
	case "escape":
		receipt.Captures[0].Path = "native-visual/../private.png"
		expected = 400
	}
	route := chi.NewRouter()
	route.Post("/attempts/{id}/visual", s.handleRunnerVisualVerification)
	for i := 0; i < 2; i++ {
		body, _ := json.Marshal(request)
		r := httptest.NewRequest("POST", "/attempts/"+id+"/visual", bytes.NewReader(body))
		r.Header.Set("X-Runner-Token", token)
		w := httptest.NewRecorder()
		route.ServeHTTP(w, r)
		if w.Code != expected {
			t.Fatalf("native capture status=%d expected=%d body=%s", w.Code, expected, w.Body.String())
		}
	}
	current, _ := s.taskSvc.GetTaskDetail(ctx, d.ID)
	images := s.inboxReviewImages(ctx, current)
	if expected == 200 {
		message, err := s.projectStudioRepo.GetMessageByID(ctx, delegationMessageID("task-visual-delivery:"+id))
		if err != nil || message.AuthorID != db.AgentIDCuan {
			t.Fatal("native output did not keep private canonical agent identity", err)
		}
		reportRoute := chi.NewRouter()
		reportRoute.Get("/attempts/{id}/report", s.handleRunnerReadAttemptReport)
		snapshot := httptest.NewRecorder()
		reportRoute.ServeHTTP(snapshot, httptest.NewRequest("GET", "/attempts/"+id+"/report?task_id="+d.ID+"&runner_id=live-runner", nil))
		if snapshot.Code != 200 || !strings.Contains(snapshot.Body.String(), "native_visual_verification") || strings.Contains(snapshot.Body.String(), "Cuan managed delegation") {
			t.Fatal("narrow native receipt reconciliation omitted identity or exposed instructions")
		}
		if len(images) != 1 || !images[0].NativeCapture {
			t.Fatal("native capture lost transport provenance", images)
		}
		events := 0
		for _, event := range current.Activities {
			if event.ActivityType == task.ActivityVisualVerification {
				events++
			}
		}
		if events != 1 {
			t.Fatal("duplicate native evidence receipt", events)
		}
		if scenario == "changed receipt" {
			receipt.Baseline = strings.Repeat("d", 64)
		}
		if scenario == "corrupted delivery" {
			if err := os.WriteFile(images[0].Path, []byte("replaced"), 0600); err != nil {
				t.Fatal(err)
			}
			verdict := s.requireVisualReview(ctx, current, inboxReviewEvaluation{Decision: "pass", VisualInspection: "Claimed visual pass"})
			if verdict.Decision == "pass" {
				t.Fatal("tampered native image passed visual gate")
			}
		}
		if scenario == "valid" {
			directory := t.TempDir()
			if err := os.WriteFile(filepath.Join(directory, "supplement.png"), data.Bytes(), 0600); err != nil {
				t.Fatal(err)
			}
			root, err := os.OpenRoot(directory)
			if err != nil {
				t.Fatal(err)
			}
			defer root.Close()
			files, err := readTaskArtifacts(root, d.ID, id, []string{"supplement.png"})
			if err != nil {
				t.Fatal(err)
			}
			if err = s.publishTaskArtifacts(ctx, current, "owner", files); err != nil {
				t.Fatal(err)
			}
			images = s.inboxReviewImages(ctx, current)
			if len(images) != 2 || !images[0].NativeCapture || images[1].NativeCapture {
				t.Fatal("supplement inherited native capture provenance", images)
			}
		}
	} else if len(images) != 0 {
		t.Fatal("invalid native evidence was delivered")
	}
	return receipt
}

func TestNativeVisualRecipeDoesNotOverrideCaptureRestriction(t *testing.T) {
	for _, restriction := range []string{"Do not capture new screenshots.", "No screenshots.", "Jangan ambil tangkapan layar.", "Tanpa screenshot.", "Do not use browser automation.", "No browser.", "Without browser automation.", "Jangan menggunakan otomasi browser.", "Jangan pakai browser.", "Tanpa browser.", "Inspect the supplied UI image. Do not launch Chromium."} {
		t.Run(restriction, func(t *testing.T) {
			s := &Server{}
			claim := &runner.ClaimTaskResponse{RequiresVision: true, Task: &task.Task{Instructions: restriction}, LiveInspection: &task.LiveInspectionIntent{TargetCommit: strings.Repeat("a", 40)}}
			if err := s.bindRunnerVisualRecipe(context.Background(), claim, "unavailable-source"); err != nil || claim.VisualRecipe != nil {
				t.Fatal("image capability expanded capture authority", err)
			}
		})
	}
}
