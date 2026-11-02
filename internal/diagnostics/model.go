package diagnostics

import "time"

// GitCommitEntry represents a single commit in the git log.
type GitCommitEntry struct {
	SHA     string `json:"sha"`
	Message string `json:"message"`
	Author  string `json:"author,omitempty"`
	Date    string `json:"date,omitempty"`
}

// GitStatusResult represents the structured read-only status of a repository.
type GitStatusResult struct {
	Branch         string   `json:"branch"`
	Upstream       string   `json:"upstream,omitempty"`
	Ahead          int      `json:"ahead,omitempty"`
	Behind         int      `json:"behind,omitempty"`
	Clean          bool     `json:"clean"`
	StagedCount    int      `json:"staged_count"`
	UnstagedCount  int      `json:"unstaged_count"`
	UntrackedCount int      `json:"untracked_count"`
	StagedFiles    []string `json:"staged_files,omitempty"`
	UnstagedFiles  []string `json:"unstaged_files,omitempty"`
	UntrackedFiles []string `json:"untracked_files,omitempty"`
	Raw            string   `json:"raw,omitempty"`
}

// GitLogResult represents the structured read-only log of commits.
type GitLogResult struct {
	Commits []GitCommitEntry `json:"commits"`
	Count   int              `json:"count"`
	Raw     string           `json:"raw,omitempty"`
}

// GitDiffResult represents a summary of changes (diff --stat).
type GitDiffResult struct {
	ChangedFiles int    `json:"changed_files"`
	Insertions   int    `json:"insertions"`
	Deletions    int    `json:"deletions"`
	Summary      string `json:"summary"`
	Raw          string `json:"raw,omitempty"`
}

// GitBranchesResult represents local and remote branches.
type GitBranchesResult struct {
	Current string   `json:"current"`
	Local   []string `json:"local"`
	Remote  []string `json:"remote,omitempty"`
	Raw     string   `json:"raw,omitempty"`
}

// DiskHealth represents disk filesystem capacity and availability.
type DiskHealth struct {
	TotalBytes  uint64  `json:"total_bytes"`
	FreeBytes   uint64  `json:"free_bytes"`
	UsedBytes   uint64  `json:"used_bytes"`
	UsedPercent float64 `json:"used_percent"`
	Path        string  `json:"path"`
	Warning     bool    `json:"warning"`
	Status      string  `json:"status"` // "HEALTHY" or "CRITICAL / LOW SPACE"
}

// MemoryHealth represents host memory usage.
type MemoryHealth struct {
	TotalBytes   uint64  `json:"total_bytes"`
	FreeBytes    uint64  `json:"free_bytes"`
	AvailBytes   uint64  `json:"avail_bytes"`
	UsedBytes    uint64  `json:"used_bytes"`
	UsedPercent  float64 `json:"used_percent"`
	Warning      bool    `json:"warning"`
	TotalMB      uint64  `json:"total_mb"`
	AvailableMB  uint64  `json:"available_mb"`
	UsedMB       uint64  `json:"used_mb"`
}

// SystemHealthResult represents comprehensive host health metrics.
type SystemHealthResult struct {
	Healthy       bool         `json:"healthy"`
	Disk          DiskHealth   `json:"disk"`
	Memory        MemoryHealth `json:"memory"`
	UptimeSeconds uint64       `json:"uptime_seconds"`
	LoadAverages  [3]float64   `json:"load_averages"`
	CheckedAt     time.Time    `json:"checked_at"`
}

// TypecheckResult represents the diagnostic result of static typechecking / linting.
type TypecheckResult struct {
	Success    bool     `json:"success"`
	Tool       string   `json:"tool"` // "tsc", "go vet", etc.
	IssueCount int      `json:"issue_count"`
	Issues     []string `json:"issues,omitempty"`
	DurationMs int64    `json:"duration_ms"`
	Raw        string   `json:"raw,omitempty"`
}

// DepsResult represents top-level dependency diagnostics.
type DepsResult struct {
	Tool       string   `json:"tool"` // "npm", "go list", etc.
	Success    bool     `json:"success"`
	Packages   []string `json:"packages,omitempty"`
	Count      int      `json:"count"`
	DurationMs int64    `json:"duration_ms"`
	Raw        string   `json:"raw,omitempty"`
}

// DiagnosticEnvelope wraps diagnostic results with execution metadata.
type DiagnosticEnvelope struct {
	Tool        string `json:"tool"`
	Success     bool   `json:"success"`
	DurationMs  int64  `json:"duration_ms"`
	Timestamp   string `json:"timestamp"`
	Data        any    `json:"data,omitempty"`
	Error       string `json:"error,omitempty"`
	Truncated   bool   `json:"truncated,omitempty"`
}

// EndpointHealthResult represents the HTTP endpoint probe result.
type EndpointHealthResult struct {
	URL            string         `json:"url"`
	StatusCode     int            `json:"status_code"`
	Status         string         `json:"status"`
	Healthy        bool           `json:"healthy"`
	LatencyMs      int64          `json:"latency_ms"`
	CommitSHA      string         `json:"commit_sha,omitempty"`
	App            string         `json:"app,omitempty"`
	Version        string         `json:"version,omitempty"`
	Timestamp      string         `json:"timestamp,omitempty"`
	ContentType    string         `json:"content_type,omitempty"`
	Server         string         `json:"server,omitempty"`
	Body           map[string]any `json:"body,omitempty"`
	RawBodyPreview string         `json:"raw_body_preview,omitempty"`
}

// DeploymentStatusResult represents the status of live deployment vs repository HEAD.
type DeploymentStatusResult struct {
	Project             string                `json:"project"`
	LiveCommitSHA       string                `json:"live_commit_sha,omitempty"`
	LiveShortSHA        string                `json:"live_short_sha,omitempty"`
	LiveVersion         string                `json:"live_version,omitempty"`
	LocalHeadSHA        string                `json:"local_head_sha,omitempty"`
	LocalShortSHA       string                `json:"local_short_sha,omitempty"`
	LocalBranch         string                `json:"local_branch,omitempty"`
	Synced              bool                  `json:"synced"`
	PendingCommitsCount int                   `json:"pending_commits_count"`
	PendingCommits      []string              `json:"pending_commits,omitempty"`
	ReleasePath         string                `json:"release_path,omitempty"`
	CurrentTarget       string                `json:"current_target,omitempty"`
	ServiceStatus       string                `json:"service_status,omitempty"`
	EndpointHealth      *EndpointHealthResult `json:"endpoint_health,omitempty"`
	CheckedAt           string                `json:"checked_at"`
}
