package runner

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/buchin/cuan/internal/project"
	"github.com/buchin/cuan/internal/task"
)

func TestRunnerClaimAndCompletionUseFrozenProjectContext(t *testing.T) {
	for _, originalDeploy := range []bool{false, true} {
		for _, codeChanges := range []bool{false, true} {
			t.Run(fmt.Sprintf("saved auto deploy %v changes %v", originalDeploy, codeChanges), func(t *testing.T) {
				ctx := context.Background()
				projects := project.NewMemoryStore()
				metadata, _ := json.Marshal(map[string]any{"cuan_settings": project.Settings{AutoDeploy: &originalDeploy, Environment: "DEPLOY_TARGET=saved\nCUAN_VERIFICATION_PHPUNIT_TARGETS=[\"tests/Feature/SavedTest.php\"]", DeployBranch: "saved-release"}})
				p := &project.Project{ID: "context-project", LocalPath: t.TempDir(), DefaultBranch: "main", Metadata: metadata}
				if err := projects.CreateProject(ctx, p); err != nil {
					t.Fatal(err)
				}
				projectSvc := project.NewService(projects, nil, "fixture", t.TempDir())
				original, _ := projectSvc.Get(ctx, p.ID)
				store := task.NewMemoryStore()
				tasks := task.NewService(store, nil)
				created, err := tasks.CreateTask(ctx, "owner", task.CreateTaskRequest{ProjectID: p.ID, Title: "Implement approved feature", Instructions: project.SnapshotProjectGuidance("Implement approved feature", original)})
				if err != nil {
					t.Fatal(err)
				}
				a, err := tasks.CreateAttempt(ctx, created.ID, "owner", task.CreateAttemptRequest{RunnerID: "context-runner", AgentName: "agent"})
				if err != nil {
					t.Fatal(err)
				}
				latestDeploy := !originalDeploy
				p.Metadata, _ = json.Marshal(map[string]any{"cuan_settings": project.Settings{AutoDeploy: &latestDeploy, Environment: "DEPLOY_TARGET=latest\nCUAN_VERIFICATION_PHPUNIT_TARGETS=[\"tests/Feature/LatestTest.php\"]", DeployBranch: "latest-release"}})
				if err := projects.UpdateProject(ctx, p); err != nil {
					t.Fatal(err)
				}
				svc := NewService(NewMemoryStore(store), store, projectSvc)
				claim, err := svc.Claim(ctx, ClaimTaskRequest{RunnerID: "context-runner"}, time.Minute)
				if err != nil || claim == nil || !claim.Claimed {
					t.Fatal("claim failed", err)
				}
				if claim.Environment != "DEPLOY_TARGET=saved\nCUAN_VERIFICATION_PHPUNIT_TARGETS=[\"tests/Feature/SavedTest.php\"]" || claim.DeployBranch != "saved-release" || claim.AutoDeploy != originalDeploy || claim.WorkingDir != original.LocalPath {
					t.Fatal("current defaults replaced saved context")
				}
				if claim.PHPUnitTargets != `["tests/Feature/SavedTest.php"]` {
					t.Fatal("ordinary worker lost frozen registered test targets", claim.PHPUnitTargets)
				}
				changed := codeChanges
				result, err := svc.Complete(ctx, a.ID, CompleteAttemptRequest{RunnerID: "context-runner", Status: task.AttemptStatusSucceeded, ExitCode: 0, AgentReportRequired: true, AgentReportConfirmed: true, CommitSHA: strings.Repeat("a", 40), HasCodeChanges: &changed, RequiresDeploy: &changed})
				if err != nil {
					t.Fatal(err)
				}
				if originalDeploy && changed && (result.Status != task.AttemptStatusFailed || !strings.Contains(result.ErrorMessage, "auto_deploy")) {
					t.Fatal("saved deployment requirement waived", result)
				}
				if (!originalDeploy || !changed) && result.Status != task.AttemptStatusSucceeded {
					t.Fatal("new defaults imposed deployment", result)
				}
				activities, err := store.ListActivities(ctx, created.ID)
				if err != nil {
					t.Fatal(err)
				}
				detail := &task.TaskDetailResponse{Task: *created, LatestAttempt: result, Activities: activities}
				required := task.AttemptPublicationRequired(detail)
				if result.Status == task.AttemptStatusSucceeded && (required == nil || *required != (originalDeploy && changed)) {
					t.Fatal("completion publication decision lost", required)
				}
			})
		}
	}
}

