package server

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"errors"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/buchin/cuan/internal/project"
	"github.com/buchin/cuan/internal/publication"
	"github.com/buchin/cuan/internal/task"
	"golang.org/x/crypto/ssh"
)

func TestNativePublicationObservationBoundToActiveRegisteredResult(t *testing.T) {
	for _, scenario := range []string{"verified", "unverified", "cancelled-during-check", "connection-revoked", "archived-during-check", "repository-changed", "branch-changed", "wrong-runner", "wrong-commit", "no-connection", "unauthenticated", "model-receipt", "process-verified", "process-wrong-repository", "process-wrong-revision", "process-development"} {
		t.Run(scenario, func(t *testing.T) {
			s, token := setupTestChatServer(t)
			ctx := context.Background()
			projects := project.NewMemoryStore()
			_, key, _ := ed25519.GenerateKey(rand.Reader)
			signer, _ := ssh.NewSignerFromKey(key)
			environment := "CUAN_PUBLICATION_VERIFIER=ssh-release\nCUAN_PUBLICATION_SSH_HOST=project.invalid\nCUAN_PUBLICATION_SSH_USER=project-user\nCUAN_PUBLICATION_SSH_PASSWORD=private-project-password\nCUAN_PUBLICATION_SSH_HOST_KEY=" + strings.TrimSpace(string(ssh.MarshalAuthorizedKey(signer.PublicKey()))) + "\nCUAN_PUBLICATION_DEPLOY_PATH=/var/www/project\nCUAN_PUBLICATION_HEALTH_URL=https://project.invalid/up"
			if scenario == "no-connection" || strings.HasPrefix(scenario, "process-") {
				environment = ""
			}
			metadata, _ := json.Marshal(map[string]any{"cuan_settings": project.Settings{Environment: environment}})
			p := &project.Project{ID: "publication-project", Name: "Project", Metadata: metadata}
			if err := projects.CreateProject(ctx, p); err != nil {
				t.Fatal(err)
			}
			s.projectSvc = project.NewService(projects, nil, "", t.TempDir())
			commit := strings.Repeat("a", 40)
			if strings.HasPrefix(scenario, "process-") {
				p.RepoURL = "https://github.com/owner/repo.git"
				if err := projects.UpdateProject(ctx, p); err != nil {
					t.Fatal(err)
				}
				s.cfg.Environment, s.cfg.SourceRepository, s.cfg.CommitSHA = "production", "owner/repo", commit
				switch scenario {
				case "process-wrong-repository":
					s.cfg.SourceRepository = "foreign/repo"
				case "process-wrong-revision":
					s.cfg.CommitSHA = strings.Repeat("b", 40)
				case "process-development":
					s.cfg.Environment = "development"
				}
			}
			item := &task.Task{ID: "publication-task", ProjectID: p.ID, Title: "Approved result", Status: task.TaskStatusDoing, CommitSHA: commit}
			if err := s.taskSvc.Store().CreateTask(ctx, item); err != nil {
				t.Fatal(err)
			}
			started := time.Now().Add(-time.Minute)
			attempt := &task.TaskAttempt{ID: "publication-attempt", TaskID: item.ID, RunnerID: "runner", Status: task.AttemptStatusRunning, CreatedAt: started, StartedAt: &started}
			if err := s.taskSvc.Store().CreateAttempt(ctx, attempt); err != nil {
				t.Fatal(err)
			}
			calls := 0
			s.publicationVerifier = func(ctx context.Context, c publication.Config, expected string) error {
				calls++
				if expected != commit || c.Password != "private-project-password" || c.Host != "project.invalid" {
					t.Error("caller replaced registered target")
				}
				if scenario == "unverified" {
					return errors.New("private transport diagnostic")
				}
				if scenario == "cancelled-during-check" {
					attempt.CancelRequested = true
					if err := s.taskSvc.Store().UpdateAttempt(ctx, attempt); err != nil {
						t.Fatal(err)
					}
				}
				if scenario == "repository-changed" || scenario == "branch-changed" {
					if scenario == "repository-changed" {
						p.RepoURL = "https://github.com/foreign/repo.git"
					} else {
						p.DefaultBranch = "other"
					}
					if err := projects.UpdateProject(ctx, p); err != nil {
						t.Fatal(err)
					}
				}
				if scenario == "archived-during-check" {
					p.IsArchived = true
					if err := projects.UpdateProject(ctx, p); err != nil {
						t.Fatal(err)
					}
				}
				if scenario == "connection-revoked" {
					p.Metadata = json.RawMessage(`{}`)
					if err := projects.UpdateProject(ctx, p); err != nil {
						t.Fatal(err)
					}
				}
				return nil
			}
			runner := "runner"
			if scenario == "wrong-runner" {
				runner = "other"
			}
			expected := commit
			if scenario == "wrong-commit" {
				expected = strings.Repeat("b", 40)
			}
			payload := map[string]any{"task_id": item.ID, "runner_id": runner, "commit_sha": expected}
			if scenario == "model-receipt" {
				payload["verified"] = true
			}
			raw, _ := json.Marshal(payload)
			req := httptest.NewRequest("POST", "/api/v1/runner/attempts/"+attempt.ID+"/publication/verify", strings.NewReader(string(raw)))
			if scenario != "unauthenticated" {
				req.Header.Set("Authorization", "Bearer "+token)
			}
			rec := httptest.NewRecorder()
			s.Router().ServeHTTP(rec, req)
			want := 200
			switch scenario {
			case "unverified":
				want = 422
			case "cancelled-during-check", "connection-revoked", "archived-during-check", "repository-changed", "branch-changed", "wrong-runner", "wrong-commit":
				want = 409
			case "no-connection", "process-wrong-repository", "process-wrong-revision", "process-development":
				want = 503
			case "unauthenticated":
				want = 401
			case "model-receipt":
				want = 400
			}
			if rec.Code != want {
				t.Fatalf("status=%d want=%d body=%s", rec.Code, want, rec.Body.String())
			}
			if strings.Contains(rec.Body.String(), "private-") {
				t.Fatal("credential or diagnostic leaked")
			}
			d, err := s.taskSvc.GetTaskDetail(ctx, item.ID)
			if err != nil {
				t.Fatal(err)
			}
			if task.VerifiedDeployment(d.Activities, d.LatestAttempt, commit, time.Now().UTC()) != (scenario == "verified" || scenario == "process-verified") {
				t.Fatal("publication fact recorded without independent active observation")
			}
			if scenario == "verified" && calls != 1 {
				t.Fatal("missing native observation")
			}
			if strings.HasPrefix(scenario, "process-") && calls != 0 {
				t.Fatal("current process borrowed an SSH connection")
			}
			if scenario == "verified" {
				repeated := httptest.NewRequest("POST", "/api/v1/runner/attempts/"+attempt.ID+"/publication/verify", strings.NewReader(string(raw)))
				repeated.Header.Set("Authorization", "Bearer "+token)
				again := httptest.NewRecorder()
				s.Router().ServeHTTP(again, repeated)
				if again.Code != 200 || calls != 1 {
					t.Fatal("verified observation was not idempotent")
				}
			}
			if d.Status != task.TaskStatusDoing || d.LatestAttempt.Status != task.AttemptStatusRunning {
				t.Fatal("observation accepted or completed work")
			}
		})
	}
}
