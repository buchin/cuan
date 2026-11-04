package diagnostics

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

const (
	DefaultTimeout   = 5 * time.Second
	TypecheckTimeout = 10 * time.Second
	MaxOutputBytes   = 128 * 1024 // 128 KB bounded output
	MaxLogLimit      = 100
	DefaultLogLimit  = 10
)

var (
	ErrForbiddenCharacters   = errors.New("forbidden characters or shell control operators in argument")
	ErrDisallowedFlag        = errors.New("mutation or non-whitelisted flag rejected")
	ErrNonWhitelistedCommand = errors.New("non-whitelisted inspection command rejected")
	ErrInvalidDirectory      = errors.New("target directory does not exist or is not a directory")
	ErrCommandTimedOut       = errors.New("inspection command timed out")
)

// Allowed subcommands for git repository inspection
var allowedRepoSubcommands = map[string]bool{
	"status":       true,
	"log":          true,
	"diff-summary": true,
	"branches":     true,
	"df":           true,
	"free":         true,
	"tsc":          true,
	"typecheck":    true,
	"health":       true,
	"deps":         true,
}

// Allowed subcommands for diagnostics / system health
var allowedDiagSubcommands = map[string]bool{
	"health":       true,
	"df":           true,
	"free":         true,
	"tsc":          true,
	"typecheck":    true,
	"deps":         true,
	"status":       true,
	"log":          true,
	"diff-summary": true,
	"branches":     true,
}

// Dangerous shell control characters and metacharacters
var forbiddenCharsPattern = regexp.MustCompile(`[;&|><$` + "`" + `\n\r\x00]`)

// Mutation flags that must be strictly rejected
var forbiddenFlags = []string{
	"--exec", "-o", "--output", "--delete", "-d", "-D", "--force", "-f",
	"--hard", "--mixed", "--soft", "--amend", "--write", "--apply",
	"--patch", "--interactive", "-i",
}

// ValidateSafeArgs verifies that a subcommand and extra arguments are strictly read-only and safe.
func ValidateSafeArgs(subcommand string, extraArgs []string) error {
	sub := strings.ToLower(strings.TrimSpace(subcommand))
	if !allowedRepoSubcommands[sub] && !allowedDiagSubcommands[sub] {
		return fmt.Errorf("%w: %s", ErrNonWhitelistedCommand, subcommand)
	}

	for _, arg := range extraArgs {
		if forbiddenCharsPattern.MatchString(arg) {
			return fmt.Errorf("%w: %q", ErrForbiddenCharacters, arg)
		}
		for _, f := range forbiddenFlags {
			if arg == f || strings.HasPrefix(arg, f+"=") {
				return fmt.Errorf("%w: %q", ErrDisallowedFlag, arg)
			}
		}
		// Reject git mutation subcommands passed as flags/args
		lower := strings.ToLower(arg)
		for _, mut := range []string{"commit", "checkout", "switch", "push", "pull", "merge", "rebase", "reset", "clean", "rm", "mv"} {
			if lower == mut {
				return fmt.Errorf("%w: %q", ErrDisallowedFlag, arg)
			}
		}
	}
	return nil
}

// boundedDiagnosticOutput drains all output but retains only a fixed-size prefix.
// Stopping the pipe reader at the limit can deadlock a still-writing command.
type boundedDiagnosticOutput struct {
	mu        sync.Mutex
	buffer    bytes.Buffer
	truncated bool
}

func (b *boundedDiagnosticOutput) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	count := len(p)
	remaining := MaxOutputBytes - b.buffer.Len()
	if len(p) > remaining {
		p = p[:remaining]
		b.truncated = true
	}
	_, _ = b.buffer.Write(p)
	return count, nil
}

