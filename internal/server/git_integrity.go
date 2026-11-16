package server

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

type GitIntegrityReport struct {
	Branch           string   `json:"branch"`
	HeadSHA          string   `json:"head_sha"`
	ShortHeadSHA     string   `json:"short_head_sha"`
	StatusSummary    string   `json:"status_summary"`
	Upstream         string   `json:"upstream"`
	AheadCount       int      `json:"ahead_count"`
	BehindCount      int      `json:"behind_count"`
	SyncStatus       string   `json:"sync_status"`
	LiveReleaseSHA   string   `json:"live_release_sha"`
	ReleaseMatch     bool     `json:"release_match"`
	UnmergedBranches []string `json:"unmerged_branches"`
	Warnings         []string `json:"warnings"`
}

func (r GitIntegrityReport) Summary() string {
	var b strings.Builder
	fmt.Fprintf(&b, "Git Working State: branch=%s, status=%s\n", r.Branch, r.StatusSummary)
	b.WriteString("Git & Deployment Integrity:\n")
	fmt.Fprintf(&b, "- HEAD Commit: %s\n", r.ShortHeadSHA)

	if r.Upstream != "" {
		fmt.Fprintf(&b, "- Remote Upstream: %s (%s, ahead: %d, behind: %d)\n", r.Upstream, r.SyncStatus, r.AheadCount, r.BehindCount)
	} else {
		b.WriteString("- Remote Upstream: none configured\n")
	}

	if r.LiveReleaseSHA != "" && r.LiveReleaseSHA != "development" {
		matchStr := "matches HEAD"
		if !r.ReleaseMatch {
			matchStr = "MISMATCH with HEAD"
		}
		fmt.Fprintf(&b, "- Live VPS Release: %s [%s]\n", r.LiveReleaseSHA, matchStr)
	}

	if len(r.UnmergedBranches) > 0 {
		displayBranches := r.UnmergedBranches
		if len(displayBranches) > 5 {
			displayBranches = displayBranches[:5]
		}
		fmt.Fprintf(&b, "- Unmerged Task Branches (not in %s): %s\n", r.Branch, strings.Join(displayBranches, ", "))
	}

	for _, w := range r.Warnings {
		fmt.Fprintf(&b, "⚠️ %s\n", w)
	}

	return b.String()
}

