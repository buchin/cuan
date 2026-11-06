package server

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/buchin/cuan/internal/project"
	"github.com/buchin/cuan/internal/publication"
	"github.com/buchin/cuan/internal/task"
	"golang.org/x/crypto/ssh"
)

func registeredReleaseMetadata(t *testing.T) json.RawMessage {
	t.Helper()
	_, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	signer, err := ssh.NewSignerFromKey(private)
	if err != nil {
		t.Fatal(err)
	}
	env := "CUAN_VERIFICATION_PHPUNIT_TARGETS=[\"tests/Feature/ExampleTest.php\"]\nCUAN_PUBLICATION_VERIFIER=ssh-release\nCUAN_PUBLICATION_SSH_HOST=project.invalid\nCUAN_PUBLICATION_SSH_USER=fixture\nCUAN_PUBLICATION_SSH_PASSWORD=fixture\nCUAN_PUBLICATION_SSH_HOST_KEY=" + strings.TrimSpace(string(ssh.MarshalAuthorizedKey(signer.PublicKey()))) + "\nCUAN_PUBLICATION_DEPLOY_PATH=/srv/project\nCUAN_PUBLICATION_HEALTH_URL=https://project.invalid/up\n"
	raw, _ := json.Marshal(map[string]any{"cuan_settings": project.Settings{Environment: env}})
	return raw
}

func TestExternalReleaseUsesRegisteredConnectionAndInvalidatesCache(t *testing.T) {
	for _, scenario := range []string{"healthy", "unavailable", "malformed", "revoked during observation"} {
		t.Run(scenario, func(t *testing.T) {
			s, _, _, _ := inboxFixture(t)
			s.cfg.Environment = "production"
			store := project.NewMemoryStore()
			s.projectSvc = project.NewService(store, nil, "", t.TempDir())
			p := &project.Project{ID: "external", RepoURL: "https://github.com/example/external", Metadata: registeredReleaseMetadata(t)}
			ctx := context.Background()
			if err := store.CreateProject(ctx, p); err != nil {
				t.Fatal(err)
			}
			calls := 0
			s.publicationObserver = func(_ context.Context, c publication.Config) (string, error) {
				calls++
				if c.Host != "project.invalid" || c.User != "fixture" {
					t.Fatal("wrong connection")
				}
				switch scenario {
				case "unavailable":
					return "", errors.New("unavailable")
				case "malformed":
					return "not-a-revision", nil
				case "revoked during observation":
					p.Metadata = nil
					_ = store.UpdateProject(ctx, p)
				}
				return strings.Repeat("b", 40), nil
			}
			d := &task.TaskDetailResponse{Task: task.Task{ID: "task", ProjectID: p.ID, CommitSHA: strings.Repeat("a", 40)}}
			got := s.runningProjectRelease(ctx, d)
			if (got != nil) != (scenario == "healthy") {
				t.Fatal("untrusted observation", got)
			}
			if got != nil && (got.Commit == d.CommitSHA || got.Provenance != "registered_ssh_release" || got.Repository != "example/external") {
				t.Fatal("observation attributed to unpublished task")
			}
			_ = s.runningProjectRelease(ctx, d)
			if calls != 1 {
				t.Fatal("repeated network operation", calls)
			}
			p.Metadata = nil
			_ = store.UpdateProject(ctx, p)
			if s.runningProjectRelease(ctx, d) != nil {
				t.Fatal("revoked configuration reused")
			}
		})
	}
}