// runBoundedCommand bounds memory, process time and waiting for inherited pipes.
func runBoundedCommand(ctx context.Context, timeout time.Duration, dir, name string, args ...string) ([]byte, bool, error) {
	if timeout <= 0 {
		timeout = DefaultTimeout
	}
	cmdCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	cmd := exec.CommandContext(cmdCtx, name, args...)
	cmd.Dir = dir
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error {
		if cmd.Process == nil {
			return nil
		}
		err := syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		if err == syscall.ESRCH {
			return os.ErrProcessDone
		}
		return err
	}
	cmd.WaitDelay = 200 * time.Millisecond
	var output boundedDiagnosticOutput
	cmd.Stdout = &output
	cmd.Stderr = &output
	err := cmd.Run()
	if errors.Is(err, exec.ErrWaitDelay) && cmd.Process != nil {
		// An exited leader can leave a same-group child holding its output pipe.
		_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	}
	if errors.Is(cmdCtx.Err(), context.DeadlineExceeded) {
		err = ErrCommandTimedOut
	} else if cmdCtx.Err() != nil {
		err = cmdCtx.Err()
	}
	output.mu.Lock()
	defer output.mu.Unlock()
	return append([]byte(nil), output.buffer.Bytes()...), output.truncated, err
}

// sanitizeRepoDir checks and resolves the repository directory path.
func sanitizeRepoDir(dir string) (string, error) {
	if dir == "" {
		var err error
		dir, err = os.Getwd()
		if err != nil {
			return "", err
		}
	}
	abs, err := filepath.Abs(dir)
	if err != nil {
		return "", err
	}
	fi, err := os.Stat(abs)
	if err != nil || !fi.IsDir() {
		return "", fmt.Errorf("%w: %s", ErrInvalidDirectory, dir)
	}
	return abs, nil
}

// InspectGitStatus inspects the working tree and branch status in a strictly read-only manner.
func InspectGitStatus(ctx context.Context, dir string) (*GitStatusResult, error) {
	dir, err := sanitizeRepoDir(dir)
	if err != nil {
		return nil, err
	}

	out, _, err := runBoundedCommand(ctx, DefaultTimeout, dir, "git", "status", "--porcelain=v1", "-b")
	if err != nil && !errors.Is(err, ErrCommandTimedOut) {
		// If git returned non-zero (e.g. not a git repo)
		return nil, fmt.Errorf("git status error: %v (output: %s)", err, string(out))
	}

	raw := string(out)
	result := &GitStatusResult{
		Clean: true,
		Raw:   raw,
	}

	scanner := bufio.NewScanner(strings.NewReader(raw))
	for scanner.Scan() {
		line := scanner.Text()
		if len(line) == 0 {
			continue
		}

		if strings.HasPrefix(line, "## ") {
			header := strings.TrimPrefix(line, "## ")
			parseBranchHeader(header, result)
			continue
		}

		result.Clean = false
		if strings.HasPrefix(line, "?? ") {
			result.UntrackedCount++
			file := strings.TrimPrefix(line, "?? ")
			result.UntrackedFiles = append(result.UntrackedFiles, file)
			continue
		}

		if len(line) >= 2 {
			stagedChar := line[0]
			unstagedChar := line[1]
			file := strings.TrimSpace(line[2:])

			if stagedChar != ' ' && stagedChar != '?' {
				result.StagedCount++
				result.StagedFiles = append(result.StagedFiles, file)
			}
			if unstagedChar != ' ' && unstagedChar != '?' {
				result.UnstagedCount++
				result.UnstagedFiles = append(result.UnstagedFiles, file)
			}
		}
	}

	return result, nil
}

