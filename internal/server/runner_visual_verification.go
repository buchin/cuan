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
var visualCaptureProhibited = regexp.MustCompile(`(?i)\b(?:(?:do not|don't|never|jangan|dilarang)\s+(?:take|capture|create|generate|use|buat|ambil|membuat|mengambil|gunakan|menggunakan|pakai)\s+|(?:no|without|tanpa)\s+)(?:(?:(?:browser\s+automation|(?:otomasi|otomatisasi)\s+browser)\s+(?:or|and|atau|dan)\s+))?(?:new\s+)?(?:screenshots?|tangkapan layar)\b`)

// Reading an attached image remains allowed; an explicit browser prohibition
// prevents the native recipe from launching browser automation on the owner's behalf.
var browserAutomationProhibited = regexp.MustCompile(`(?i)\b(?:(?:do not|don't|never|jangan|dilarang)\s+(?:use|run|launch|open|invoke|gunakan|menggunakan|pakai|memakai|jalankan|menjalankan|buka|membuka)\s+(?:(?:a|an|the|any|automated|headless|automation|otomasi|otomatisasi|alat|tool|tools)\s+)*(?:browser|playwright|puppeteer|chromium)|(?:no|without|tanpa)\s+(?:(?:otomasi|otomatisasi|headless)\s+)?(?:browser|playwright|puppeteer|chromium))\b`)

func (s *Server) bindRunnerVisualRecipe(ctx context.Context, claim *runner.ClaimTaskResponse, snapshot string, revision ...string) error {
	if !claim.RequiresVision || claim.LiveInspection == nil {
		return nil
	}
	scope := project.TaskScopeInstructions(claim.Task.Instructions) + "\n" + strings.Join(revision, "\n")
	if visualCaptureProhibited.MatchString(scope) || browserAutomationProhibited.MatchString(scope) {
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

var originalVisualScope = regexp.MustCompile(`(?i)\b(?:ui|ux|frontend|front-end|layout|typography|styling|antarmuka|tampilan|desain|visual)\b|\.(?:tsx|jsx|css)\b`)

// Remove only explicit mutation prohibitions, not visual inspection duties.
// The bounded list vocabulary also covers "no push, deploy, or UI changes"
// without swallowing a later request to inspect the UI.
var visualMutationProhibition = regexp.MustCompile(`(?i)\b(?:no|without|tanpa|tidak ada|jangan|dilarang|do not|don't|never)\s+(?:(?:push|pr|deploy|deployment|perubahan konfigurasi|configuration changes?)(?:\s*,\s*|\s+(?:or|and|atau|dan)\s+))*(?:(?:perubahan|modifikasi|ubah|mengubah|menyentuh|change|changes? to|modify|edit|touch)\s+(?:(?:komponen|components?|elemen)\s+)?(?:ui|ux|frontend|front-end|layout|typography|styling|antarmuka|tampilan|desain|visual)\b|(?:ui|ux|frontend|front-end|layout|typography|styling|antarmuka|tampilan|desain|visual)\s+changes?\b)`)

var commandOutputScope = regexp.MustCompile(`(?i)--help\b|\b(?:cli|command-line|terminal|stdout|stderr)\b`)
var webOutputScope = regexp.MustCompile(`(?i)\b(?:halaman|page|browser|web|component|komponen|panel)\b`)
var ambiguousDisplayScope = regexp.MustCompile(`(?i)\btampilan\b`)

func hasOriginalVisualScope(text string) bool {
	text = visualMutationProhibition.ReplaceAllString(text, "")
	text = visualCaptureProhibited.ReplaceAllString(text, "")
	for _, line := range strings.Split(text, "\n") {
		// "Tampilan output --help" describes terminal text, not a request for
		// browser screenshots. Keep explicit UI/layout and web requirements.
		if commandOutputScope.MatchString(line) && !webOutputScope.MatchString(line) {
			line = ambiguousDisplayScope.ReplaceAllString(line, "")
		}
		if originalVisualScope.MatchString(line) {
			return true
		}
	}
	return false
}

// runnerClaimRequiresVision classifies task evidence requirements separately
// from the native inspection's generic transport instructions.
func (s *Server) runnerClaimRequiresVision(ctx context.Context, claim *runner.ClaimTaskResponse, detail *task.TaskDetailResponse, revision string) bool {
	// The native claim has already validated this inspection. Its revision
	// contains conditional screenshot guidance for every kind of task; that
	// boilerplate is not a visual acceptance requirement.
	if claim.LiveInspection != nil {
		revision = ""
		// Keep visual work subject to the repository's visual quality gate
		// even when the owner did not literally ask for a screenshot.
		if detail != nil {
			if hasOriginalVisualScope(project.TaskScopeInstructions(detail.Instructions) + "\n" + detail.Title) {
				return true
			}
			for _, criterion := range detail.DoDChecklist {
				if hasOriginalVisualScope(criterion.Text) {
					return true
				}
			}
		}
	}
	return s.taskRequiresVision(ctx, claim.OwnerID, detail, "", revision)
}
