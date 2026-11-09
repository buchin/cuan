package git

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"time"

	"github.com/buchin/cuan/internal/agent"
)

type acceptedVerificationSnapshot struct {
	Policy           string            `json:"policy"`
	TaskID           string            `json:"task_id"`
	SourceCommit     string            `json:"source_commit"`
	TargetCommit     string            `json:"target_commit"`
	AcceptedAt       time.Time         `json:"accepted_at"`
	GeneratedConfigs map[string]string `json:"generated_configs"`
}

// Record native acceptance without removing files. Only the existing clean,
// independently owned snapshot can become a later maintenance candidate.
func (c *Client) MarkVerificationSnapshotAccepted(ctx context.Context, root, taskID, source, target string, acceptedAt time.Time) error {
	if taskID == "" || acceptedAt.IsZero() || !snapshotObjectID.MatchString(source) || !snapshotObjectID.MatchString(target) {
		return errors.New("accepted snapshot identity unavailable")
	}
	root, err := filepath.Abs(root)
	if err != nil {
		return err
	}
	if _, err = os.Lstat(filepath.Join(root, ".git", verificationColdMarker)); err == nil {
		return nil
	} else if !os.IsNotExist(err) {
		return err
	}
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	lease, err := AcquireVerificationLease(ctx, root)
	if err != nil {
		return err
	}
	defer lease.Close()
	receipt := filepath.Join(root, ".git", "cuan-verification.json")
	info, err := os.Lstat(receipt)
	if err != nil || !info.Mode().IsRegular() || info.Size() > 4096 {
		return errors.New("native snapshot binding unavailable")
	}
	raw, err := os.ReadFile(receipt)
	var binding verificationSnapshotBinding
	if err != nil || json.Unmarshal(raw, &binding) != nil || binding.SourceCommit != source || binding.TargetCommit != target {
		return errors.New("native snapshot binding differs")
	}
	path := filepath.Join(root, ".git", "cuan-verification-accepted.json")
	if info, err = os.Lstat(path); err == nil {
		if !info.Mode().IsRegular() || info.Size() > 4096 {
			return errors.New("accepted snapshot receipt changed")
		}
		oldRaw, e := os.ReadFile(path)
		var old acceptedVerificationSnapshot
		if e != nil || json.Unmarshal(oldRaw, &old) != nil || old.Policy != "retained-git-source-v1" || old.TaskID != taskID || old.SourceCommit != source || old.TargetCommit != target {
			return errors.New("accepted snapshot receipt differs")
		}
		if !old.AcceptedAt.Before(acceptedAt) {
			return nil
		}
	} else if !os.IsNotExist(err) {
		return err
	}
	if err = c.validateVerificationSnapshot(ctx, root, binding); err != nil {
		return err
	}
	status, err := c.snapshotGit(ctx, root, "status", "--porcelain=v1", "--ignored")
	if err != nil || !reconstructibleWorkspaceStatus(root, status) {
		return errors.New("unique snapshot files must be retained")
	}
	accepted := acceptedVerificationSnapshot{Policy: "retained-git-source-v1", TaskID: taskID, SourceCommit: source, TargetCommit: target, AcceptedAt: acceptedAt.UTC(), GeneratedConfigs: map[string]string{}}
	for _, name := range []string{".agents/mcp_config.json", ".gemini/config/mcp_config.json"} {
		data, e := os.ReadFile(filepath.Join(root, name))
		if os.IsNotExist(e) {
			continue
		}
		if e != nil || !bytes.Equal(data, agent.GeneratedWorkspaceMCPConfig()) {
			return errors.New("custom snapshot settings must be retained")
		}
		digest := sha256.Sum256(data)
		accepted.GeneratedConfigs[name] = hex.EncodeToString(digest[:])
	}
	raw, err = json.Marshal(accepted)
	if err != nil {
		return err
	}
	file, err := os.CreateTemp(filepath.Join(root, ".git"), ".accepted-preparing-")
	if err != nil {
		return err
	}
	defer os.Remove(file.Name())
	if _, err = file.Write(raw); err == nil {
		err = file.Sync()
	}
	closeErr := file.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	return os.Rename(file.Name(), path)
}
