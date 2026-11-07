package server

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/buchin/cuan/internal/executionpolicy"
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
			if scenario == "unpublished" {
				if observation.ResultCommitContained == nil || *observation.ResultCommitContained || observation.ObservedRelease != target {
					t.Fatal("known missing publication was conflated with unavailable proof", observation)
				}
				repair := inboxReviewEvaluation{Decision: "revise", RemainingWork: "implementation", Finding: "Approved result commit is not in the release", Explanation: "Complete approved integration", Revision: "Reconcile existing work, integrate the approved result and deploy only remaining changes", ManagerAction: "replan", ManagerReason: "Independent audit and native ancestry show the result is not published"}
				actual := s.requirePublicationReview(ctx, d, repair)
				if actual.RemainingWork != "implementation" || actual.Finding != repair.Finding || actual.ManagerAction != repair.ManagerAction {
					t.Fatal("confirmed missing release sent back to another readonly audit", actual)
				}
				if again := s.requirePublicationReview(ctx, d, actual); again != actual {
					t.Fatal("publication handoff changed when rechecked")
				}
				repository := s.projectStudioRepo
				s.projectStudioRepo = nil
				if unknown := s.requirePublicationReview(ctx, d, repair); unknown.RemainingWork != "verification" {
					t.Fatal("unavailable owner limits inferred execution authority", unknown)
				}
				s.projectStudioRepo = repository
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

// Exercise the actual review -> durable launch handoff, not just classification.
// A new launch must use the original approval and retain existing attempt history.
func TestConfirmedMissingPublicationReturnsToDev(t *testing.T) {
	ctx := context.Background()
	s, d := managerFixture(t)
	q := reviewRequests(t, s, d.SourceRoomID)[0]
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
	if err := os.WriteFile(filepath.Join(root, "result.txt"), []byte("old release"), 0600); err != nil {
		t.Fatal(err)
	}
	run("add", ".")
	run("commit", "-m", "Existing release")
	released := run("rev-parse", "HEAD")
	if err := os.WriteFile(filepath.Join(root, "result.txt"), []byte("approved result"), 0600); err != nil {
		t.Fatal(err)
	}
	run("add", ".")
	run("commit", "-m", "Approved task result")
	result := run("rev-parse", "HEAD")
	approved := "Implement the approved result, commit and deploy it."
	d.Instructions = "Cuan managed delegation v1.\n" + approved + "\nApproval message: " + d.SourceMessageID
	d.CommitSHA = result
	d.ExecutionCategory = task.ExecutionCategoryCode
	if err := s.taskSvc.Store().UpdateTask(ctx, &d.Task); err != nil {
		t.Fatal(err)
	}
	origin, err := s.projectStudioRepo.GetMessageByID(ctx, d.SourceMessageID)
	if err != nil {
		t.Fatal(err)
	}
	origin.Content = approved
	if err := s.projectStudioRepo.UpdateMessage(ctx, origin); err != nil {
		t.Fatal(err)
	}
	store := project.NewMemoryStore()
	if err := store.CreateProject(ctx, &project.Project{ID: d.ProjectID, RepoURL: "https://github.com/fixture/repo", LocalPath: root}); err != nil {
		t.Fatal(err)
	}
	s.projectSvc = project.NewService(store, nil, "", t.TempDir())
	s.cfg.Environment, s.cfg.SourceRepository, s.cfg.CommitSHA = "production", "fixture/repo", released
	if err := s.taskSvc.Store().CreateTask(ctx, &task.Task{ID: "busy", ProjectID: d.ProjectID, Status: task.TaskStatusDoing, ExecutionCategory: task.ExecutionCategoryCode}); err != nil {
		t.Fatal(err)
	}
	decision := inboxReviewEvaluation{Decision: "revise", RemainingWork: "implementation", Finding: "Hasil yang disetujui belum diterapkan", Explanation: "Integrasi hasil masih perlu diselesaikan", Revision: "Reconcile existing release effects and integrate only the approved changes, then verify publication", ManagerAction: "replan", ManagerReason: "Native reconciliation shows the approved result commit is absent from the release"}
	raw, _ := json.Marshal(decision)
	output := s.applyInboxReviewEvaluation(ctx, q, string(raw))
	launch, err := s.taskSvc.GetLaunch(ctx, task.LaunchID(d.ID, q.RequestID))
	if err != nil || launch == nil || launch.State != "queued" {
		t.Fatal("PM did not persist dev continuation", err, output)
	}
	limits := executionpolicy.FromInstructions(launch.Revision)
	if limits.ReadOnlyRepository || limits.DenyDeploy || !strings.Contains(launch.Revision, result) || !strings.Contains(launch.Revision, released) {
		t.Fatal("PM sent dev back to readonly audit", launch.Revision)
	}
	after, err := s.taskSvc.GetTaskDetail(ctx, d.ID)
	if err != nil || after.Status == task.TaskStatusDone || len(after.Attempts) != len(d.Attempts) {
		t.Fatal("handoff accepted work or rewrote history", err)
	}
	s.applyInboxReviewEvaluation(ctx, q, string(raw))
	repeated, err := s.taskSvc.GetLaunch(ctx, launch.ID)
	if err != nil || repeated.Revision != launch.Revision {
		t.Fatal("retry changed durable handoff", err)
	}
}
