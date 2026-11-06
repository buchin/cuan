package server

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/buchin/cuan/internal/project"
	"github.com/buchin/cuan/internal/publication"
	"github.com/buchin/cuan/internal/task"
)

func TestPublicationReviewRecognizesContainedCommitWithoutWaivingFeatureReview(t *testing.T) {
	for _, scenario := range []string{"contained build", "contained ssh", "unpublished", "replace refs", "grafts", "missing objects", "foreign build", "archived"} {
		t.Run(scenario, func(t *testing.T) {
			ctx := context.Background()
			root := t.TempDir()
			run := func(args ...string) string {
				t.Helper()
				out, err := exec.Command("git", append([]string{"-C", root}, args...)...).CombinedOutput()
				if err != nil {
					t.Fatal(err, string(out))
				}
				return strings.TrimSpace(string(out))
			}
			run("init", "--initial-branch=main")
			run("config", "user.name", "Fixture")
			run("config", "user.email", "fixture@example.invalid")
			if err := os.WriteFile(filepath.Join(root, "feature.txt"), []byte("feature"), 0600); err != nil {
				t.Fatal(err)
			}
			run("add", ".")
			run("commit", "-m", "Implemented feature")
			source := run("rev-parse", "HEAD")
			if err := os.WriteFile(filepath.Join(root, "other.txt"), []byte("later release"), 0600); err != nil {
				t.Fatal(err)
			}
			run("add", ".")
			run("commit", "-m", "Later release")
			target := run("rev-parse", "HEAD")
			if scenario == "unpublished" || scenario == "replace refs" || scenario == "grafts" {
				source = run("commit-tree", "-p", source, "-m", "Unpublished sibling", "HEAD^{tree}")
				if scenario == "replace refs" {
					run("replace", "--graft", target, source)
				}
				if scenario == "grafts" {
					if err := os.WriteFile(filepath.Join(root, ".git", "info", "grafts"), []byte(target+" "+source+"\n"), 0600); err != nil {
						t.Fatal(err)
					}
				}
			}
			if err := os.WriteFile(filepath.Join(root, "feature.txt"), []byte("PRESERVE PENDING"), 0600); err != nil {
				t.Fatal(err)
			}
			before := run("status", "--porcelain=v1")
			refs := run("for-each-ref", "--format=%(refname):%(objectname)")
			s, _, _, _ := inboxFixture(t)
			s.cfg.Environment, s.cfg.SourceRepository, s.cfg.CommitSHA = "production", "fixture/repo", target
			store := project.NewMemoryStore()
			p := &project.Project{ID: "project", RepoURL: "https://github.com/fixture/repo", LocalPath: root}
			if scenario == "contained ssh" {
				s.cfg.SourceRepository = "another/repo"
				p.Metadata = registeredReleaseMetadata(t)
				s.publicationObserver = func(context.Context, publication.Config) (string, error) { return target, nil }
			}
			if scenario == "foreign build" {
				s.cfg.SourceRepository = "another/repo"
			}
			if scenario == "archived" {
				p.IsArchived = true
			}
			if err := store.CreateProject(ctx, p); err != nil {
				t.Fatal(err)
			}
			s.projectSvc = project.NewService(store, nil, "", t.TempDir())
			if scenario == "missing objects" {
				source = strings.Repeat("a", 40)
				s.projectSvc.SetSharedRepositoryAccess(func(context.Context) (string, error) { return "", context.Canceled })
			}
			d := &task.TaskDetailResponse{Task: task.Task{ID: "task", ProjectID: p.ID, CommitSHA: source, Instructions: "Implement and deploy the approved feature", ExecutionCategory: task.ExecutionCategoryCode}, LatestAttempt: &task.TaskAttempt{ID: "attempt", TaskID: "task", Status: task.AttemptStatusSucceeded, CreatedAt: time.Now()}}
			if err := s.taskSvc.Store().CreateTask(ctx, &d.Task); err != nil {
				t.Fatal(err)
			}
			if err := s.taskSvc.Store().CreateAttempt(ctx, d.LatestAttempt); err != nil {
				t.Fatal(err)
			}
			observation := s.inboxReviewPublication(ctx, d)
			want := strings.HasPrefix(scenario, "contained")
			if observation == nil || observation.Verified != want {
				t.Fatal("incorrect publication observation", observation)
			}
			if want && (!strings.HasSuffix(observation.Provenance, "_contains_result_commit") || observation.Commit != source) {
				t.Fatal("ancestry rewritten as exact release", observation)
			}
			gap := inboxReviewEvaluation{Decision: "revise", RemainingWork: "verification", Finding: "Feature evidence missing", Explanation: "Feature evidence missing", Revision: "Check feature behavior"}
			if want && s.requirePublicationReview(ctx, d, gap) != gap {
				t.Fatal("publication changed feature review")
			}
			if d.CommitSHA != source || run("status", "--porcelain=v1") != before || run("for-each-ref", "--format=%(refname):%(objectname)") != refs || run("rev-parse", "HEAD") != target {
				t.Fatal("publication inspection changed repository or task history")
			}
		})
	}
}
