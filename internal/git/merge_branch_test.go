package git

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

func TestRemoteBranchMergePinsSourceAndDoesNotReplay(t *testing.T) {
	for _, scenario := range []string{"success", "missing access", "invalid source", "conflict", "unavailable", "already contained", "invalid receipt", "redirect", "preexisting", "observation failed", "inconsistent ancestry", "unknown ancestry"} {
		t.Run(scenario, func(t *testing.T) {
			t.Setenv("GH_TOKEN", "wrong-host-owner")
			var calls atomic.Int32
			source, merged := strings.Repeat("a", 40), strings.Repeat("b", 40)
			api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Header.Get("Authorization") != "Bearer saved-owner" {
					t.Error("wrong observation identity")
				}
				if r.Method == "GET" {
					if scenario == "observation failed" {
						w.WriteHeader(503)
						return
					}
					if strings.Contains(r.URL.Path, "/commits/") {
						json.NewEncoder(w).Encode(map[string]string{"sha": merged})
						return
					}
					status, ancestor := "diverged", strings.Repeat("c", 40)
					if scenario == "inconsistent ancestry" {
						status = "ahead"
					}
					if scenario == "unknown ancestry" {
						status = "unknown"
					}
					if scenario == "preexisting" || (scenario == "already contained" && calls.Load() > 0) {
						status, ancestor = "ahead", source
					}
					json.NewEncoder(w).Encode(map[string]any{"status": status, "merge_base_commit": map[string]string{"sha": ancestor}})
					return
				}
				calls.Add(1)
				var payload map[string]string
				if r.Method != "POST" || r.URL.Path != "/repos/owner/repo/merges" || r.Header.Get("Authorization") != "Bearer saved-owner" || json.NewDecoder(r.Body).Decode(&payload) != nil || payload["head"] != source || payload["base"] != "dev" {
					t.Error("merge changed registered source, target or identity")
				}
				switch scenario {
				case "conflict":
					w.WriteHeader(409)
				case "unavailable":
					w.WriteHeader(503)
				case "already contained":
					w.WriteHeader(204)
				case "redirect":
					w.Header().Set("Location", "/redirected")
					w.WriteHeader(307)
				case "invalid receipt":
					w.WriteHeader(201)
					w.Write([]byte(`{"sha":"unknown"}`))
				default:
					w.WriteHeader(201)
					json.NewEncoder(w).Encode(map[string]string{"sha": merged})
				}
			}))
			defer api.Close()
			client := NewClient()
			client.SetAPIBaseURL(api.URL)
			req := MergeBranchRequest{Owner: "owner", Repo: "repo", BaseBranch: "dev", SourceCommit: source, Token: "saved-owner"}
			if scenario == "missing access" {
				req.Token = ""
			}
			if scenario == "invalid source" {
				req.SourceCommit = "HEAD"
			}
			result, err := client.MergeBranch(context.Background(), req)
			if scenario == "success" || scenario == "preexisting" || scenario == "already contained" {
				if err != nil || result == nil || !result.Merged || result.SHA != merged {
					t.Fatal("exact remote merge not confirmed", err)
				}
			} else if err == nil || result != nil {
				t.Fatal("uncertain merge accepted")
			}
			if errors.Is(err, ErrMergeConflict) != (scenario == "conflict") {
				t.Fatal("confirmed merge conflict conflated with uncertain publication", err)
			}
			want := int32(1)
			if scenario == "missing access" || scenario == "invalid source" || scenario == "preexisting" || scenario == "observation failed" || scenario == "inconsistent ancestry" || scenario == "unknown ancestry" {
				want = 0
			}
			if calls.Load() != want {
				t.Fatal("replayed or unauthorized mutation", calls.Load())
			}
		})
	}
}
