package runner

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"time"

	"github.com/buchin/cuan/internal/executionpolicy"
	"github.com/buchin/cuan/internal/git"
	"github.com/buchin/cuan/internal/project"
	"github.com/buchin/cuan/internal/release"
	"github.com/buchin/cuan/internal/task"
)

const managedReleaseClaimActivity = "managed_release_claim"

// ManagedReleaseClaim freezes native publication ownership at admission. It is
// not a release receipt and does not exempt completion from deployment checks.
type ManagedReleaseClaim struct {
	Version          int    `json:"version"`
	TaskID           string `json:"task_id"`
	ProjectID        string `json:"project_id"`
	RoomID           string `json:"room_id"`
	AttemptID        string `json:"attempt_id"`
	RunnerID         string `json:"runner_id"`
	InstructionsHash string `json:"instructions_hash"`
	Repository       string `json:"repository"`
	TargetBranch     string `json:"target_branch"`
}

func (s *RunnerService) bindManagedReleaseClaim(ctx context.Context, req ClaimTaskRequest, claim *ClaimTaskResponse) error {
	if claim == nil || !claim.Claimed || claim.Task == nil || claim.Attempt == nil || !req.SupportsNativeRelease || !claim.AutoDeploy || claim.NativeRelease != nil || claim.Verification != nil || claim.LiveInspection != nil {
		return nil
	}
	t, att := claim.Task, claim.Attempt
	limits := claim.PublicationLimits
	if project.ParseEnvironment(claim.Environment)["CUAN_MANAGED_RELEASE"] != "1" || t.ProjectID == "" || t.SourceRoomID == "" || t.ExecutionCategory != task.ExecutionCategoryCode || limits.DenyPush || limits.DenyDeploy || limits.ReadOnlyRepository || executionpolicy.StandaloneArtifactWork(t.Instructions) {
		return nil
	}
	if att.TaskID != t.ID || att.RunnerID != req.RunnerID || att.Status != task.AttemptStatusRunning || att.StartedAt == nil || att.CancelRequested || att.CanceledAt != nil || claim.RepoOwner == "" || claim.RepoName == "" || strings.TrimSpace(claim.DeployBranch) == "" {
		return errors.New("managed release admission identity is unavailable")
	}
	binding := &ManagedReleaseClaim{Version: 1, TaskID: t.ID, ProjectID: t.ProjectID, RoomID: t.SourceRoomID, AttemptID: att.ID, RunnerID: att.RunnerID, InstructionsHash: task.LaunchInstructionsHash(t.Instructions), Repository: claim.RepoOwner + "/" + claim.RepoName, TargetBranch: claim.DeployBranch}
	raw, err := json.Marshal(binding)
	if err != nil {
		return err
	}
	if err := s.taskStore.CreateActivity(ctx, &task.TaskActivity{ID: "managed-release-claim:" + att.ID, TaskID: t.ID, AttemptID: &att.ID, ActivityType: managedReleaseClaimActivity, Author: att.RunnerID, Message: string(raw), CreatedAt: time.Now().UTC()}); err != nil {
		return err
	}
	claim.ManagedRelease = binding
	return nil
}

// managedReleaseClaimForCompletion reads only the canonical backend admission.
// A newly enabled project setting cannot create ownership retroactively.
func (s *RunnerService) managedReleaseClaimForCompletion(ctx context.Context, t *task.Task, att *task.TaskAttempt) (*ManagedReleaseClaim, error) {
	activities, err := s.taskStore.ListActivities(ctx, t.ID)
	if err != nil {
		return nil, err
	}
	var receipt *task.TaskActivity
	for _, a := range activities {
		if a == nil || a.ID != "managed-release-claim:"+att.ID {
			continue
		}
		if receipt != nil {
			return nil, errors.New("duplicate managed release admission")
		}
		receipt = a
	}
	if receipt == nil {
		return nil, nil
	}
	var binding ManagedReleaseClaim
	decoder := json.NewDecoder(strings.NewReader(receipt.Message))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&binding) != nil || decoder.Decode(&struct{}{}) != io.EOF || binding.Version != 1 || binding.TaskID != t.ID || binding.ProjectID != t.ProjectID || binding.RoomID != t.SourceRoomID || binding.AttemptID != att.ID || binding.RunnerID != att.RunnerID || binding.InstructionsHash != task.LaunchInstructionsHash(t.Instructions) || receipt.TaskID != t.ID || receipt.AttemptID == nil || *receipt.AttemptID != att.ID || receipt.Author != att.RunnerID || receipt.ActivityType != managedReleaseClaimActivity || att.StartedAt == nil || receipt.CreatedAt.Before(*att.StartedAt) || receipt.CreatedAt.After(time.Now().UTC()) || att.CancelRequested || att.CanceledAt != nil || t.ProjectID == "" || t.SourceRoomID == "" || t.ExecutionCategory != task.ExecutionCategoryCode || s.projectFinder == nil {
		return nil, errors.New("managed release admission no longer matches execution")
	}
	limits, err := task.AttemptPublicationPolicy(ctx, s.taskStore, t, att.ID)
	if err != nil {
		return nil, err
	}
	if limits.DenyPush || limits.DenyDeploy || limits.ReadOnlyRepository || executionpolicy.StandaloneArtifactWork(t.Instructions) {
		return nil, errors.New("managed release publication is not authorized")
	}
	p, err := s.projectFinder.TaskContext(ctx, t.ProjectID, t.Instructions)
	if err != nil || p == nil {
		return nil, errors.New("saved managed release context unavailable")
	}
	settings := project.SettingsFromMetadata(p.Metadata)
	branch := strings.TrimSpace(settings.DeployBranch)
	if branch == "" {
		branch = p.DefaultBranch
	}
	if !settings.IsAutoDeployEnabled() || project.ParseEnvironment(settings.Environment)["CUAN_MANAGED_RELEASE"] != "1" || binding.Repository != p.RepoOwner+"/"+p.RepoName || binding.TargetBranch != branch || branch == "" {
		return nil, errors.New("managed release context differs from admission")
	}
	return &binding, nil
}