func parseBranchHeader(header string, result *GitStatusResult) {
	// Format: branch...upstream [ahead N, behind M]
	// Or: HEAD (no branch)
	if strings.Contains(header, "no branch") {
		result.Branch = "HEAD (detached)"
		return
	}

	parts := strings.Split(header, "...")
	if len(parts) == 1 {
		result.Branch = strings.Fields(parts[0])[0]
		return
	}

	result.Branch = strings.TrimSpace(parts[0])
	rest := parts[1]

	// Extract upstream and ahead/behind counts
	if idx := strings.Index(rest, " ["); idx != -1 {
		result.Upstream = strings.TrimSpace(rest[:idx])
		bracket := rest[idx+2:]
		bracket = strings.TrimSuffix(bracket, "]")
		for _, token := range strings.Split(bracket, ", ") {
			if strings.HasPrefix(token, "ahead ") {
				result.Ahead, _ = strconv.Atoi(strings.TrimPrefix(token, "ahead "))
			} else if strings.HasPrefix(token, "behind ") {
				result.Behind, _ = strconv.Atoi(strings.TrimPrefix(token, "behind "))
			}
		}
	} else {
		result.Upstream = strings.TrimSpace(rest)
	}
}

// InspectGitLog retrieves the commit history bounded to limit entries.
func InspectGitLog(ctx context.Context, dir string, limit int) (*GitLogResult, error) {
	dir, err := sanitizeRepoDir(dir)
	if err != nil {
		return nil, err
	}
	if limit <= 0 {
		limit = DefaultLogLimit
	}
	if limit > MaxLogLimit {
		limit = MaxLogLimit
	}

	limitArg := fmt.Sprintf("-n%d", limit)
	out, _, err := runBoundedCommand(ctx, DefaultTimeout, dir, "git", "log", limitArg, "--format=%h|%an|%ar|%s")
	if err != nil {
		return nil, fmt.Errorf("git log error: %v (output: %s)", err, string(out))
	}

	raw := string(out)
	result := &GitLogResult{
		Commits: make([]GitCommitEntry, 0),
		Raw:     raw,
	}

	scanner := bufio.NewScanner(strings.NewReader(raw))
	for scanner.Scan() {
		line := scanner.Text()
		if strings.TrimSpace(line) == "" {
			continue
		}
		parts := strings.SplitN(line, "|", 4)
		if len(parts) >= 4 {
			result.Commits = append(result.Commits, GitCommitEntry{
				SHA:     parts[0],
				Author:  parts[1],
				Date:    parts[2],
				Message: parts[3],
			})
		} else if len(parts) > 0 {
			result.Commits = append(result.Commits, GitCommitEntry{
				SHA:     parts[0],
				Message: line,
			})
		}
	}
	result.Count = len(result.Commits)
	return result, nil
}

// InspectGitDiffSummary retrieves diff stat summary without dumping full file diffs.
func InspectGitDiffSummary(ctx context.Context, dir string) (*GitDiffResult, error) {
	dir, err := sanitizeRepoDir(dir)
	if err != nil {
		return nil, err
	}

	out, _, err := runBoundedCommand(ctx, DefaultTimeout, dir, "git", "diff", "--stat")
	if err != nil {
		return nil, fmt.Errorf("git diff error: %v (output: %s)", err, string(out))
	}

	raw := string(out)
	// If unstaged diff is empty, also check staged diff
	if strings.TrimSpace(raw) == "" {
		stagedOut, _, stagedErr := runBoundedCommand(ctx, DefaultTimeout, dir, "git", "diff", "--cached", "--stat")
		if stagedErr == nil && len(stagedOut) > 0 {
			raw = string(stagedOut)
		}
	}

	result := &GitDiffResult{
		Raw: raw,
	}

	// Parse summary line, e.g. " 3 files changed, 25 insertions(+), 4 deletions(-)"
	lines := strings.Split(strings.TrimSpace(raw), "\n")
	if len(lines) > 0 {
		lastLine := strings.TrimSpace(lines[len(lines)-1])
		if strings.Contains(lastLine, "changed") {
			result.Summary = lastLine
			parseDiffStatSummary(lastLine, result)
		}
	}
	return result, nil
}