func TestInvalidSavedProjectContextStopsClaimBeforeProviderExecution(t *testing.T) {
	ctx := context.Background()
	projects := project.NewMemoryStore()
	projects.CreateProject(ctx, &project.Project{ID: "context-project", LocalPath: t.TempDir()})
	projectSvc := project.NewService(projects, nil, "fixture", t.TempDir())
	store := task.NewMemoryStore()
	tasks := task.NewService(store, nil)
	created, err := tasks.CreateTask(ctx, "owner", task.CreateTaskRequest{ProjectID: "context-project", Title: "Approved task", Instructions: "Approved work\n\nProject guidance snapshot:\n```cuan-project-context\ninvalid\n```"})
	if err != nil {
		t.Fatal(err)
	}
	a, err := tasks.CreateAttempt(ctx, created.ID, "owner", task.CreateAttemptRequest{RunnerID: "context-runner", AgentName: "agent"})
	if err != nil {
		t.Fatal(err)
	}
	svc := NewService(NewMemoryStore(store), store, projectSvc)
	if claim, err := svc.Claim(ctx, ClaimTaskRequest{RunnerID: "context-runner"}, time.Minute); err == nil || claim != nil {
		t.Fatal("invalid context executed", err)
	}
	finished, err := store.GetAttempt(ctx, a.ID)
	if err != nil || finished.Status != task.AttemptStatusFailed || !strings.Contains(finished.ErrorMessage, "no provider execution") {
		t.Fatal("failed claim left active work", err, finished)
	}
	// An older worker may already be running when its saved context becomes
	// unreadable. A successful process report must not waive publication checks.
	second, err := tasks.CreateTask(ctx, "owner", task.CreateTaskRequest{ProjectID: "context-project", Title: "Older running task", Instructions: created.Instructions})
	if err != nil {
		t.Fatal(err)
	}
	next, err := tasks.CreateAttempt(ctx, second.ID, "owner", task.CreateAttemptRequest{RunnerID: "context-runner", AgentName: "agent"})
	if err != nil {
		t.Fatal(err)
	}
	next.Status = task.AttemptStatusRunning
	if err := store.UpdateAttempt(ctx, next); err != nil {
		t.Fatal(err)
	}
	second.Status = task.TaskStatusDoing
	if err := store.UpdateTask(ctx, second); err != nil {
		t.Fatal(err)
	}

	completed, err := svc.Complete(ctx, next.ID, CompleteAttemptRequest{RunnerID: "context-runner", Status: task.AttemptStatusSucceeded, ExitCode: 0, AgentReportRequired: true, AgentReportConfirmed: true})
	if err != nil || completed.Status != task.AttemptStatusFailed || !strings.Contains(completed.ErrorMessage, "context could not be verified") {
		t.Fatal("unavailable context waived completion checks", err)
	}
}

func TestAccountOperationKeepsProjectAccessWithoutSourceTestsOrDeployment(t *testing.T) {
	for _, legacyCode := range []bool{false, true} {
		t.Run(fmt.Sprint(legacyCode), func(t *testing.T) {
			ctx := context.Background()
			projects := project.NewMemoryStore()
			deploy := true
			environment := "PROJECT_TEST_ACCESS_KEY=fixture\nCUAN_VERIFICATION_PHPUNIT_TARGETS=[\"tests/UnrelatedTest.php\"]"
			metadata, _ := json.Marshal(map[string]any{"cuan_settings": project.Settings{AutoDeploy: &deploy, Environment: environment}})
			p := &project.Project{ID: "account-project", LocalPath: t.TempDir(), DefaultBranch: "main", Metadata: metadata}
			if err := projects.CreateProject(ctx, p); err != nil {
				t.Fatal(err)
			}
			projectSvc := project.NewService(projects, nil, "fixture", t.TempDir())
			saved, _ := projectSvc.Get(ctx, p.ID)
			store := task.NewMemoryStore()
			tasks := task.NewService(store, nil)
			category := ""
			if legacyCode {
				category = task.ExecutionCategoryCode
			}
			item, err := tasks.CreateTask(ctx, "owner", task.CreateTaskRequest{ProjectID: p.ID, Title: "Aktivasi akses pelanggan", Instructions: project.SnapshotProjectGuidance("Aktifkan akses produk yang sudah ada untuk customer@example.invalid. Verifikasi entitlement aktif.", saved), ExecutionCategory: category})
			if err != nil {
				t.Fatal(err)
			}
			if !legacyCode && item.ExecutionCategory != task.ExecutionCategoryOps {
				t.Fatal("account operation became implementation")
			}
			if _, err := tasks.CreateAttempt(ctx, item.ID, "owner", task.CreateAttemptRequest{RunnerID: "worker", AgentName: "agent"}); err != nil {
				t.Fatal(err)
			}
			svc := NewService(NewMemoryStore(store), store, projectSvc)
			claim, err := svc.Claim(ctx, ClaimTaskRequest{RunnerID: "worker"}, time.Minute)
			if err != nil || !claim.Claimed {
				t.Fatal(err)
			}
			if claim.Environment != environment || claim.AutoDeploy || claim.PHPUnitTargets != "" || !claim.PublicationLimits.ReadOnlyRepository || !claim.PublicationLimits.DenyDeploy || !claim.PublicationLimits.DenyPush {
				t.Fatal("operation inherited source gates or lost original project access")
			}
		})
	}
}
