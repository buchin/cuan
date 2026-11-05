package server

import (
	"bytes"
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"github.com/buchin/cuan/internal/project"
	"image/png"
	"net/http"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/buchin/cuan/internal/runner"
	"github.com/buchin/cuan/internal/task"
	"github.com/go-chi/chi/v5"
)

func (s *Server) handleRunnerVisualVerification(w http.ResponseWriter, r *http.Request) {
	if s.cfg.RunnerToken == "" || subtle.ConstantTimeCompare([]byte(r.Header.Get("X-Runner-Token")), []byte(s.cfg.RunnerToken)) != 1 {
		respondJSONError(w, 401, "Native capture requires runner identity")
		return
	}
	var req runner.VisualVerificationRequest
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 35<<20))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&req) != nil || !req.Verification.Valid() || len(req.Files) != len(req.Verification.Captures) {
		respondJSONError(w, 400, "Invalid native visual evidence")
		return
	}
	id := chi.URLParam(r, "id")
	attempt, err := s.taskSvc.GetAttempt(r.Context(), id)
	if err != nil || attempt == nil || attempt.RunnerID != req.RunnerID || attempt.Status != task.AttemptStatusRunning || attempt.CancelRequested || attempt.CanceledAt != nil {
		respondJSONError(w, 409, "Visual capture attempt is not active")
		return
	}
	d, err := s.taskSvc.GetTaskDetail(r.Context(), attempt.TaskID)
	if err != nil || d.LatestAttempt == nil || d.LatestAttempt.ID != id || d.Status == "cancelled" || d.Status == "canceled" {
		respondJSONError(w, 409, "Visual capture task has changed")
		return
	}
	proof, err := task.ValidateLiveInspection(r.Context(), s.taskSvc.Store(), &d.Task, id)
	recipe, recipeErr := task.BoundVisualRecipe(d.Activities, d.ID, id)
	if err != nil || recipeErr != nil || proof == nil || recipe == nil || req.Verification.Recipe != *recipe || recipe.SourceCommit != proof.TargetCommit {
		respondJSONError(w, 409, "Visual evidence is not bound to this inspection recipe")
		return
	}
	raw, _ := json.Marshal(req.Verification)
	if previous := task.VisualVerificationForAttempt(d.Activities, d.ID, id, req.RunnerID); previous != nil {
		saved, _ := json.Marshal(previous)
		if !bytes.Equal(saved, raw) {
			respondJSONError(w, 409, "Different visual evidence already exists for this attempt")
			return
		}
	}
	owner, err := s.taskArtifactOwner(r.Context(), d)
	if err != nil {
		respondJSONError(w, 409, "Visual evidence ownership unavailable")
		return
	}
	files := make([]preparedTaskArtifact, 0, len(req.Files))
	total := 0
	for i, data := range req.Files {
		capture := req.Verification.Captures[i]
		total += len(data)
		hash := sha256.Sum256(data)
		dimensions, decodeErr := png.DecodeConfig(bytes.NewReader(data))
		if len(data) > attachmentLimit || total > 25<<20 || hex.EncodeToString(hash[:]) != capture.SHA256 || decodeErr != nil || dimensions.Width != capture.Width || dimensions.Height != capture.Height {
			respondJSONError(w, 400, "Native screenshot content or dimensions do not match")
			return
		}
		attachment := conversationAttachment{ID: delegationMessageID(task.VisualCaptureAttachmentKey(d.ID, id, capture)), Name: filepath.Base(capture.Path), Type: "image/png", Size: int64(len(data))}
		attachment.URL = "/api/v1/chat/attachments/" + attachment.ID
		files = append(files, preparedTaskArtifact{attachment: attachment, data: data})
	}
	if err := s.publishTaskArtifactsWithKey(r.Context(), d, owner, files, "task-visual-delivery:"+id); err != nil {
		respondJSONError(w, 503, "Native visual files could not be delivered; saved output preserved")
		return
	}
	if previous := task.VisualVerificationForAttempt(d.Activities, d.ID, id, req.RunnerID); previous == nil {
		err = s.taskSvc.Store().CreateActivity(r.Context(), &task.TaskActivity{ID: "native-visual:" + id, TaskID: d.ID, AttemptID: &id, ActivityType: task.ActivityVisualVerification, Author: req.RunnerID, Message: string(raw), CreatedAt: time.Now().UTC()})
		if err != nil {
			respondJSONError(w, 503, "Native visual provenance could not be retained")
			return
		}
	}
	respondJSON(w, 200, map[string]bool{"recorded": true})
}

// Called after the registered project's exact snapshot has been validated and
// before the model capability is issued. The model cannot supply this binding.
var visualCaptureProhibited = regexp.MustCompile(`(?i)\b(?:(?:do not|don't|never|jangan|dilarang)\s+(?:take|capture|create|generate|buat|ambil|membuat|mengambil)\s+|(?:no|tanpa)\s+)(?:new\s+)?(?:screenshots?|tangkapan layar)\b`)

func (s *Server) bindRunnerVisualRecipe(ctx context.Context, claim *runner.ClaimTaskResponse, snapshot string, revision ...string) error {
	if !claim.RequiresVision || claim.LiveInspection == nil {
		return nil
	}
	if visualCaptureProhibited.MatchString(project.TaskScopeInstructions(claim.Task.Instructions) + "\n" + strings.Join(revision, "\n")) {
		return nil
	}
	recipe, err := runner.VisualRecipeAt(snapshot, claim.LiveInspection.TargetCommit)
	if err != nil || recipe == nil {
		return err
	}
	raw, _ := json.Marshal(recipe)
	err = s.taskSvc.Store().CreateActivity(ctx, &task.TaskActivity{ID: "visual-recipe:" + claim.Attempt.ID, TaskID: claim.Task.ID, AttemptID: &claim.Attempt.ID, ActivityType: task.ActivityVisualRecipe, Author: "coordinator", Message: string(raw), CreatedAt: time.Now().UTC()})
	if err == nil {
		claim.VisualRecipe = recipe
	}
	return err
}