func parseDiffStatSummary(summary string, result *GitDiffResult) {
	parts := strings.Split(summary, ",")
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if strings.Contains(part, "file") {
			fields := strings.Fields(part)
			if len(fields) > 0 {
				result.ChangedFiles, _ = strconv.Atoi(fields[0])
			}
		} else if strings.Contains(part, "insertion") {
			fields := strings.Fields(part)
			if len(fields) > 0 {
				result.Insertions, _ = strconv.Atoi(fields[0])
			}
		} else if strings.Contains(part, "deletion") {
			fields := strings.Fields(part)
			if len(fields) > 0 {
				result.Deletions, _ = strconv.Atoi(fields[0])
			}
		}
	}
}

// InspectGitBranches lists all branches and active branch in a repository.
func InspectGitBranches(ctx context.Context, dir string) (*GitBranchesResult, error) {
	dir, err := sanitizeRepoDir(dir)
	if err != nil {
		return nil, err
	}

	out, _, err := runBoundedCommand(ctx, DefaultTimeout, dir, "git", "branch", "-a", "--format=%(HEAD)|%(refname:short)|%(upstream:short)")
	if err != nil {
		return nil, fmt.Errorf("git branch error: %v (output: %s)", err, string(out))
	}

	raw := string(out)
	result := &GitBranchesResult{
		Local:  make([]string, 0),
		Remote: make([]string, 0),
		Raw:    raw,
	}

	scanner := bufio.NewScanner(strings.NewReader(raw))
	for scanner.Scan() {
		line := scanner.Text()
		if strings.TrimSpace(line) == "" {
			continue
		}
		parts := strings.Split(line, "|")
		isHead := len(parts) > 0 && strings.TrimSpace(parts[0]) == "*"
		branchName := ""
		if len(parts) > 1 {
			branchName = strings.TrimSpace(parts[1])
		}

		if isHead {
			result.Current = branchName
		}
		if strings.HasPrefix(branchName, "origin/") || strings.HasPrefix(branchName, "remotes/") {
			result.Remote = append(result.Remote, branchName)
		} else if branchName != "" {
			result.Local = append(result.Local, branchName)
		}
	}
	return result, nil
}

// InspectSystemHealth provides a fast, non-blocking check of host disk, memory, uptime, and load.
func InspectSystemHealth(ctx context.Context, targetPath string) (*SystemHealthResult, error) {
	if targetPath == "" {
		targetPath = "/"
	}

	res := &SystemHealthResult{
		Healthy:   true,
		CheckedAt: time.Now().UTC(),
	}

	// 1. Disk inspection (statfs)
	var stat syscall.Statfs_t
	if err := syscall.Statfs(targetPath, &stat); err != nil && targetPath != "/" {
		targetPath = "/"
		_ = syscall.Statfs(targetPath, &stat)
	}

	res.Disk.Path = targetPath
	res.Disk.TotalBytes = stat.Blocks * uint64(stat.Bsize)
	res.Disk.FreeBytes = stat.Bavail * uint64(stat.Bsize)
	if res.Disk.TotalBytes > res.Disk.FreeBytes {
		res.Disk.UsedBytes = res.Disk.TotalBytes - res.Disk.FreeBytes
	}
	if res.Disk.TotalBytes > 0 {
		res.Disk.UsedPercent = float64(res.Disk.UsedBytes) / float64(res.Disk.TotalBytes) * 100.0
	}
	res.Disk.Status = "HEALTHY"
	if res.Disk.UsedPercent >= 85.0 || res.Disk.FreeBytes < (2*1024*1024*1024) {
		res.Disk.Warning = true
		res.Disk.Status = "CRITICAL / LOW SPACE"
		res.Healthy = false
	}

	// 2. Memory inspection via /proc/meminfo or sysinfo
	inspectMemoryLinux(&res.Memory)
	if res.Memory.UsedPercent >= 90.0 {
		res.Memory.Warning = true
		res.Healthy = false
	}

	// 3. Sysinfo for uptime and load average
	var si syscall.Sysinfo_t
	if err := syscall.Sysinfo(&si); err == nil {
		res.UptimeSeconds = uint64(si.Uptime)
		res.LoadAverages[0] = float64(si.Loads[0]) / 65536.0
		res.LoadAverages[1] = float64(si.Loads[1]) / 65536.0
		res.LoadAverages[2] = float64(si.Loads[2]) / 65536.0
	}

	return res, nil
}

