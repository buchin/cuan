package project

import (
	"context"
	"errors"
	"time"
)

// Settings writes are atomic with the expected version. Other project fields
// (credentials, modules, archive state) cannot be overwritten by a stale form.
func (s *DBStore) UpdateSettingsIfUnchanged(ctx context.Context, p *Project, expected time.Time) error {
	p.UpdatedAt = time.Now().UTC().Truncate(time.Microsecond)
	result, err := s.db.ExecContext(ctx, `UPDATE projects SET name=$2,repo_url=$3,repo_owner=$4,repo_name=$5,
 default_branch=$6,local_path=$7,clone_status=$8,last_error=$9,metadata=$10,source_metadata=$11,updated_at=$12
 WHERE id=$1 AND updated_at=$13`, p.ID, p.Name, p.RepoURL, p.RepoOwner, p.RepoName, p.DefaultBranch, p.LocalPath, p.CloneStatus, p.LastError, []byte(p.Metadata), []byte(p.SourceMetadata), p.UpdatedAt, expected)
	if err != nil {
		return err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if count != 1 {
		return ErrSettingsConflict
	}
	return nil
}

func (m *MemoryStore) UpdateSettingsIfUnchanged(ctx context.Context, p *Project, expected time.Time) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	previous, ok := m.projects[p.ID]
	if !ok {
		return ErrProjectNotFound
	}
	if !previous.UpdatedAt.Equal(expected) {
		return ErrSettingsConflict
	}
	m.projects[p.ID] = cloneMemoryProject(p)
	return nil
}

func (s *Service) saveSettings(ctx context.Context, p *Project, expected time.Time) error {
	writer, ok := s.store.(interface {
		UpdateSettingsIfUnchanged(context.Context, *Project, time.Time) error
	})
	if !ok {
		return ErrSettingsConflict
	}
	err := writer.UpdateSettingsIfUnchanged(ctx, p, expected)
	if err != nil && !errors.Is(err, ErrSettingsConflict) && !errors.Is(err, ErrProjectNotFound) {
		return errors.New("Could not save project settings. Check the name and repository, then try again.")
	}
	return err
}
