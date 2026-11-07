package runner

import (
	"context"
	"errors"

	"github.com/buchin/cuan/internal/git"
	"github.com/buchin/cuan/internal/project"
	"github.com/buchin/cuan/internal/release"
	"github.com/buchin/cuan/internal/task"
)

// executePreparedNativeRelease consumes only a native coordinator preparation.
// It runs no model and returns process facts, never deployment/DoD acceptance.
// The caller must retain those facts, reconcile live state and schedule review.
func executePreparedNativeRelease(ctx context.Context, intent *task.NativeReleaseIntent, prepared *project.PreparedNativeReleaseWorkspace, environment []string) (release.Result, error) {
	unknown := release.Result{ExitCode: -1}
	if intent == nil || prepared == nil || environment == nil || prepared.Path == "" || prepared.SourceCommit != intent.SourceCommit || prepared.Branch != intent.TargetBranch || prepared.ManifestHash != intent.ManifestHash || !git.IsCommitHash(prepared.SourceCommit) || !git.IsCommitHash(prepared.TargetCommit) {
		return unknown, errors.New("native release preparation does not match admitted source")
	}
	if err := git.NewClient().VerifySnapshotCommit(ctx, prepared.Path, prepared.SourceCommit, prepared.TargetCommit); err != nil {
		return unknown, errors.New("native release target does not contain admitted source in a clean checkout")
	}
	plan, err := release.Load(ctx, prepared.Path, prepared.TargetCommit, prepared.Branch)
	if err != nil {
		return unknown, err
	}
	if plan.Description().ManifestHash != intent.ManifestHash {
		return unknown, errors.New("native release descriptor changed after preparation")
	}
	return release.Run(ctx, plan, environment), nil
}
