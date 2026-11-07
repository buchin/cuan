package git

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
)

func TestMergePullRequestDoesNotBorrowIdentityOrRepeatUncertainMutation(t *testing.T) {
	for _, scenario := range []string{"missing token", "missing source", "invalid source", "conflict", "unavailable", "invalid response", "not merged", "missing receipt sha", "redirect"} {
		t.Run(scenario, func(t *testing.T) {
			root := t.TempDir()
			marker := filepath.Join(root, "unexpected-gh")
			if err := os.WriteFile(filepath.Join(root, "gh"), []byte("#!/bin/sh\nprintf invoked > \"$GH_FIXTURE_MARKER\"\n"), 0700); err != nil {
				t.Fatal(err)
			}
			t.Setenv("PATH", root+":"+os.Getenv("PATH"))
			t.Setenv("GH_FIXTURE_MARKER", marker)
			t.Setenv("GH_TOKEN", "ambient-wrong-owner")
			t.Setenv("GITHUB_TOKEN", "ambient-other-owner")
			var calls atomic.Int32
			api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				if r.Header.Get("Authorization") != "Bearer saved-owner" {
					t.Error("merge borrowed another identity")
				}
				switch scenario {
				case "redirect":
					w.Header().Set("Location", "/redirected")
					w.WriteHeader(307)
				case "conflict":
					w.WriteHeader(409)
				case "unavailable":
					w.WriteHeader(503)
				case "invalid response":
					w.Write([]byte("lost response"))
				case "not merged":
					w.Write([]byte(`{"merged":false,"sha":"not-accepted"}`))
				default:
					w.Write([]byte(`{"merged":true}`))
				}
			}))
			defer api.Close()
			client := NewClient()
			client.SetAPIBaseURL(api.URL)
			req := MergePullRequestRequest{Owner: "owner", Repo: "repo", PullNumber: 7, Token: "saved-owner", ExpectedHeadSHA: strings.Repeat("a", 40)}
			switch scenario {
			case "missing token":
				req.Token = ""
			case "missing source":
				req.ExpectedHeadSHA = ""
			case "invalid source":
				req.ExpectedHeadSHA = strings.Repeat("z", 40)
			}
			result, err := client.MergePullRequest(context.Background(), req)
			if err == nil || result != nil {
				t.Fatal("unconfirmed merge accepted")
			}
			want := int32(1)
			if scenario == "missing token" || scenario == "missing source" || scenario == "invalid source" {
				want = 0
			}
			if calls.Load() != want {
				t.Fatal("unexpected repeated or unauthorized provider mutation", calls.Load())
			}
			if _, err := os.Stat(marker); !os.IsNotExist(err) {
				t.Fatal("merge retried through ambient gh")
			}
		})
	}
}

func TestPullRequestFailureDoesNotUseAmbientIdentityOrCreateAfterUncertainLookup(t *testing.T) {
	for _, scenario := range []string{"lookup unavailable", "invalid lookup", "create unavailable"} {
		t.Run(scenario, func(t *testing.T) {
			root := t.TempDir()
			marker := filepath.Join(root, "ambient-gh-called")
			if err := os.WriteFile(filepath.Join(root, "gh"), []byte("#!/bin/sh\ntouch \"$GH_FIXTURE_MARKER\"\nexit 1\n"), 0700); err != nil {
				t.Fatal(err)
			}
			t.Setenv("GH_FIXTURE_MARKER", marker)
			t.Setenv("PATH", root+":"+os.Getenv("PATH"))
			t.Setenv("GH_TOKEN", "wrong-host-owner")
			creates := 0
			api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Header.Get("Authorization") != "Bearer saved-owner" {
					t.Error("saved identity lost")
				}
				if r.Method == "POST" {
					creates++
					w.WriteHeader(503)
					return
				}
				if scenario == "lookup unavailable" {
					w.WriteHeader(503)
					return
				}
				if scenario == "invalid lookup" {
					w.Write([]byte("broken"))
					return
				}
				w.Write([]byte(`[]`))
			}))
			defer api.Close()
			client := NewClient()
			client.SetAPIBaseURL(api.URL)
			_, err := client.CreateOrUpdatePullRequest(context.Background(), PullRequestRequest{Owner: "owner", Repo: "repo", HeadBranch: "task/one", Token: "saved-owner"})
			if err == nil {
				t.Fatal("uncertain PR accepted")
			}
			if _, err := os.Stat(marker); !os.IsNotExist(err) {
				t.Fatal("ambient gh identity used", err)
			}
			if scenario != "create unavailable" && creates != 0 || creates > 1 {
				t.Fatal("uncertain lookup created PR", creates)
			}
		})
	}
}

func TestFormatPRBody(t *testing.T) {
	req := PullRequestRequest{
		Title:         "Feature: Isolated Git Workspaces",
		Description:   "Implement isolated workspaces per task",
		DiffSummary:   "2 files changed, 50 insertions(+)",
		Verification:  "Run targeted tests: ./scripts/test.sh --fast",
		TestStatus:    "Targeted tests passed (exit 0)",
		IsRevision:    true,
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

func TestMergePullRequest(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/repos/buchin/cuan/pulls/101/merge", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method != http.MethodPut {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		var payload map[string]interface{}
		_ = json.NewDecoder(r.Body).Decode(&payload)

		if payload["merge_method"] != "squash" {
			t.Errorf("expected merge_method 'squash', got %v", payload["merge_method"])
		}
		if payload["sha"] != strings.Repeat("a", 40) || r.Header.Get("Authorization") != "Bearer saved-owner" {
			t.Error("merge lost source precondition or saved identity")
		}

		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"sha":     "sha1234567890abcdef",
			"merged":  true,
			"message": "Pull Request successfully merged",
		})
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	client := NewClient()
	client.SetAPIBaseURL(server.URL)

	ctx := context.Background()
	result, err := client.MergePullRequest(ctx, MergePullRequestRequest{
		Owner:           "buchin",
		Repo:            "cuan",
		PullNumber:      101,
		CommitTitle:     "feat: squash merge task work",
		CommitMessage:   "Applied via Cuan one-click apply",
		MergeMethod:     "squash",
		Token:           "saved-owner",
		ExpectedHeadSHA: strings.Repeat("a", 40),
	})
	if err != nil {
		t.Fatalf("MergePullRequest failed: %v", err)
	}

	if !result.Merged {
		t.Errorf("expected Merged=true, got false")
	}
	if result.SHA != "sha1234567890abcdef" {
		t.Errorf("expected SHA 'sha1234567890abcdef', got '%s'", result.SHA)
	}
}
