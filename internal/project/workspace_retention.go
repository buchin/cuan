package project

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// WorkspaceRetentionPeriod returns the duration completed workspaces are retained.
// Defaults to 7 days, configurable via CUAN_WORKSPACE_TTL or CUAN_WORKSPACE_RETENTION_HOURS.
// Setting CUAN_WORKSPACE_RETENTION_HOURS=0 or CUAN_WORKSPACE_TTL=0 allows immediate retirement.
func WorkspaceRetentionPeriod() time.Duration {
	if val := os.Getenv("CUAN_WORKSPACE_TTL"); val != "" {
		if d, err := time.ParseDuration(val); err == nil && d >= 0 {
			return d
		}
		if h, err := strconv.Atoi(val); err == nil && h >= 0 {
			return time.Duration(h) * time.Hour
		}
	}
	if val := os.Getenv("CUAN_WORKSPACE_RETENTION_HOURS"); val != "" {
		if h, err := strconv.Atoi(val); err == nil && h >= 0 {
			return time.Duration(h) * time.Hour
		}
	}
	return 7 * 24 * time.Hour
}

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
	return inspectWorkspaceRetention(ctx, path, status, updatedAt, now, active, confirmedRemoteHead, false)
}

// InspectResolvedWorkspaceRetention permits an explicitly resolved conversation
// to discard its working checkout immediately, preserving the project repository.
func InspectResolvedWorkspaceRetention(ctx context.Context, path, status string, updatedAt, now time.Time, active bool, confirmedRemoteHead string) (WorkspaceRetentionDecision, error) {
	return inspectWorkspaceRetention(ctx, path, status, updatedAt, now, active, confirmedRemoteHead, true)
}

func inspectWorkspaceRetention(ctx context.Context, path, status string, updatedAt, now time.Time, active bool, confirmedRemoteHead string, resolved bool) (WorkspaceRetentionDecision, error) {
	keep := func(reason string) (WorkspaceRetentionDecision, error) {
		return WorkspaceRetentionDecision{Reason: reason}, nil
	}
	if active {
		return keep("execution_active")
	}
	isDone := status == "done"
	isCancelled := status == "cancelled" || status == "canceled"
	if !isDone && !isCancelled {
		return keep("task_not_done")
	}
	if isDone && !resolved && (updatedAt.IsZero() || now.Sub(updatedAt) < WorkspaceRetentionPeriod()) {
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
	// Explicit resolution discards this task's working copy, including local edits.
	// RetireExecutionWorkspace validates repository/branch identity and execution
	// ownership first. Chat attachments live in their separate durable store.
	if resolved {
		return WorkspaceRetentionDecision{Eligible: true, Reason: "conversation_resolved", Head: head}, nil
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
	if isDone && (confirmedRemoteHead == "" || confirmedRemoteHead != head) {
		return keep("remote_copy_unverified")
	}
	return WorkspaceRetentionDecision{Eligible: true, Reason: "clean_remote_copy_confirmed", Head: head}, nil
}
