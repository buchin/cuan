package git

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestFormatPRBody(t *testing.T) {
	req := PullRequestRequest{
		Title:        "Feature: Isolated Git Workspaces",
		Description:  "Implement isolated workspaces per task",
		DiffSummary:  "2 files changed, 50 insertions(+)",
		Verification: "Run targeted tests: ./scripts/test.sh --fast",
		TestStatus:   "Targeted tests passed (exit 0)",
		IsRevision:   true,
		RevisionNotes: "Updated branch protection hook and added pre-push guardrail",
	}

	body := FormatPRBody(req)

	if !strings.Contains(body, "## Description") {
		t.Errorf("missing Description section")
	}
	if !strings.Contains(body, "Implement isolated workspaces per task") {
		t.Errorf("missing description text")
	}
	if !strings.Contains(body, "2 files changed, 50 insertions(+)") {
		t.Errorf("missing diff summary")
	}
	if !strings.Contains(body, "## Verification & Testing Instructions") {
		t.Errorf("missing verification section")
	}
	if !strings.Contains(body, "Targeted tests passed (exit 0)") {
		t.Errorf("missing test status")
	}
	if !strings.Contains(body, "## Revision History & Checkpoint") {
		t.Errorf("missing revision history section")
	}
	if !strings.Contains(body, "Updated branch protection hook") {
		t.Errorf("missing revision notes")
	}
}

func TestCreateOrUpdatePullRequest_Cycle(t *testing.T) {
	var prDatabase []map[string]interface{}
	nextPRNumber := 101

	mux := http.NewServeMux()

	// Handler for /repos/buchin/cuan/pulls
	mux.HandleFunc("/repos/buchin/cuan/pulls", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		if r.Method == http.MethodGet {
			// Query existing PR by head
			headParam := r.URL.Query().Get("head")
			var matches []map[string]interface{}
			for _, pr := range prDatabase {
				headRef := pr["head"].(map[string]interface{})["ref"].(string)
				ownerHead := "buchin:" + headRef
				if (headRef == headParam || ownerHead == headParam) && pr["state"] == "open" {
					matches = append(matches, pr)
				}
			}
			_ = json.NewEncoder(w).Encode(matches)
			return
		}

		if r.Method == http.MethodPost {
			// Create new PR
			var payload map[string]string
			_ = json.NewDecoder(r.Body).Decode(&payload)

			newPR := map[string]interface{}{
				"number":   nextPRNumber,
				"html_url": "https://github.com/buchin/cuan/pull/" + string(rune('0'+nextPRNumber%10)),
				"title":    payload["title"],
				"body":     payload["body"],
				"state":    "open",
				"head": map[string]interface{}{
					"ref": payload["head"],
				},
				"base": map[string]interface{}{
					"ref": payload["base"],
				},
			}
			nextPRNumber++
			prDatabase = append(prDatabase, newPR)
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(newPR)
			return
		}

		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	})

	// Handler for /repos/buchin/cuan/pulls/101 (update)
	mux.HandleFunc("/repos/buchin/cuan/pulls/101", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodPatch {
			var payload map[string]string
			_ = json.NewDecoder(r.Body).Decode(&payload)

			for _, pr := range prDatabase {
				if pr["number"] == 101 {
					pr["title"] = payload["title"]
					pr["body"] = payload["body"]
					_ = json.NewEncoder(w).Encode(pr)
					return
				}
			}
			http.NotFound(w, r)
			return
		}
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	client := NewClient()
	client.SetAPIBaseURL(server.URL)
	ctx := context.Background()

	// 1. Initial Cycle: Create PR
	pr1, err := client.CreateOrUpdatePullRequest(ctx, PullRequestRequest{
		Owner:        "buchin",
		Repo:         "cuan",
		Title:        "feat: initial task work",
		HeadBranch:   "task/task_rev_test",
		BaseBranch:   "main",
		Description:  "First attempt",
		DiffSummary:  "+10 lines",
		Verification: "Run test",
		TestStatus:   "PASS (exit 0)",
		IsRevision:   false,
	})
	if err != nil {
		t.Fatalf("create PR failed: %v", err)
	}

	if pr1.Number != 101 {
		t.Fatalf("expected PR #101, got %d", pr1.Number)
	}
	if pr1.IsUpdated {
		t.Fatalf("first PR call should be a create, not update")
	}

	// 2. Revision Cycle: Update existing PR without duplicate!
	pr2, err := client.CreateOrUpdatePullRequest(ctx, PullRequestRequest{
		Owner:         "buchin",
		Repo:          "cuan",
		Title:         "feat: initial task work (revised)",
		HeadBranch:    "task/task_rev_test",
		BaseBranch:    "main",
		Description:   "Second attempt (revision)",
		DiffSummary:   "+25 lines",
		Verification:  "Run test",
		TestStatus:    "PASS (exit 0)",
		IsRevision:    true,
		RevisionNotes: "Fixed review comments",
	})
	if err != nil {
		t.Fatalf("revision update PR failed: %v", err)
	}

	if pr2.Number != 101 {
		t.Fatalf("expected updated PR to maintain number #101, got %d (DUPLICATE PR DETECTED!)", pr2.Number)
	}
	if !pr2.IsUpdated {
		t.Fatalf("second PR call should be marked as IsUpdated=true")
	}

	// Verify database only has 1 PR
	if len(prDatabase) != 1 {
		t.Fatalf("expected exactly 1 PR in database, got %d (duplicate created!)", len(prDatabase))
	}
}