func inspectMemoryLinux(m *MemoryHealth) {
	f, err := os.Open("/proc/meminfo")
	if err != nil {
		return
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		key := strings.TrimSuffix(fields[0], ":")
		val, _ := strconv.ParseUint(fields[1], 10, 64)
		bytesVal := val * 1024

		switch key {
		case "MemTotal":
			m.TotalBytes = bytesVal
			m.TotalMB = val / 1024
		case "MemAvailable":
			m.AvailBytes = bytesVal
			m.AvailableMB = val / 1024
		case "MemFree":
			m.FreeBytes = bytesVal
		}
	}

	if m.TotalBytes > m.AvailBytes {
		m.UsedBytes = m.TotalBytes - m.AvailBytes
		m.UsedMB = m.UsedBytes / (1024 * 1024)
	}
	if m.TotalBytes > 0 {
		m.UsedPercent = float64(m.UsedBytes) / float64(m.TotalBytes) * 100.0
	}
}

// InspectTypecheck runs static typechecking (tsc --noEmit or go vet) bounded to TypecheckTimeout.
func InspectTypecheck(ctx context.Context, dir string) (*TypecheckResult, error) {
	dir, err := sanitizeRepoDir(dir)
	if err != nil {
		return nil, err
	}

	start := time.Now()
	res := &TypecheckResult{
		Status:    TypecheckNotRun,
		Directory: dir,
		Issues:    make([]string, 0),
	}

	// Detect if TypeScript project (look for tsconfig.json or web/tsconfig.json)
	tsDir := ""
	if _, err := os.Stat(filepath.Join(dir, "tsconfig.json")); err == nil {
		tsDir = dir
	} else if _, err := os.Stat(filepath.Join(dir, "web", "tsconfig.json")); err == nil {
		tsDir = filepath.Join(dir, "web")
	}

	if tsDir != "" {
		res.Tool = "tsc --noEmit"
		localTSC := filepath.Join(tsDir, "node_modules", ".bin", "tsc")
		if _, err := os.Stat(localTSC); os.IsNotExist(err) {
			localTSC = filepath.Join(dir, "web", "node_modules", ".bin", "tsc")
		}

		cmdName := ""
		var cmdArgs []string

		if _, err := os.Stat(localTSC); err == nil {
			cmdName = localTSC
			cmdArgs = []string{"--noEmit"}
		} else if _, err := exec.LookPath("tsc"); err == nil {
			cmdName = "tsc"
			cmdArgs = []string{"--noEmit"}
		} else {
			cmdName = "npx"
			cmdArgs = []string{"--no-install", "tsc", "--noEmit"}
		}

		res.Directory = tsDir
		out, truncated, err := runBoundedCommand(ctx, TypecheckTimeout, tsDir, cmdName, cmdArgs...)
		res.DurationMs = time.Since(start).Milliseconds()
		res.Raw = string(out)
		setTypecheckOutcome(res, err, truncated)

		scanner := bufio.NewScanner(strings.NewReader(res.Raw))
		for scanner.Scan() {
			line := scanner.Text()
			if strings.Contains(line, "error TS") {
				res.Issues = append(res.Issues, line)
			}
		}
		res.IssueCount = len(res.Issues)
		return res, nil
	}

	// If Go project (go.mod exists)
	if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
		res.Tool = "go vet ./..."
		out, truncated, err := runBoundedCommand(ctx, TypecheckTimeout, dir, "go", "vet", "./...")
		res.DurationMs = time.Since(start).Milliseconds()
		res.Raw = string(out)
		setTypecheckOutcome(res, err, truncated)

		scanner := bufio.NewScanner(strings.NewReader(res.Raw))
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line != "" {
				res.Issues = append(res.Issues, line)
			}
		}
		res.IssueCount = len(res.Issues)
		return res, nil
	}

	// Neither TypeScript nor Go detected
	res.Tool = "none"
	res.Reason = "No TypeScript or Go configuration detected; no typecheck was run."
	res.Raw = res.Reason
	return res, nil
}

