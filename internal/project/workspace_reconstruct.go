package project

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/buchin/cuan/internal/crypto"
)

// Reconstruction never reuses the missing directory, resets its branch or
// prunes its Git registration. It is a new working copy, not proof that lost
// uncommitted files have been recovered.
type workspaceReconstruction struct{ TaskID, Source, OriginalBranch, Head, Branch string }

func writeRecoveryEvidence(path string, data []byte) error {
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return err
	}
	_, err = f.Write(data)
	if err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	dir, err := os.Open(filepath.Dir(path))
	if err != nil {
		return err
	}
	defer dir.Close()
	return dir.Sync()
}

func reconstructionIdentity(taskID string, p *ProjectResponse) (source, key string, err error) {
	if p == nil || p.RepoURL == "" || strings.TrimSpace(taskID) == "" {
		return "", "", errors.New("Repository task identity is required")
	}
	source, err = filepath.EvalSymlinks(p.LocalPath)
	if err != nil {
		return
	}
	digest := sha256.Sum256([]byte(taskID + "\x00" + source))
	key = hex.EncodeToString(digest[:])
	return
}
func (s *Service) reconstructionPaths(key string) (dir, record string) {
	dir, _ = filepath.Abs(filepath.Join(s.baseDir, "reconstructed-workspaces", key))
	return dir, filepath.Join(dir, "receipt")
}
func (s *Service) readReconstruction(taskID, source, key string) (*workspaceReconstruction, error) {
	_, record := s.reconstructionPaths(key)
	for _, path := range []string{filepath.Dir(record), record} {
		info, e := os.Lstat(path)
		if e != nil {
			return nil, e
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return nil, errors.New("Reconstruction evidence must not be a symbolic link")
		}
	}
	encrypted, err := os.ReadFile(record)
	if err != nil {
		return nil, err
	}
	raw, err := crypto.Decrypt(string(encrypted), s.authSecret)
	if err != nil {
		return nil, errors.New("Reconstructed workspace receipt could not be verified")
	}
	var r workspaceReconstruction
	if json.Unmarshal([]byte(raw), &r) != nil || r.TaskID != taskID || r.Source != source || r.OriginalBranch != "cuan/task-"+key || r.Branch != "cuan/recovered-"+key || r.Head == "" {
		return nil, errors.New("Reconstructed workspace identity changed")
	}
	return &r, nil
}
func (s *Service) reconstructedWorkspace(ctx context.Context, taskID, source, key string) (string, error) {
	r, err := s.readReconstruction(taskID, source, key)
	if err != nil {
		return "", err
	}
	dir, _ := s.reconstructionPaths(key)
	work := filepath.Join(dir, "worktree")
	info, err := os.Lstat(work)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return "", errors.New("Reconstructed workspace is missing or invalid; retained evidence needs review")
	}
	run := func(path string, args ...string) (string, error) {
		out, e := exec.CommandContext(ctx, "git", append([]string{"-C", path}, args...)...).Output()
		return strings.TrimSpace(string(out)), e
	}
	common, err := run(work, "rev-parse", "--path-format=absolute", "--git-common-dir")
	if err != nil {
		return "", errors.New("Reconstructed workspace Git state is unavailable")
	}
	expected, err := run(source, "rev-parse", "--path-format=absolute", "--git-common-dir")
	if err != nil || common != expected {
		return "", errors.New("Reconstructed workspace belongs to another repository")
	}
	branch, err := run(work, "symbolic-ref", "--short", "HEAD")
	if err != nil || branch != r.Branch {
		return "", errors.New("Reconstructed workspace branch changed; retained work needs review")
	}
	return work, nil
}

