package server

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/buchin/cuan/internal/project"
	"github.com/buchin/cuan/internal/runner"
	"github.com/buchin/cuan/internal/task"
)

func TestNativeArtifactReportUsesWorkerCheckoutWithoutProjectFallback(t *testing.T) {
	for _, scenario := range []string{"native output", "missing worker output", "missing workspace", "linked workspace", "linked repository", "inspection root", "local session", "foreign attempt"} {
		t.Run(scenario, func(t *testing.T) {
			ctx := context.Background()
			s, _, _, _ := inboxFixture(t)
			base := t.TempDir()
			s.cfg.WorkspacesDir = t.TempDir()
			projects := project.NewMemoryStore()
			if err := projects.CreateProject(ctx, &project.Project{ID: "artifact-project", Name: "Artifact fixture", LocalPath: base}); err != nil {
				t.Fatal(err)
			}
			s.projectSvc = project.NewService(projects, nil, "fixture", t.TempDir())
			store := task.NewMemoryStore()
			s.runnerSvc = runner.NewService(runner.NewMemoryStore(store), store)
			if _, err := s.runnerSvc.Heartbeat(ctx, runner.HeartbeatRequest{RunnerID: "artifact-native", Hostname: "fixture"}, time.Minute); err != nil {
				t.Fatal(err)
			}
			detail := &task.TaskDetailResponse{Task: task.Task{ID: "artifact-task", ProjectID: "artifact-project", ExecutionCategory: task.ExecutionCategoryCode}, LatestAttempt: &task.TaskAttempt{ID: "artifact-attempt", TaskID: "artifact-task", RunnerID: "artifact-native", Status: task.AttemptStatusRunning}}
			if err := os.WriteFile(filepath.Join(base, "report.txt"), []byte("OLD PROJECT OUTPUT"), 0600); err != nil {
				t.Fatal(err)
			}
			worker := filepath.Join(s.cfg.WorkspacesDir, detail.ID)
			if err := os.MkdirAll(filepath.Join(worker, ".git"), 0700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(worker, "report.txt"), []byte("CURRENT WORKER OUTPUT"), 0600); err != nil {
				t.Fatal(err)
			}
			root, err := os.OpenRoot(base)
			if err != nil {
				t.Fatal(err)
			}
			defer root.Close()
			grant := taskAgentGrant{TaskID: detail.ID, AttemptID: detail.LatestAttempt.ID, ArtifactRoot: root}
			switch scenario {
			case "missing worker output":
				os.Remove(filepath.Join(worker, "report.txt"))
			case "missing workspace":
				os.RemoveAll(worker)
			case "linked workspace":
				os.RemoveAll(worker)
				os.Symlink(base, worker)
			case "linked repository":
				os.Remove(filepath.Join(worker, ".git"))
				os.Symlink(base, filepath.Join(worker, ".git"))
			case "inspection root":
				isolated := t.TempDir()
				os.WriteFile(filepath.Join(isolated, "report.txt"), []byte("INSPECTION OUTPUT"), 0600)
				inspection, e := os.OpenRoot(isolated)
				if e != nil {
					t.Fatal(e)
				}
				defer inspection.Close()
				grant.ArtifactRoot = inspection
			case "local session":
				detail.LatestAttempt.RunnerID = "local_terminal"
			case "foreign attempt":
				grant.AttemptID = "another-attempt"
			}
			selected, closeRoot, err := s.taskReportArtifactRoot(ctx, grant, detail)
			if err != nil {
				if scenario != "missing workspace" && scenario != "linked workspace" && scenario != "linked repository" && scenario != "foreign attempt" {
					t.Fatal(err)
				}
				return
			}
			defer closeRoot()
			if scenario == "missing workspace" || scenario == "linked workspace" || scenario == "linked repository" || scenario == "foreign attempt" {
				t.Fatal("invalid native workspace accepted")
			}
			files, err := readTaskArtifacts(selected, detail.ID, grant.AttemptID, []string{"report.txt"})
			if scenario == "missing worker output" {
				if err == nil {
					t.Fatal("silently delivered stale project output")
				}
				return
			}
			if err != nil || len(files) != 1 {
				t.Fatal(err)
			}
			want := "CURRENT WORKER OUTPUT"
			if scenario == "inspection root" {
				want = "INSPECTION OUTPUT"
			}
			if scenario == "local session" {
				want = "OLD PROJECT OUTPUT"
			}
			if string(files[0].data) != want {
				t.Fatalf("wrong artifact source: %q", files[0].data)
			}
		})
	}
}

func TestNativeArtifactReportDeliversActualWorkerFileThroughAuthenticatedAPI(t *testing.T) {
	taskArtifactDeliveryContract(t, false, true)
}