func setTypecheckOutcome(result *TypecheckResult, err error, truncated bool) {
	result.Truncated = truncated
	result.Success = err == nil
	if err == nil {
		result.Status = TypecheckPassed
		return
	}
	result.Reason = err.Error()
	var exit *exec.ExitError
	switch {
	case errors.Is(err, ErrCommandTimedOut):
		result.Status = TypecheckTimedOut
	case errors.Is(err, context.Canceled):
		result.Status = TypecheckCanceled
	case errors.Is(err, exec.ErrWaitDelay):
		result.Status = TypecheckFailed
	case errors.As(err, &exit):
		result.Status = TypecheckFailed
	default:
		result.Status = TypecheckNotRun
	}
}

// InspectDependencies inspects top-level project dependencies without network calls.
func InspectDependencies(ctx context.Context, dir string) (*DepsResult, error) {
	dir, err := sanitizeRepoDir(dir)
	if err != nil {
		return nil, err
	}

	start := time.Now()
	res := &DepsResult{
		Packages: make([]string, 0),
	}

	// Check package.json in dir or dir/web
	pkgPath := filepath.Join(dir, "package.json")
	if _, err := os.Stat(pkgPath); os.IsNotExist(err) {
		pkgPath = filepath.Join(dir, "web", "package.json")
	}

	if data, err := os.ReadFile(pkgPath); err == nil {
		res.Tool = "package.json"
		var pkg struct {
			Dependencies    map[string]string `json:"dependencies"`
			DevDependencies map[string]string `json:"devDependencies"`
		}
		if json.Unmarshal(data, &pkg) == nil {
			for name, ver := range pkg.Dependencies {
				res.Packages = append(res.Packages, fmt.Sprintf("%s@%s", name, ver))
			}
			for name, ver := range pkg.DevDependencies {
				res.Packages = append(res.Packages, fmt.Sprintf("%s@%s (dev)", name, ver))
			}
			res.Success = true
			res.Count = len(res.Packages)
			res.DurationMs = time.Since(start).Milliseconds()
			return res, nil
		}
	}

	// Check go.mod
	goModPath := filepath.Join(dir, "go.mod")
	if data, err := os.ReadFile(goModPath); err == nil {
		res.Tool = "go.mod"
		scanner := bufio.NewScanner(bytes.NewReader(data))
		inRequire := false
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line == "require (" {
				inRequire = true
				continue
			}
			if inRequire && line == ")" {
				inRequire = false
				continue
			}
			if inRequire && line != "" && !strings.HasPrefix(line, "//") {
				fields := strings.Fields(line)
				if len(fields) >= 2 {
					res.Packages = append(res.Packages, fmt.Sprintf("%s %s", fields[0], fields[1]))
				}
			} else if strings.HasPrefix(line, "require ") {
				fields := strings.Fields(line)
				if len(fields) >= 3 {
					res.Packages = append(res.Packages, fmt.Sprintf("%s %s", fields[1], fields[2]))
				}
			}
		}
		res.Success = true
		res.Count = len(res.Packages)
		res.DurationMs = time.Since(start).Milliseconds()
		return res, nil
	}

	res.Tool = "none"
	res.Success = true
	res.Raw = "No package.json or go.mod found"
	return res, nil
}