func inspectGitIntegrity(ctx context.Context, dir string, liveReleaseSHA string) GitIntegrityReport {
	report := GitIntegrityReport{
		LiveReleaseSHA: liveReleaseSHA,
		ReleaseMatch:   true,
	}

	gitCmd := func(args ...string) (string, error) {
		cmdCtx, cmdCancel := context.WithTimeout(ctx, 2*time.Second)
		defer cmdCancel()
		cmd := exec.CommandContext(cmdCtx, "git", append([]string{"--no-pager", "-C", dir}, args...)...)
		cmd.Env = append(os.Environ(), "GIT_OPTIONAL_LOCKS=0", "GIT_TERMINAL_PROMPT=0")
		out, err := cmd.Output()
		return strings.TrimSpace(string(out)), err
	}

	// 1. Current Branch & Status
	branch, _ := gitCmd("branch", "--show-current")
	report.Branch = branch
	if report.Branch == "" {
		report.Branch = "detached"
	}

	statusSummary, _ := gitCmd("status", "--short")
	if statusSummary == "" {
		report.StatusSummary = "clean"
	} else {
		report.StatusSummary = truncateProjectText(statusSummary, 300)
	}

	// 2. Head SHA
	headSHA, _ := gitCmd("rev-parse", "HEAD")
	report.HeadSHA = headSHA
	shortHeadSHA, _ := gitCmd("rev-parse", "--short", "HEAD")
	report.ShortHeadSHA = shortHeadSHA

	// 3. Upstream & Ahead/Behind
	upstream, err := gitCmd("rev-parse", "--abbrev-ref", "@{upstream}")
	if err == nil && upstream != "" {
		report.Upstream = upstream
		counts, err := gitCmd("rev-list", "--left-right", "--count", "HEAD..."+upstream)
		if err == nil {
			fields := strings.Fields(counts)
			if len(fields) >= 2 {
				ahead, _ := strconv.Atoi(fields[0])
				behind, _ := strconv.Atoi(fields[1])
				report.AheadCount = ahead
				report.BehindCount = behind

				switch {
				case ahead == 0 && behind == 0:
					report.SyncStatus = "in sync"
				case ahead > 0 && behind == 0:
					report.SyncStatus = fmt.Sprintf("ahead by %d", ahead)
					report.Warnings = append(report.Warnings, fmt.Sprintf("GIT SYNC WARNING: Local %s is ahead of %s by %d commit(s). Push to origin required.", report.Branch, upstream, ahead))
				case ahead == 0 && behind > 0:
					report.SyncStatus = fmt.Sprintf("behind by %d", behind)
					report.Warnings = append(report.Warnings, fmt.Sprintf("GIT SYNC WARNING: Local %s is behind %s by %d commit(s). Pull from origin required.", report.Branch, upstream, behind))
				default:
					report.SyncStatus = fmt.Sprintf("diverged (ahead %d, behind %d)", ahead, behind)
					report.Warnings = append(report.Warnings, fmt.Sprintf("GIT INTEGRITY ALERT: Local %s and %s have diverged (ahead %d, behind %d). Reconciliation required.", report.Branch, upstream, ahead, behind))
				}
			}
		}
	} else {
		// Fallback check against remote tracking refs if upstream branch is not explicitly set
		for _, candidate := range []string{"origin/main", "origin/master"} {
			if _, e := gitCmd("rev-parse", "--verify", candidate+"^{commit}"); e == nil {
				report.Upstream = candidate
				counts, e := gitCmd("rev-list", "--left-right", "--count", "HEAD..."+candidate)
				if e == nil {
					fields := strings.Fields(counts)
					if len(fields) >= 2 {
						ahead, _ := strconv.Atoi(fields[0])
						behind, _ := strconv.Atoi(fields[1])
						report.AheadCount = ahead
						report.BehindCount = behind
						switch {
						case ahead == 0 && behind == 0:
							report.SyncStatus = "in sync"
						case ahead > 0 && behind == 0:
							report.SyncStatus = fmt.Sprintf("ahead by %d", ahead)
						case ahead == 0 && behind > 0:
							report.SyncStatus = fmt.Sprintf("behind by %d", behind)
						default:
							report.SyncStatus = fmt.Sprintf("diverged (ahead %d, behind %d)", ahead, behind)
						}
					}
				}
				break
			}
		}
		if report.SyncStatus == "" {
			report.SyncStatus = "no upstream"
		}
	}

	// 4. Live Release Check
	if liveReleaseSHA != "" && liveReleaseSHA != "development" && headSHA != "" {
		if !strings.HasPrefix(headSHA, liveReleaseSHA) && !strings.HasPrefix(liveReleaseSHA, headSHA) && !strings.HasPrefix(liveReleaseSHA, shortHeadSHA) {
			report.ReleaseMatch = false
			report.Warnings = append(report.Warnings, fmt.Sprintf("DEPLOYMENT INTEGRITY MISMATCH: Live release (%s) does not match workspace HEAD (%s). A build and deployment is needed to bring current code live.", liveReleaseSHA, shortHeadSHA))
		}
	}

	// 5. Unmerged Task Branches
	localUnmerged, _ := gitCmd("branch", "--no-merged", "HEAD")
	var unmerged []string
	if localUnmerged != "" {
		for _, line := range strings.Split(localUnmerged, "\n") {
			name := strings.TrimSpace(strings.TrimPrefix(line, "*"))
			if name != "" && name != report.Branch {
				unmerged = append(unmerged, name)
			}
		}
	}
	report.UnmergedBranches = unmerged
	if len(unmerged) > 0 {
		report.Warnings = append(report.Warnings, fmt.Sprintf("UNMERGED WORK DETECTED: %d local branch(es) not merged into %s. Deploying will not include these commits.", len(unmerged), report.Branch))
	}

	return report
}

// fetchAndInspectUpstream fetches remote commits non-destructively without altering
// working tree files, uncommitted edits, or local HEAD, and returns git integrity status.
func fetchAndInspectUpstream(ctx context.Context, dir string, liveReleaseSHA string) GitIntegrityReport {
	cmdCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(cmdCtx, "git", "-C", dir, "fetch", "--quiet")
	cmd.Env = append(os.Environ(), "GIT_OPTIONAL_LOCKS=0", "GIT_TERMINAL_PROMPT=0")
	_ = cmd.Run()

	return inspectGitIntegrity(ctx, dir, liveReleaseSHA)
}