// ReconstructTaskExecutionWorkspace is only called for an authorized execution
// continuation after ordinary workspace resolution reports a missing worktree.
// Merely viewing a task cannot reconstruct it. The caller must still reserve the
// same task attempt before allowing an agent to execute.
func (s *Service) ReconstructTaskExecutionWorkspace(ctx context.Context, taskID string, p *ProjectResponse) (string, error) {
	source, key, err := reconstructionIdentity(taskID, p)
	if err != nil {
		return "", err
	}
	executionWorkspaceMu.Lock()
	defer executionWorkspaceMu.Unlock()
	dir, record := s.reconstructionPaths(key)
	if _, err = os.Lstat(record); err == nil {
		return s.reconstructedWorkspace(ctx, taskID, source, key)
	} else if !os.IsNotExist(err) {
		return "", err
	}
	original, _ := filepath.Abs(filepath.Join(s.baseDir, "task-workspaces", key))
	if _, err = os.Lstat(original); !os.IsNotExist(err) {
		return "", errors.New("Original workspace must remain untouched; inspect it before reconstruction")
	}
	// An existing evidence directory, even incomplete, is never overwritten.
	if err = os.MkdirAll(filepath.Dir(dir), 0700); err != nil {
		return "", err
	}
	if err = os.Mkdir(dir, 0700); err != nil {
		return "", errors.New("Workspace reconstruction evidence already exists or is unavailable")
	}
	branch := "cuan/task-" + key
	out, err := exec.CommandContext(ctx, "git", "-C", source, "rev-parse", "--verify", "refs/heads/"+branch+"^{commit}").Output()
	if err != nil {
		return "", errors.New("Retained task branch is unavailable")
	}
	r := workspaceReconstruction{taskID, source, branch, strings.TrimSpace(string(out)), "cuan/recovered-" + key}
	// Preserve any staged delta before an agent runs. Do not use checkout/reset on
	// the original index. A registered missing worktree may still hold staged work.
	common, err := exec.CommandContext(ctx, "git", "-C", source, "rev-parse", "--path-format=absolute", "--git-common-dir").Output()
	if err != nil {
		return "", err
	}
	registrations, _ := os.ReadDir(filepath.Join(strings.TrimSpace(string(common)), "worktrees"))
	for _, entry := range registrations {
		if !entry.IsDir() {
			continue
		}
		meta := filepath.Join(strings.TrimSpace(string(common)), "worktrees", entry.Name())
		head, e := os.ReadFile(filepath.Join(meta, "HEAD"))
		if e != nil || strings.TrimSpace(string(head)) != "ref: refs/heads/"+branch {
			continue
		}
		index, e := os.ReadFile(filepath.Join(meta, "index"))
		if e != nil && !os.IsNotExist(e) {
			return "", errors.New("Retained task index could not be preserved")
		}
		if e == nil {
			if e = writeRecoveryEvidence(filepath.Join(dir, "original-index"), index); e != nil {
				return "", e
			}
		}
		patch, e := exec.CommandContext(ctx, "git", "--git-dir="+meta, "--work-tree="+original, "diff", "--cached", "--binary", r.Head, "--").Output()
		if e != nil {
			return "", errors.New("Retained staged work could not be preserved")
		}
		if e = writeRecoveryEvidence(filepath.Join(dir, "staged.patch"), patch); e != nil {
			return "", e
		}
	}
	raw, _ := json.Marshal(r)
	encrypted, err := crypto.Encrypt(string(raw), s.authSecret)
	if err != nil {
		return "", err
	}
	// Persist the identity before creating the branch. A partial reconstruction
	// must fail closed rather than silently creating a second replacement.
	if err = writeRecoveryEvidence(record, []byte(encrypted)); err != nil {
		return "", err
	}
	work := filepath.Join(dir, "worktree")
	if err = exec.CommandContext(ctx, "git", "-C", source, "worktree", "add", "-b", r.Branch, work, r.Head).Run(); err != nil {
		return "", errors.New("Could not reconstruct the retained committed tree; recovery evidence was preserved")
	}
	s.configureWorkspaceDeployBranch(ctx, work, p)
	if err = syncWorkspaceEnvironment(work, SettingsFromMetadata(p.Metadata).Environment); err != nil {
		return "", err
	}
	return work, nil
}

func WorkspaceReconstructionContext(work string) string {
	if filepath.Base(filepath.Dir(work)) == "" || filepath.Base(work) != "worktree" || filepath.Base(filepath.Dir(filepath.Dir(work))) != "reconstructed-workspaces" {
		return ""
	}
	return "\n## Workspace reconstruction\nThe original task working directory was missing. This new working copy starts from its retained committed branch. The original branch and Git registration were not reset, deleted or pruned. Unstaged and untracked files are NOT proven recovered. Previous summaries/checkmarks are historical claims, not current acceptance evidence. Inspect the preserved staged.patch and original-index in the parent evidence directory when present; recover relevant changes deliberately. Recheck every original acceptance criterion and report any missing artifact honestly. Never report lost files as recovered without inspecting them.\n"
}