// ManagedReleaseSource is native source observation, not publication evidence.
type ManagedReleaseSource struct {
	Commit           string `json:"commit"`
	Branch           string `json:"branch"`
	ManifestHash     string `json:"manifest_hash"`
	VerificationHash string `json:"verification_hash"`
}

func observeManagedReleaseSource(ctx context.Context, root, commit, branch, summary string) (*ManagedReleaseSource, error) {
	if !task.PassingProjectCheckSummary(summary) {
		return nil, errors.New("passing native project checks required for release handoff")
	}
	plan, err := release.Load(ctx, root, commit, branch)
	if err != nil {
		return nil, err
	}
	description := plan.Description()
	return &ManagedReleaseSource{Commit: description.Commit, Branch: description.Branch, ManifestHash: description.ManifestHash, VerificationHash: task.VerificationSummaryHash(summary)}, nil
}

// A resumed developer must reconcile the baseline supplied for this execution,
// rather than repeatedly submitting an old checkpoint for the same merge failure.
// Publication may still merge newer remote changes after independent review.
func (r *Runner) verifyManagedDeveloperBaseline(ctx context.Context, workspace *git.TaskWorkspace) error {
	const gap = "Native developer source does not include the pinned current project baseline. Reconcile the approved changes with that baseline on the private task branch, resolve local conflicts while preserving unrelated work and release gates, then report a clean updated checkpoint for independent review. Do not repeat an unchanged source handoff."
	if workspace == nil || workspace.BaseCommit == "" || r.gitClient == nil {
		return errors.New("Native developer baseline is unavailable; saved work is preserved and no source handoff was accepted")
	}
	head, err := r.gitClient.GetLatestCommitSHA(ctx, workspace.Path)
	if err != nil {
		return errors.New("Native developer checkpoint is unavailable; saved work is preserved")
	}
	contained, err := r.gitClient.CommitContained(ctx, workspace.Path, workspace.BaseCommit, head)
	if err != nil {
		return errors.New("Native developer baseline ancestry could not be verified; saved work is preserved")
	}
	if !contained {
		return errors.New(gap)
	}
	return nil
}

func validateManagedReleaseSource(binding *ManagedReleaseClaim, req CompleteAttemptRequest) error {
	source := req.ManagedReleaseSource
	if source == nil {
		return nil
	}
	if binding == nil || source.Commit != req.CommitSHA || !git.IsCommitHash(source.Commit) || source.Branch == "" || source.Branch != req.BranchName || source.VerificationHash != task.VerificationSummaryHash(req.TestSummary) || !task.PassingProjectCheckSummary(req.TestSummary) || len(source.ManifestHash) != 64 {
		return errors.New("managed release source does not match native completion")
	}
	if _, err := hex.DecodeString(source.ManifestHash); err != nil {
		return errors.New("managed release manifest identity unavailable")
	}
	return nil
}

func (s *RunnerService) prepareManagedReleaseHandoff(ctx context.Context, binding *ManagedReleaseClaim, t *task.Task, att *task.TaskAttempt, req CompleteAttemptRequest, now time.Time) (*task.TaskActivity, error) {
	if binding == nil {
		return nil, nil
	}
	if req.ManagedReleaseSource == nil || validateManagedReleaseSource(binding, req) != nil || !req.AgentReportRequired || !req.AgentReportConfirmed || req.ExitCode != 0 || req.LiveDeploymentConfirmed || strings.TrimSpace(req.DiffSummary) == "" || req.PRURL != "" || req.PRNumber != 0 {
		return nil, errors.New("managed release source handoff evidence is incomplete")
	}
	activities, err := s.taskStore.ListActivities(ctx, t.ID)
	if err != nil {
		return nil, err
	}
	if !task.ImplementationFinalReport(activities, t.ID, att.ID) {
		return nil, errors.New("managed release requires a saved implementation handoff")
	}
	source := req.ManagedReleaseSource
	receipt := task.NativeReleaseHandoff{Version: 1, TaskID: t.ID, AttemptID: att.ID, SourceCommit: source.Commit, InstructionsHash: binding.InstructionsHash, VerificationHash: source.VerificationHash, Repository: binding.Repository, TargetBranch: binding.TargetBranch, ManifestHash: source.ManifestHash}
	raw, err := json.Marshal(receipt)
	if err != nil {
		return nil, err
	}
	return &task.TaskActivity{ID: "native-release-handoff:" + att.ID, TaskID: t.ID, AttemptID: &att.ID, ActivityType: task.ActivityNativeReleaseHandoff, Author: att.RunnerID, Message: string(raw), CreatedAt: now}, nil
}