// ProbeEndpointHealth performs a safe, bounded HTTP GET request against a target URL.
func ProbeEndpointHealth(ctx context.Context, rawURL string, timeoutSecs int) (*EndpointHealthResult, error) {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return nil, errors.New("url is required")
	}

	parsed, err := url.Parse(rawURL)
	if err != nil {
		return nil, fmt.Errorf("invalid url: %w", err)
	}
	scheme := strings.ToLower(parsed.Scheme)
	if scheme != "http" && scheme != "https" {
		return nil, fmt.Errorf("unsupported url scheme %q: only http and https are allowed", parsed.Scheme)
	}

	if timeoutSecs <= 0 {
		timeoutSecs = 5
	}
	if timeoutSecs > 15 {
		timeoutSecs = 15
	}

	reqCtx, cancel := context.WithTimeout(ctx, time.Duration(timeoutSecs)*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("User-Agent", "Cuan-Diagnostics-MCP/1.0")
	req.Header.Set("Accept", "application/json, text/plain, */*")

	client := &http.Client{
		Timeout: time.Duration(timeoutSecs) * time.Second,
	}

	start := time.Now()
	resp, err := client.Do(req)
	latencyMs := time.Since(start).Milliseconds()

	if err != nil {
		return &EndpointHealthResult{
			URL:            rawURL,
			StatusCode:     0,
			Status:         err.Error(),
			Healthy:        false,
			LatencyMs:      latencyMs,
			RawBodyPreview: fmt.Sprintf("Request failed: %v", err),
		}, nil
	}
	defer resp.Body.Close()

	bodyReader := io.LimitReader(resp.Body, 64*1024)
	bodyBytes, _ := io.ReadAll(bodyReader)

	result := &EndpointHealthResult{
		URL:         rawURL,
		StatusCode:  resp.StatusCode,
		Status:      resp.Status,
		Healthy:     resp.StatusCode >= 200 && resp.StatusCode < 400,
		LatencyMs:   latencyMs,
		ContentType: resp.Header.Get("Content-Type"),
		Server:      resp.Header.Get("Server"),
	}

	var jsonMap map[string]any
	if err := json.Unmarshal(bodyBytes, &jsonMap); err == nil {
		result.Body = jsonMap
		if v, ok := jsonMap["commit_sha"].(string); ok && v != "" {
			result.CommitSHA = v
		} else if v, ok := jsonMap["commitSha"].(string); ok && v != "" {
			result.CommitSHA = v
		} else if v, ok := jsonMap["sha"].(string); ok && v != "" {
			result.CommitSHA = v
		} else if v, ok := jsonMap["revision"].(string); ok && v != "" {
			result.CommitSHA = v
		}

		if v, ok := jsonMap["app"].(string); ok && v != "" {
			result.App = v
		}
		if v, ok := jsonMap["version"].(string); ok && v != "" {
			result.Version = v
		}
		if v, ok := jsonMap["timestamp"].(string); ok && v != "" {
			result.Timestamp = v
		}
	} else {
		previewLen := len(bodyBytes)
		if previewLen > 512 {
			previewLen = 512
		}
		result.RawBodyPreview = string(bodyBytes[:previewLen])
	}

	return result, nil
}

