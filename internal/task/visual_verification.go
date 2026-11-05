package task

import (
	"encoding/json"
	"path/filepath"
	"strings"
)

const VisualVerificationScript = "scripts/verify-ui.sh"
const ActivityVisualRecipe = "visual_recipe_bound"
const ActivityVisualVerification = "visual_verification"

type VisualRecipe struct {
	Script       string `json:"script"`
	ScriptHash   string `json:"script_hash"`
	SourceCommit string `json:"source_commit"`
}

func (r *VisualRecipe) Valid() bool {
	return r != nil && r.Script == VisualVerificationScript && validVerificationHash(r.ScriptHash) && fullObjectID(r.SourceCommit)
}

type VisualCapture struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
	Width  int    `json:"width"`
	Height int    `json:"height"`
}

type VisualVerification struct {
	Recipe     VisualRecipe    `json:"recipe"`
	Baseline   string          `json:"baseline"`
	Components []string        `json:"components"`
	Captures   []VisualCapture `json:"captures"`
}

func (v *VisualVerification) Valid() bool {
	if v == nil || !v.Recipe.Valid() || !validVerificationHash(v.Baseline) || len(v.Components) == 0 || len(v.Components) > 16 || len(v.Captures) == 0 || len(v.Captures) > 8 {
		return false
	}
	seen := map[string]bool{}
	for _, component := range v.Components {
		if !visualPath(component) || seen[component] {
			return false
		}
		seen[component] = true
	}
	seen = map[string]bool{}
	for _, capture := range v.Captures {
		if !visualPath(capture.Path) || !strings.HasPrefix(capture.Path, "native-visual/") || !strings.HasSuffix(capture.Path, ".png") || !validVerificationHash(capture.SHA256) || capture.Width < 1 || capture.Height < 1 || capture.Width > 16384 || capture.Height > 16384 || seen[capture.Path] {
			return false
		}
		seen[capture.Path] = true
	}
	return true
}
func visualPath(path string) bool {
	if !filepath.IsLocal(path) || len(path) > 512 || strings.ContainsAny(path, "\\\r\n\t") {
		return false
	}
	for _, part := range strings.Split(path, "/") {
		if strings.HasPrefix(part, ".") || part == "" {
			return false
		}
	}
	return true
}
func VisualCaptureAttachmentKey(taskID, attemptID string, capture VisualCapture) string {
	// The same delivery identity used by readTaskArtifacts for session files.
	if !validVerificationHash(capture.SHA256) {
		return ""
	}
	return "task-artifact:" + taskID + ":" + attemptID + ":session:" + capture.Path + ":" + capture.SHA256
}

func BoundVisualRecipe(activities []*TaskActivity, taskID, attemptID string) (*VisualRecipe, error) {
	var found *VisualRecipe
	for _, activity := range activities {
		if activity == nil || activity.TaskID != taskID || activity.AttemptID == nil || *activity.AttemptID != attemptID || activity.ActivityType != ActivityVisualRecipe || activity.Author != "coordinator" {
			continue
		}
		var recipe VisualRecipe
		if json.Unmarshal([]byte(activity.Message), &recipe) != nil || !recipe.Valid() || (found != nil && *found != recipe) {
			return nil, ErrInvalidTaskInput
		}
		found = &recipe
	}
	return found, nil
}

func VisualVerificationForAttempt(activities []*TaskActivity, taskID, attemptID, runnerID string) *VisualVerification {
	recipe, err := BoundVisualRecipe(activities, taskID, attemptID)
	if err != nil || recipe == nil {
		return nil
	}
	for _, activity := range activities {
		if activity == nil || activity.TaskID != taskID || activity.AttemptID == nil || *activity.AttemptID != attemptID || activity.Author != runnerID || activity.ActivityType != ActivityVisualVerification {
			continue
		}
		var receipt VisualVerification
		if json.Unmarshal([]byte(activity.Message), &receipt) == nil && receipt.Valid() && receipt.Recipe == *recipe {
			return &receipt
		}
	}
	return nil
}
