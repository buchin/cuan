package server

import (
	"context"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/buchin/cuan/internal/task"
)

func (s *Server) forgetTaskGrant(digest [32]byte) {
	if value, ok := s.taskAgentGrants.LoadAndDelete(digest); ok {
		grant := value.(taskAgentGrant)
		if grant.ArtifactRoot != nil {
			grant.ArtifactRoot.Close()
		}
		if grant.EvidenceRoot != nil {
			grant.EvidenceRoot.Close()
		}
	}
}

// Only expired authenticated grant records may identify disposable session
// output. Repositories, attachments and arbitrary /tmp directories are excluded.
func (s *Server) pruneExpiredTaskEvidence(ctx context.Context, now time.Time) {
	if s.taskSvc == nil || s.cfg == nil {
		return
	}
	directory := filepath.Dir(taskGrantPath([32]byte{}))
	entries, err := os.ReadDir(directory)
	if err != nil {
		return
	}
	for _, entry := range entries {
		if ctx.Err() != nil {
			return
		}
		if entry.IsDir() || entry.Type()&os.ModeSymlink != 0 || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		raw, err := hex.DecodeString(strings.TrimSuffix(entry.Name(), ".json"))
		if err != nil || len(raw) != 32 {
			continue
		}
		var digest [32]byte
		copy(digest[:], raw)
		record, err := s.readTaskGrantRecord(digest)
		if err != nil || !record.Expires.Add(time.Hour).Before(now) {
			continue
		}
		attempt, err := s.taskSvc.Store().GetAttempt(ctx, record.AttemptID)
		if err != nil && !errors.Is(err, task.ErrAttemptNotFound) {
			continue
		}
		if attempt != nil && (attempt.TaskID != record.TaskID || attempt.Status == task.AttemptStatusRunning || attempt.Status == task.AttemptStatusQueued) {
			continue
		}
		if record.EvidenceDirectory != "" {
			path := filepath.Clean(record.EvidenceDirectory)
			parent := filepath.Dir(path)
			if filepath.Base(path) != "artifacts" || filepath.Dir(parent) != filepath.Clean(os.TempDir()) || !strings.HasPrefix(filepath.Base(parent), "cuan-task-agent-") {
				continue
			}
			s.forgetTaskGrant(digest)
			if os.RemoveAll(parent) != nil {
				continue
			}
		} else {
			s.forgetTaskGrant(digest)
		}
		os.Remove(taskGrantPath(digest))
	}
}
