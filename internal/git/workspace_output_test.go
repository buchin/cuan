package git

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func webWorkspaceFixture(t *testing.T) (string, string, *Client) {
	t.Helper()
	base := filepath.Join(t.TempDir(), "source")
	createTestBaseRepo(t, base)
	if err := os.MkdirAll(filepath.Join(base, "web", "dist"), 0700); err != nil {
		t.Fatal(err)
	}
	for path, data := range map[string]string{".gitignore": "web/dist/\nweb/node_modules/\n", "web/package.json": "{}\n", "web/dist/index.html": "BASE_BUILD"} {
		if err := os.WriteFile(filepath.Join(base, path), []byte(data), 0600); err != nil {
			t.Fatal(err)
		}
	}
	for _, args := range [][]string{{"add", "."}, {"commit", "-m", "Web fixture"}} {
		if out, err := exec.Command("git", append([]string{"-C", base}, args...)...).CombinedOutput(); err != nil {
			t.Fatal(err, string(out))
		}
	}
	return base, t.TempDir(), NewClient()
}

func TestTaskBuildOutputsCannotOverwriteSourceOrAnotherTask(t *testing.T) {
	base, root, client := webWorkspaceFixture(t)
	var paths []string
	for _, id := range []string{"first", "second"} {
		ws, err := client.PrepareTaskWorkspace(context.Background(), WorkspaceConfig{TaskID: id, BaseRepoDir: base, WorkspacesBaseDir: root, DefaultBranch: "main"})
		if err != nil {
			t.Fatal(err)
		}
		paths = append(paths, ws.Path)
		if err := os.MkdirAll(filepath.Join(ws.Path, "web", "dist"), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(ws.Path, "web", "dist", "index.html"), []byte(id), 0600); err != nil {
			t.Fatal(err)
		}
	}
	for path, want := range map[string]string{filepath.Join(base, "web", "dist", "index.html"): "BASE_BUILD", filepath.Join(paths[0], "web", "dist", "index.html"): "first", filepath.Join(paths[1], "web", "dist", "index.html"): "second"} {
		if data, err := os.ReadFile(path); err != nil || string(data) != want {
			t.Fatal("build escaped its task workspace", path, string(data), err)
		}
	}
}

func TestRetainedManagedOutputLinkIsDetachedWithoutChangingTarget(t *testing.T) {
	base, root, client := webWorkspaceFixture(t)
	cfg := WorkspaceConfig{TaskID: "retained", BaseRepoDir: base, WorkspacesBaseDir: root, DefaultBranch: "main"}
	ws, err := client.PrepareTaskWorkspace(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	dist := filepath.Join(ws.Path, "web", "dist")
	if _, err := os.Lstat(dist); err == nil {
		if err := os.Remove(dist); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Symlink(filepath.Join(base, "web", "dist"), dist); err != nil {
		t.Fatal(err)
	}
	if _, err = client.PrepareTaskWorkspace(context.Background(), cfg); err != nil {
		t.Fatal(err)
	}
	if info, err := os.Lstat(dist); err == nil && info.Mode()&os.ModeSymlink != 0 {
		t.Fatal("retained task can still overwrite shared build")
	}
	if data, err := os.ReadFile(filepath.Join(base, "web", "dist", "index.html")); err != nil || string(data) != "BASE_BUILD" {
		t.Fatal("migration changed the shared target", err)
	}
}

func TestWorkspaceOutputPreservesUnmanagedAndTrackedLinks(t *testing.T) {
	for _, tracked := range []bool{false, true} {
		base, root, client := webWorkspaceFixture(t)
		cfg := WorkspaceConfig{TaskID: "saved-link", BaseRepoDir: base, WorkspacesBaseDir: root, DefaultBranch: "main"}
		ws, err := client.PrepareTaskWorkspace(context.Background(), cfg)
		if err != nil {
			t.Fatal(err)
		}
		target := t.TempDir()
		if tracked {
			target = filepath.Join(base, "web", "dist")
		}
		link := filepath.Join(ws.Path, "web", "dist")
		if err := os.Symlink(target, link); err != nil {
			t.Fatal(err)
		}
		if tracked {
			for _, args := range [][]string{{"add", "-f", "web/dist"}, {"commit", "-m", "Retained explicit output link"}} {
				if out, err := exec.Command("git", append([]string{"-C", ws.Path}, args...)...).CombinedOutput(); err != nil {
					t.Fatal(err, string(out))
				}
			}
		}
		if _, err := client.PrepareTaskWorkspace(context.Background(), cfg); err == nil {
			t.Fatal("unmanaged/tracked shared output was accepted")
		}
		if actual, err := os.Readlink(link); err != nil || actual != target {
			t.Fatal("saved user link was removed", err)
		}
	}
}
