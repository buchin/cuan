package auth

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"
)

type DBStore struct {
	db *sql.DB
}

func NewDBStore(db *sql.DB) *DBStore {
	return &DBStore{db: db}
}

func (s *DBStore) IsSetup(ctx context.Context) (bool, error) {
	var count int
	query := "SELECT count(*) FROM owners WHERE is_setup = true"
	if err := s.db.QueryRowContext(ctx, query).Scan(&count); err != nil {
		return false, fmt.Errorf("check is_setup: %w", err)
	}
	return count > 0, nil
}

func (s *DBStore) CountOwners(ctx context.Context) (int, error) {
	var count int
	query := "SELECT count(*) FROM owners"
	if err := s.db.QueryRowContext(ctx, query).Scan(&count); err != nil {
		return 0, fmt.Errorf("count owners: %w", err)
	}
	return count, nil
}

func (s *DBStore) CreateOwner(ctx context.Context, owner *Owner) error {
	username := owner.Username
	if username == "" {
		if strings.Contains(owner.Email, "@") {
			username = strings.Split(owner.Email, "@")[0]
		} else {
			username = "admin"
		}
	}
	owner.Username = username

	query := `
		INSERT INTO owners (id, email, username, password_hash, is_setup, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`
	_, err := s.db.ExecContext(ctx, query,
		owner.ID,
		owner.Email,
		owner.Username,
		owner.PasswordHash,
		owner.IsSetup,
		owner.CreatedAt,
		owner.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("create owner: %w", err)
	}
	return nil
}

func (s *DBStore) GetOwner(ctx context.Context) (*Owner, error) {
	query := `
		SELECT id, email, COALESCE(username, 'admin'), password_hash, is_setup, created_at, updated_at
		FROM owners
		ORDER BY created_at ASC
		LIMIT 1
	`
	var o Owner
	err := s.db.QueryRowContext(ctx, query).Scan(
		&o.ID,
		&o.Email,
		&o.Username,
		&o.PasswordHash,
		&o.IsSetup,
		&o.CreatedAt,
		&o.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, ErrOwnerNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get owner: %w", err)
	}
	return &o, nil
}

func (s *DBStore) UpdateOwnerPassword(ctx context.Context, ownerID string, passwordHash string) error {
	query := `
		UPDATE owners
		SET password_hash = $2, updated_at = $3
		WHERE id = $1
	`
	res, err := s.db.ExecContext(ctx, query, ownerID, passwordHash, time.Now().UTC())
	if err != nil {
		return fmt.Errorf("update owner password: %w", err)
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("update owner password rows affected: %w", err)
	}
	if rows == 0 {
		return ErrOwnerNotFound
	}
	return nil
}

func (s *DBStore) CreateSession(ctx context.Context, session *Session) error {
	query := `
		INSERT INTO owner_sessions (token_hash, owner_id, expires_at, created_at)
		VALUES ($1, $2, $3, $4)
	`
	_, err := s.db.ExecContext(ctx, query,
		session.TokenHash,
		session.OwnerID,
		session.ExpiresAt,
		session.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("create session: %w", err)
	}
	return nil
}

func (s *DBStore) GetSession(ctx context.Context, tokenHash string) (*Session, *Owner, error) {
	query := `
		SELECT 
			s.token_hash, s.owner_id, s.expires_at, s.created_at,
			o.id, o.email, COALESCE(o.username, 'admin'), o.password_hash, o.is_setup, o.created_at, o.updated_at
		FROM owner_sessions s
		JOIN owners o ON s.owner_id = o.id
		WHERE s.token_hash = $1
	`
	var sess Session
	var o Owner

	err := s.db.QueryRowContext(ctx, query, tokenHash).Scan(
		&sess.TokenHash,
		&sess.OwnerID,
		&sess.ExpiresAt,
		&sess.CreatedAt,
		&o.ID,
		&o.Email,
		&o.Username,
		&o.PasswordHash,
		&o.IsSetup,
		&o.CreatedAt,
		&o.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil, ErrUnauthorized
	}
	if err != nil {
		return nil, nil, fmt.Errorf("get session: %w", err)
	}

	return &sess, &o, nil
}

func (s *DBStore) DeleteSession(ctx context.Context, tokenHash string) error {
	query := "DELETE FROM owner_sessions WHERE token_hash = $1"
	_, err := s.db.ExecContext(ctx, query, tokenHash)
	if err != nil {
		return fmt.Errorf("delete session: %w", err)
	}
	return nil
}

func (s *DBStore) DeleteExpiredSessions(ctx context.Context) error {
	query := "DELETE FROM owner_sessions WHERE expires_at < $1"
	_, err := s.db.ExecContext(ctx, query, time.Now().UTC())
	if err != nil {
		return fmt.Errorf("delete expired sessions: %w", err)
	}
	return nil
}