// InspectDeploymentStatus inspects live deployment status and compares it with local repository HEAD.
func InspectDeploymentStatus(ctx context.Context, project, dir, healthURL string) (*DeploymentStatusResult, error) {
	project = strings.TrimSpace(project)
	if project == "" {
		if envProj := os.Getenv("CUAN_PROJECT_NAME"); envProj != "" {
			project = envProj
		} else {
			project = "cuan"
		}
	}

	res := &DeploymentStatusResult{
		Project:        project,
		CheckedAt:      time.Now().UTC().Format(time.RFC3339),
		PendingCommits: []string{},
	}

	cleanDir, dirErr := sanitizeRepoDir(dir)
	if dirErr == nil {
		headOut, _, err := runBoundedCommand(ctx, DefaultTimeout, cleanDir, "git", "rev-parse", "HEAD")
		if err == nil {
			fullSHA := strings.TrimSpace(string(headOut))
			if len(fullSHA) >= 7 {
				res.LocalHeadSHA = fullSHA
				res.LocalShortSHA = fullSHA[:7]
			}
		}

		branchOut, _, err := runBoundedCommand(ctx, DefaultTimeout, cleanDir, "git", "rev-parse", "--abbrev-ref", "HEAD")
		if err == nil {
			res.LocalBranch = strings.TrimSpace(string(branchOut))
		}
	}

	probeURL := strings.TrimSpace(healthURL)
	if probeURL == "" {
		if project == "cuan" || project == "default" {
			probeURL = "https://cuan.dojo.cc/healthz"
		}
	}

	if probeURL != "" {
		probeRes, probeErr := ProbeEndpointHealth(ctx, probeURL, 5)
		if probeErr == nil && probeRes != nil {
			res.EndpointHealth = probeRes
			if probeRes.CommitSHA != "" {
				res.LiveCommitSHA = probeRes.CommitSHA
				if len(probeRes.CommitSHA) >= 7 {
					res.LiveShortSHA = probeRes.CommitSHA[:7]
				} else {
					res.LiveShortSHA = probeRes.CommitSHA
				}
			}
			if probeRes.Version != "" {
				res.LiveVersion = probeRes.Version
			}
		}
	}

	for _, cand := range []string{
		filepath.Join("/opt", project, "current"),
		"/opt/cuan/current",
	} {
		if fi, err := os.Lstat(cand); err == nil {
			res.ReleasePath = cand
			if fi.Mode()&os.ModeSymlink != 0 {
				if target, err := os.Readlink(cand); err == nil {
					res.CurrentTarget = target
				}
			}
			if res.LiveCommitSHA == "" {
				for _, revFile := range []string{
					filepath.Join(cand, "REVISION"),
					filepath.Join(res.CurrentTarget, "REVISION"),
				} {
					if revBytes, err := os.ReadFile(revFile); err == nil {
						rev := strings.TrimSpace(string(revBytes))
						if rev != "" {
							res.LiveCommitSHA = rev
							if len(rev) >= 7 {
								res.LiveShortSHA = rev[:7]
							}
							break
						}
					}
				}
			}
			break
		}
	}

	svcName := project + ".service"
	if project == "default" {
		svcName = "cuan.service"
	}
	svcOut, _, err := runBoundedCommand(ctx, 2*time.Second, "", "systemctl", "is-active", svcName)
	if err == nil {
		res.ServiceStatus = strings.TrimSpace(string(svcOut))
	}

	if res.LiveCommitSHA != "" && res.LocalHeadSHA != "" {
		if strings.HasPrefix(res.LocalHeadSHA, res.LiveCommitSHA) ||
			strings.HasPrefix(res.LiveCommitSHA, res.LocalHeadSHA) ||
			res.LiveCommitSHA == res.LocalHeadSHA {
			res.Synced = true
			res.PendingCommitsCount = 0
		} else {
			res.Synced = false
			if cleanDir != "" {
				logOut, _, err := runBoundedCommand(ctx, DefaultTimeout, cleanDir, "git", "log", res.LiveCommitSHA+"..HEAD", "--oneline", "-n10")
				if err == nil {
					lines := strings.Split(strings.TrimSpace(string(logOut)), "\n")
					var pending []string
					for _, l := range lines {
						l = strings.TrimSpace(l)
						if l != "" {
							pending = append(pending, l)
						}
					}
					res.PendingCommits = pending
					res.PendingCommitsCount = len(pending)
				}
			}
		}
	} else if res.LiveCommitSHA != "" || res.LocalHeadSHA != "" {
		res.Synced = false
	}

	return res, nil
}
