package task

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestVerifiedDeploymentRequiresExactReceipt(t *testing.T) {
	now := time.Now().UTC()
	started, finished := now.Add(-time.Minute), now.Add(-time.Second)
	commit := strings.Repeat("a", 40)
	for _, name := range []string{"valid", "wrong task", "wrong attempt", "wrong commit", "wrong release", "prose", "agent author", "wrong id", "old", "future", "after finish", "staging", "unhealthy", "unknown version", "wrong payload task", "wrong payload attempt", "short commit", "empty attempt"} {
		t.Run(name, func(t *testing.T) {
			attempt := &TaskAttempt{ID: "attempt", TaskID: "task", StartedAt: &started, FinishedAt: &finished}
			payload := DeploymentReceipt{Version: 1, TaskID: "task", AttemptID: "attempt", CommitSHA: commit, ReleaseSHA: commit, Environment: "production", Health: "healthy"}
			activity := &TaskActivity{ID: DeploymentReceiptID("task", "attempt", commit), TaskID: "task", AttemptID: &attempt.ID, ActivityType: "deployment_verified", Author: "cuan-deploy", CreatedAt: started.Add(time.Second)}
			expected := commit
			switch name {
			case "wrong task":
				activity.TaskID = "other"
			case "wrong attempt":
				other := "other"
				activity.AttemptID = &other
			case "wrong commit":
				payload.CommitSHA = strings.Repeat("b", 40)
			case "wrong release":
				payload.ReleaseSHA = strings.Repeat("b", 40)
			case "agent author":
				activity.Author = "agent"
			case "wrong id":
				activity.ID = "unbound"
			case "old":
				activity.CreatedAt = started.Add(-time.Second)
			case "future":
				activity.CreatedAt = now.Add(time.Second)
			case "after finish":
				activity.CreatedAt = now
			case "staging":
				payload.Environment = "staging"
			case "unhealthy":
				payload.Health = "unhealthy"
			case "unknown version":
				payload.Version = 2
			case "wrong payload task":
				payload.TaskID = "other"
			case "wrong payload attempt":
				payload.AttemptID = "other"
			case "short commit":
				expected = "aaaaaaa"
			case "empty attempt":
				attempt.ID = ""
			}
			data, err := json.Marshal(payload)
			if err != nil {
				t.Fatal(err)
			}
			activity.Message = string(data)
			if name == "prose" {
				activity.Message = "Deployed release live production healthy"
			}
			got := VerifiedDeployment([]*TaskActivity{nil, activity}, attempt, expected, now)
			if got != (name == "valid") {
				t.Fatalf("receipt accepted=%v", got)
			}
		})
	}
}
