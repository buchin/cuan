package task

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestVisualVerificationBindsRecipeRunnerAndAttempt(t *testing.T) {
	recipe := VisualRecipe{Script: VisualVerificationScript, ScriptHash: strings.Repeat("a", 64), SourceCommit: strings.Repeat("b", 40)}
	receipt := VisualVerification{Recipe: recipe, Baseline: strings.Repeat("c", 64), Components: []string{"web/Component.tsx"}, Captures: []VisualCapture{{Path: "native-visual/mobile.png", SHA256: strings.Repeat("d", 64), Width: 390, Height: 844}}}
	for _, scenario := range []string{"valid", "agent binding", "agent capture", "other attempt", "other recipe", "escape", "hidden", "no coverage", "duplicate", "wrong dimensions", "wrong hash"} {
		t.Run(scenario, func(t *testing.T) {
			value := receipt
			value.Captures = append([]VisualCapture{}, receipt.Captures...)
			id := "attempt"
			bindingAuthor, captureAuthor := "coordinator", "runner"
			switch scenario {
			case "agent binding":
				bindingAuthor = "agent"
			case "agent capture":
				captureAuthor = "agent"
			case "other attempt":
				id = "other"
			case "other recipe":
				value.Recipe.ScriptHash = strings.Repeat("e", 64)
			case "escape":
				value.Captures[0].Path = "native-visual/../secret.png"
			case "hidden":
				value.Captures[0].Path = "native-visual/.secret.png"
			case "no coverage":
				value.Components = nil
			case "duplicate":
				value.Captures = append(value.Captures, value.Captures[0])
			case "wrong dimensions":
				value.Captures[0].Width = 0
			case "wrong hash":
				value.Captures[0].SHA256 = "fake"
			}
			rawRecipe, _ := json.Marshal(recipe)
			rawReceipt, _ := json.Marshal(value)
			activities := []*TaskActivity{{TaskID: "task", AttemptID: &id, ActivityType: ActivityVisualRecipe, Author: bindingAuthor, Message: string(rawRecipe)}, {TaskID: "task", AttemptID: &id, ActivityType: ActivityVisualVerification, Author: captureAuthor, Message: string(rawReceipt)}}
			verified := VisualVerificationForAttempt(activities, "task", "attempt", "runner")
			if (verified != nil) != (scenario == "valid") {
				t.Fatal("unbound visual provenance was accepted")
			}
		})
	}
}
