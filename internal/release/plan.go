// Package release executes a repository's maintained release entry point.
// Process success is not deployment evidence or task acceptance.
package release

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/buchin/cuan/internal/git"
)

const ManifestPath = ".cuan/release.json"

type specification struct {
	Version        int      `json:"version"`
	Command        []string `json:"command"`
	TimeoutSeconds int      `json:"timeout_seconds"`
}

// Plan is immutable outside this package. Authorization and receipt validation
// remain the caller's responsibility; this object binds execution to source.
type Plan struct {
	root, commit, branch string
	spec                 specification
	manifestHash         [32]byte
}

type Description struct {
	Commit         string   `json:"commit"`
	Branch         string   `json:"branch"`
	Command        []string `json:"command"`
	TimeoutSeconds int      `json:"timeout_seconds"`
}

func (p *Plan) Description() Description {
	return Description{p.commit, p.branch, append([]string(nil), p.spec.Command...), p.spec.TimeoutSeconds}
}

func source(ctx context.Context, root, commit, branch string) error {
	if !git.IsCommitHash(commit) || branch == "" {
		return errors.New("full source commit and target branch are required")
	}
	check, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	if git.NewClient().VerifySnapshotCommit(check, root, commit, commit) != nil {
		return errors.New("release requires the exact clean source commit")
	}
	cmd := exec.CommandContext(check, "git", "-C", root, "symbolic-ref", "--short", "HEAD")
	cmd.Env = []string{"PATH=" + os.Getenv("PATH"), "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_NOSYSTEM=1"}
	actual, err := cmd.Output()
	if err != nil || strings.TrimSpace(string(actual)) != branch {
		return errors.New("release target branch does not match the prepared source")
	}
	return nil
}

func trackedFile(ctx context.Context, root, relative string, executable bool) error {
	if !filepath.IsLocal(relative) {
		return errors.New("release paths must stay inside the repository")
	}
	path := root
	for _, part := range strings.Split(filepath.Clean(relative), string(filepath.Separator)) {
		path = filepath.Join(path, part)
		info, err := os.Lstat(path)
		if err != nil || info.Mode()&os.ModeSymlink != 0 {
			return errors.New("release paths must not traverse symbolic links")
		}
	}
	info, err := os.Stat(path)
	if err != nil || !info.Mode().IsRegular() || (executable && info.Mode().Perm()&0111 == 0) {
		return errors.New("release entry point must be a regular executable file")
	}
	cmd := exec.CommandContext(ctx, "git", "-C", root, "ls-files", "--error-unmatch", "--", filepath.Clean(relative))
	cmd.Env = []string{"PATH=" + os.Getenv("PATH"), "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_NOSYSTEM=1"}
	if cmd.Run() != nil {
		return errors.New("release descriptor and entry point must be tracked source files")
	}
	return nil
}

func Load(ctx context.Context, root, commit, branch string) (*Plan, error) {
	absolute, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	real, err := filepath.EvalSymlinks(absolute)
	if err != nil {
		return nil, errors.New("release repository is unavailable")
	}
	if err = source(ctx, real, commit, branch); err != nil {
		return nil, err
	}
	if err = trackedFile(ctx, real, ManifestPath, false); err != nil {
		return nil, err
	}
	file, err := os.Open(filepath.Join(real, ManifestPath))
	if err != nil {
		return nil, errors.New("release descriptor is unavailable")
	}
	defer file.Close()
	raw, err := io.ReadAll(io.LimitReader(file, 16385))
	if err != nil || len(raw) > 16384 {
		return nil, errors.New("release descriptor exceeds its size budget")
	}
	var spec specification
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&spec) != nil || decoder.Decode(&struct{}{}) != io.EOF || spec.Version != 1 || len(spec.Command) == 0 || len(spec.Command) > 16 || spec.TimeoutSeconds < 1 || spec.TimeoutSeconds > 1800 {
		return nil, errors.New("invalid release descriptor")
	}
	for _, arg := range spec.Command {
		if arg == "" || len(arg) > 2048 || strings.ContainsRune(arg, 0) {
			return nil, errors.New("invalid release command argument")
		}
	}
	if err = trackedFile(ctx, real, spec.Command[0], true); err != nil {
		return nil, err
	}
	return &Plan{real, commit, branch, spec, sha256.Sum256(raw)}, nil
}
