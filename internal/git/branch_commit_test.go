package git

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestBranchCommitUsesSavedAccessAndRejectsUncertainReceipt(t *testing.T) {
	for _, scenario := range []string{"valid", "short", "trailing", "redirect", "unavailable"} {
		t.Run(scenario, func(t *testing.T) {
			calls := 0
			api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				if r.Method != http.MethodGet || r.Header.Get("Authorization") != "Bearer saved-owner" || r.URL.EscapedPath() != "/repos/owner/repo/commits/release%2Fstable" {
					t.Error("lost explicit read-only repository binding")
				}
				switch scenario {
				case "redirect":
					w.Header().Set("Location", "/unexpected")
					w.WriteHeader(302)
				case "unavailable":
					w.WriteHeader(503)
				case "short":
					w.Write([]byte(`{"sha":"abc"}`))
				case "trailing":
					w.Write([]byte(`{"sha":"` + strings.Repeat("a", 40) + `"} {}`))
				default:
					w.Write([]byte(`{"sha":"` + strings.Repeat("a", 40) + `"}`))
				}
			}))
			defer api.Close()
			client := NewClient()
			client.SetAPIBaseURL(api.URL)
			got, err := client.BranchCommit(context.Background(), "owner", "repo", "release/stable", "saved-owner")
			if (err == nil) != (scenario == "valid") || calls != 1 {
				t.Fatalf("unexpected receipt acceptance: %q %v calls=%d", got, err, calls)
			}
			if scenario == "valid" && got != strings.Repeat("a", 40) {
				t.Fatal("revision changed")
			}
			if _, err := client.BranchCommit(context.Background(), "owner", "repo", "release/stable", ""); err == nil || calls != 1 {
				t.Fatal("missing saved access reached remote")
			}
		})
	}
}
