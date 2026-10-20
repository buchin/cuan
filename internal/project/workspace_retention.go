package project

import (
	"context"
	"errors"
	"os/exec"
	"strings"
	"time"
)

// WorkspaceRetentionDecision explains why a completed task can or cannot release
// its checkout. Callers must also hold execution ownership before any removal.
type WorkspaceRetentionDecision struct {
	Eligible bool   `json:"eligible"`
	Reason   string `json:"reason"`
	Head     string `json:"head,omitempty"`
}

// InspectWorkspaceRetention never fetches, deletes files, or assumes ignored
// files are disposable. A remote confirmation must be obtained separately.
func InspectWorkspaceRetention(ctx context.Context, path, status string, updatedAt, now time.Time, active bool, confirmedRemoteHead string) (WorkspaceRetentionDecision, error) {
	keep := func(reason string) (WorkspaceRetentionDecision, error) {
		return WorkspaceRetentionDecision{Reason: reason}, nil
	}
	if active {
		return keep("execution_active")
	}
	if status != "done" {
		return keep("task_not_done")
	}
	if updatedAt.IsZero() || now.Sub(updatedAt) < 7*24*time.Hour {
		return keep("retention_period")
	}
	run := func(args ...string) (string, error) {
		out, err := exec.CommandContext(ctx, "git", append([]string{"-C", path}, args...)...).Output()
		return strings.TrimSpace(string(out)), err
	}
	head, err := run("rev-parse", "--verify", "HEAD^{commit}")
	if err != nil {
		return WorkspaceRetentionDecision{}, errors.New("Could not verify workspace commit")
	}
	// Include ignored files: generated-looking files may contain the only copy
	// of user output, so they require a separately declared disposal policy.
	dirty, err := run("status", "--porcelain=v1", "--untracked-files=all", "--ignored=matching")
	if err != nil {
		return WorkspaceRetentionDecision{}, errors.New("Could not inspect workspace changes")
	}
	if dirty != "" {
		return keep("local_files_retained")
	}
	if confirmedRemoteHead == "" || confirmedRemoteHead != head {
		return keep("remote_copy_unverified")
	}
	return WorkspaceRetentionDecision{Eligible: true, Reason: "clean_remote_copy_confirmed", Head: head}, nil
}
