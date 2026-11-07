package project

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"path/filepath"
	"strings"
	"time"

	"github.com/buchin/cuan/internal/executionpolicy"
	"github.com/buchin/cuan/internal/git"
	"github.com/buchin/cuan/internal/release"
)

type NativeReleaseWorkspaceRequest struct {
	ProjectID, TaskID, Instructions, Repository, TargetBranch string
	SourceCommit, TargetCommit, ManifestHash                  string
}

type PreparedNativeReleaseWorkspace struct {
	Path         string `json:"path"`
	SourceCommit string `json:"source_commit"`
	TargetCommit string `json:"target_commit"`
	Branch       string `json:"branch"`
	ManifestHash string `json:"manifest_hash"`
}

// NativeReleaseWorkspace prepares immutable publication output without executing
// it. Caller owns original approval, handoff and merge-receipt validation.
func (s *Service) NativeReleaseWorkspace(ctx context.Context, req NativeReleaseWorkspaceRequest) (*PreparedNativeReleaseWorkspace, error) {
	if req.TaskID == "" || !git.IsCommitHash(req.SourceCommit) || !git.IsCommitHash(req.TargetCommit) || len(req.ManifestHash) != 64 {
		return nil, errors.New("release workspace source identity unavailable")
	}
	if _, err := hex.DecodeString(req.ManifestHash); err != nil {
		return nil, errors.New("release descriptor identity unavailable")
	}
	limits := executionpolicy.FromInstructions(req.Instructions)
	if limits.DenyPush || limits.DenyDeploy || limits.ReadOnlyRepository || executionpolicy.StandaloneArtifactWork(req.Instructions) {
		return nil, errors.New("release workspace publication is not authorized")
	}
	p, err := s.TaskContext(ctx, req.ProjectID, req.Instructions)
	if err != nil || p == nil || p.IsArchived || p.LocalPath == "" {
		return nil, errors.New("saved release workspace context unavailable")
	}
	owner, repo, err := git.ParseGitHubURL(p.RepoURL)
	if err != nil || req.Repository != owner+"/"+repo {
		return nil, errors.New("release workspace repository differs from saved task")
	}
	settings := SettingsFromMetadata(p.Metadata)
	branch := strings.TrimSpace(settings.DeployBranch)
	if branch == "" {
		branch = p.DefaultBranch
	}
	if branch == "" || branch != req.TargetBranch || !settings.IsAutoDeployEnabled() || ParseEnvironment(settings.Environment)["CUAN_MANAGED_RELEASE"] != "1" {
		return nil, errors.New("release workspace target differs from saved task")
	}
	digest := sha256.Sum256([]byte(req.ProjectID + "\x00" + req.TaskID + "\x00" + req.SourceCommit + "\x00" + req.TargetCommit + "\x00" + branch + "\x00" + req.ManifestHash))
	destination := filepath.Join(s.baseDir, "release-workspaces", hex.EncodeToString(digest[:]))
	path, err := s.gitClient.PrepareReleaseWorkspace(ctx, p.LocalPath, destination, req.SourceCommit, req.TargetCommit, branch)
	if errors.Is(err, git.ErrVerificationCommitUnavailable) {
		current, loadErr := s.store.GetProject(ctx, req.ProjectID)
		if loadErr != nil || current == nil || current.IsArchived {
			return nil, errors.New("saved release connection unavailable")
		}
		token, accessErr := s.repositoryToken(ctx, current.EncryptedToken)
		if accessErr != nil {
			return nil, accessErr
		}
		if strings.TrimSpace(token) == "" {
			return nil, ErrGitHubConnectionRequired
		}
		fetch, cancel := context.WithTimeout(ctx, 30*time.Second)
		defer cancel()
		for _, commit := range []string{req.SourceCommit, req.TargetCommit} {
			if err := s.gitClient.FetchVerificationCommit(fetch, p.LocalPath, p.RepoURL, commit, token); err != nil {
				return nil, err
			}
		}
		path, err = s.gitClient.PrepareReleaseWorkspace(ctx, p.LocalPath, destination, req.SourceCommit, req.TargetCommit, branch)
	}
	if err != nil {
		return nil, err
	}
	if err := s.gitClient.BindReleaseOrigin(ctx, path, p.RepoURL); err != nil {
		return nil, err
	}
	plan, err := release.Load(ctx, path, req.TargetCommit, branch)
	if err != nil {
		return nil, err
	}
	if plan.Description().ManifestHash != req.ManifestHash {
		return nil, errors.New("release descriptor differs from admitted source; no script executed")
	}
	return &PreparedNativeReleaseWorkspace{Path: path, SourceCommit: req.SourceCommit, TargetCommit: req.TargetCommit, Branch: branch, ManifestHash: req.ManifestHash}, nil
}
