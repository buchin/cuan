package project

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"github.com/buchin/cuan/internal/crypto"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

type WorkspaceRetirementState struct {
	Status     string
	UpdatedAt  time.Time
	RemoteHead string
}
type workspaceReceipt struct{ TaskID, Source, Branch, Head, State string }

var ErrExecutionWorkspaceMissing = errors.New("Original task workspace is missing; its branch was retained for recovery")

func (s *Service) workspaceReceiptPath(key string) string {
	return filepath.Join(s.baseDir, "retired-workspaces", key+".json")
}
func (s *Service) saveWorkspaceReceipt(key string, receipt workspaceReceipt) error {
	raw, err := json.Marshal(receipt)
	if err != nil {
		return err
	}
	encrypted, err := crypto.Encrypt(string(raw), s.authSecret)
	if err != nil {
		return err
	}
	path := s.workspaceReceiptPath(key)
	if err = os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	file, err := os.CreateTemp(filepath.Dir(path), ".receipt-")
	if err != nil {
		return err
	}
	defer os.Remove(file.Name())
	_, err = file.WriteString(encrypted)
	if err == nil {
		err = file.Sync()
	}
	closeErr := file.Close()
	if err == nil {
		err = closeErr
	}
	if err == nil {
		err = os.Rename(file.Name(), path)
	}
	if err == nil {
		directory, openErr := os.Open(filepath.Dir(path))
		if openErr != nil {
			return openErr
		}
		err = directory.Sync()
		_ = directory.Close()
	}
	return err
}

// RetireExecutionWorkspace requires an exclusive task execution lease and a fresh
// task/remote observation under that lease. It never force-removes files or branches.
func (s *Service) RetireExecutionWorkspace(ctx context.Context, taskID string, p *ProjectResponse, now time.Time, acquire func() (io.Closer, error), observe func() (WorkspaceRetirementState, error)) (WorkspaceRetentionDecision, error) {
	if acquire == nil || observe == nil {
		return WorkspaceRetentionDecision{}, errors.New("Execution lease and current state are required")
	}
	lease, err := acquire()
	if err != nil {
		return WorkspaceRetentionDecision{}, err
	}
	if lease == nil {
		return WorkspaceRetentionDecision{}, errors.New("Execution lease is unavailable")
	}
	defer lease.Close()
	if p == nil || p.RepoURL == "" {
		return WorkspaceRetentionDecision{Reason: "not_a_repository_task"}, nil
	}
	source, err := filepath.EvalSymlinks(p.LocalPath)
	if err != nil {
		return WorkspaceRetentionDecision{}, err
	}
	digest := sha256.Sum256([]byte(taskID + "\x00" + source))
	key := hex.EncodeToString(digest[:])
	destination := filepath.Join(s.baseDir, "task-workspaces", key)
	if _, err = os.Lstat(destination); os.IsNotExist(err) {
		return WorkspaceRetentionDecision{Reason: "workspace_absent"}, nil
	} else if err != nil {
		return WorkspaceRetentionDecision{}, err
	}
	verified, err := s.ExecutionWorkspace(ctx, taskID, p)
	if err != nil {
		return WorkspaceRetentionDecision{}, err
	}
	executionWorkspaceMu.Lock()
	defer executionWorkspaceMu.Unlock()
	state, err := observe()
	if err != nil {
		return WorkspaceRetentionDecision{}, err
	}
	decision, err := InspectWorkspaceRetention(ctx, verified, state.Status, state.UpdatedAt, now, false, state.RemoteHead)
	if err != nil || !decision.Eligible {
		return decision, err
	}
	receipt := workspaceReceipt{taskID, source, "cuan/task-" + key, decision.Head, "pending"}
	if err = s.saveWorkspaceReceipt(key, receipt); err != nil {
		return decision, err
	}
	if exec.CommandContext(ctx, "git", "-C", source, "worktree", "remove", verified).Run() != nil {
		return WorkspaceRetentionDecision{}, errors.New("Workspace retained: Git refused removal")
	}
	receipt.State = "retired"
	if err = s.saveWorkspaceReceipt(key, receipt); err != nil {
		return WorkspaceRetentionDecision{}, errors.New("Workspace retired but recovery receipt needs repair")
	}
	return decision, nil
}
func (s *Service) restoreRetiredWorkspace(ctx context.Context, key, taskID, source, branch, destination string) error {
	encrypted, err := os.ReadFile(s.workspaceReceiptPath(key))
	if err != nil {
		if os.IsNotExist(err) {
			return ErrExecutionWorkspaceMissing
		}
		return errors.New("Workspace recovery receipt is unavailable")
	}
	raw, err := crypto.Decrypt(string(encrypted), s.authSecret)
	if err != nil {
		return errors.New("Workspace recovery receipt could not be verified")
	}
	var receipt workspaceReceipt
	if json.Unmarshal([]byte(raw), &receipt) != nil || receipt.TaskID != taskID || receipt.Source != source || receipt.Branch != branch || receipt.State != "retired" {
		return errors.New("Workspace retirement was not confirmed; retained work needs review")
	}
	out, err := exec.CommandContext(ctx, "git", "-C", source, "rev-parse", "refs/heads/"+branch).Output()
	if err != nil || strings.TrimSpace(string(out)) != receipt.Head {
		return errors.New("Retained task branch changed; recovery needs review")
	}
	if exec.CommandContext(ctx, "git", "-C", source, "worktree", "add", destination, branch).Run() != nil {
		return errors.New("Could not restore the retained task workspace")
	}
	// A retired receipt is single-use. A later external deletion must not be
	// mistaken for another deliberate cleanup and silently lose pending edits.
	if err := os.Remove(s.workspaceReceiptPath(key)); err != nil {
		return errors.New("Workspace restored but receipt cleanup needs review")
	}
	return nil
}
