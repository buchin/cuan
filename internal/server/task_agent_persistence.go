package server

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"time"

	"github.com/buchin/cuan/internal/agent"
	"github.com/buchin/cuan/internal/crypto"
)

type persistedTaskGrant struct {
	TaskID, AttemptID, AgentID, Workspace, Digest string
	EvidenceDirectory                             string
	Expires                                       time.Time
}

func taskGrantPath(digest [32]byte) string {
	return filepath.Join(agent.ConfigHome(), ".local", "share", "cuan", "task-session-grants", hex.EncodeToString(digest[:])+".json")
}
func (s *Server) persistTaskGrant(digest [32]byte, g taskAgentGrant, workspace string) error {
	record := persistedTaskGrant{TaskID: g.TaskID, AttemptID: g.AttemptID, AgentID: g.AgentID, Workspace: workspace, Digest: hex.EncodeToString(digest[:]), Expires: g.Expires}
	if g.EvidenceRoot != nil {
		record.EvidenceDirectory = g.EvidenceRoot.Name()
	}
	raw, err := json.Marshal(record)
	if err != nil {
		return err
	}
	encrypted, err := crypto.Encrypt(string(raw), s.cfg.AuthSecret)
	if err != nil {
		return err
	}
	path := taskGrantPath(digest)
	if err = os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	_, err = f.WriteString(encrypted)
	if err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	dir, err := os.Open(filepath.Dir(path))
	if err != nil {
		return err
	}
	defer dir.Close()
	return dir.Sync()
}
func (s *Server) readTaskGrantRecord(digest [32]byte) (persistedTaskGrant, error) {
	var record persistedTaskGrant
	path := taskGrantPath(digest)
	info, err := os.Lstat(path)
	if err != nil {
		return record, err
	}
	if !info.Mode().IsRegular() || info.Size() > 16000 {
		return record, errors.New("Invalid persisted session grant")
	}
	encrypted, err := os.ReadFile(path)
	if err != nil {
		return record, err
	}
	raw, err := crypto.Decrypt(string(encrypted), s.cfg.AuthSecret)
	if err != nil {
		return record, err
	}
	if json.Unmarshal([]byte(raw), &record) != nil || record.Digest != hex.EncodeToString(digest[:]) || record.TaskID == "" || record.AttemptID == "" || record.AgentID == "" || record.Expires.IsZero() {
		return record, errors.New("Invalid persisted session grant")
	}
	return record, nil
}

func (s *Server) loadTaskGrant(digest [32]byte) (taskAgentGrant, error) {
	var grant taskAgentGrant
	record, err := s.readTaskGrantRecord(digest)
	if err != nil || record.Expires.Before(time.Now()) {
		return grant, errors.New("Invalid persisted session grant")
	}
	grant = taskAgentGrant{TaskID: record.TaskID, AttemptID: record.AttemptID, AgentID: record.AgentID, Expires: record.Expires}
	if record.Workspace != "" {
		info, err := os.Lstat(record.Workspace)
		if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return taskAgentGrant{}, errors.New("Task artifact workspace unavailable")
		}
		grant.ArtifactRoot, err = os.OpenRoot(record.Workspace)
		if err != nil {
			return taskAgentGrant{}, err
		}
	}
	if record.EvidenceDirectory != "" {
		info, err := os.Lstat(record.EvidenceDirectory)
		if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			if grant.ArtifactRoot != nil {
				grant.ArtifactRoot.Close()
			}
			return taskAgentGrant{}, errors.New("Task evidence directory unavailable")
		}
		grant.EvidenceRoot, err = os.OpenRoot(record.EvidenceDirectory)
		if err != nil {
			if grant.ArtifactRoot != nil {
				grant.ArtifactRoot.Close()
			}
			return taskAgentGrant{}, err
		}
	}
	if actual, loaded := s.taskAgentGrants.LoadOrStore(digest, grant); loaded {
		if grant.ArtifactRoot != nil {
			grant.ArtifactRoot.Close()
		}
		if grant.EvidenceRoot != nil {
			grant.EvidenceRoot.Close()
		}
		return actual.(taskAgentGrant), nil
	}
	return grant, nil
}

// The durable record contains only an encrypted, attempt-bound grant. Tokens
// themselves are never written to this index or inferred from their length.
