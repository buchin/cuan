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
	if owner.Role == "" {
		owner.Role = RoleOwner
	}
	owner.IsActive = true

	// First-owner setup may race across browsers or server processes. Serialize
	// creation in PostgreSQL, then check within the same transaction.
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin owner setup: %w", err)
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, "LOCK TABLE owners IN SHARE ROW EXCLUSIVE MODE"); err != nil && !strings.Contains(err.Error(), "near \"LOCK\"") {
		return fmt.Errorf("lock owner setup: %w", err)
	}
	var exists bool
	if err = tx.QueryRowContext(ctx, "SELECT EXISTS (SELECT 1 FROM owners)").Scan(&exists); err != nil {
		return fmt.Errorf("check owner setup: %w", err)
	}
	if exists {
		return ErrSetupAlreadyCompleted
	}

	query := `
		INSERT INTO owners (id, email, username, password_hash, is_setup, name, role, is_active, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	`
	_, err = tx.ExecContext(ctx, query,
		owner.ID,
		owner.Email,
		owner.Username,
		owner.PasswordHash,
		owner.IsSetup,
		owner.Name,
		owner.Role,
		owner.IsActive,
		owner.CreatedAt,
		owner.UpdatedAt,
	)
	if err != nil && (strings.Contains(err.Error(), "no such column") || strings.Contains(err.Error(), "has no column") || strings.Contains(err.Error(), "does not exist")) {
		queryFallback := `
			INSERT INTO owners (id, email, username, password_hash, is_setup, created_at, updated_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7)
		`
		_, err = tx.ExecContext(ctx, queryFallback,
			owner.ID,
			owner.Email,
			owner.Username,
			owner.PasswordHash,
			owner.IsSetup,
			owner.CreatedAt,
			owner.UpdatedAt,
		)
	}
	if err != nil {
		return fmt.Errorf("create owner: %w", err)
	}
	return tx.Commit()
}

func (s *DBStore) CreateUser(ctx context.Context, user *Owner) error {
	username := user.Username
	if username == "" {
		if strings.Contains(user.Email, "@") {
			username = strings.Split(user.Email, "@")[0]
		} else {
			username = user.ID
		}
	}
	user.Username = username
	if user.Role == "" {
		user.Role = RoleMember
	}
	user.IsActive = true

	query := `
		INSERT INTO owners (id, email, username, password_hash, is_setup, name, role, is_active, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	`
	_, err := s.db.ExecContext(ctx, query,
		user.ID,
		user.Email,
		user.Username,
		user.PasswordHash,
		user.IsSetup,
		user.Name,
		user.Role,
		user.IsActive,
		user.CreatedAt,
		user.UpdatedAt,
	)
	if err != nil {
		if strings.Contains(err.Error(), "duplicate key") || strings.Contains(err.Error(), "UNIQUE constraint failed") {
			return ErrUserAlreadyExists
		}
		return fmt.Errorf("create user: %w", err)
	}
	return nil
}

func (s *DBStore) GetOwner(ctx context.Context) (*Owner, error) {
	query := `
		SELECT id, email, COALESCE(username, 'admin'), password_hash, is_setup, created_at, updated_at,
			COALESCE(name, ''), COALESCE(role, 'owner'), COALESCE(is_active, true)
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
		&o.Name,
		&o.Role,
		&o.IsActive,
	)
	if err != nil && (strings.Contains(err.Error(), "no such column") || strings.Contains(err.Error(), "does not exist")) {
		err = s.db.QueryRowContext(ctx, `
			SELECT id, email, COALESCE(username, 'admin'), password_hash, is_setup, created_at, updated_at
			FROM owners
			ORDER BY created_at ASC
			LIMIT 1
		`).Scan(
			&o.ID,
			&o.Email,
			&o.Username,
			&o.PasswordHash,
			&o.IsSetup,
			&o.CreatedAt,
			&o.UpdatedAt,
		)
		o.Role = RoleOwner
		o.IsActive = true
	}
	if err == sql.ErrNoRows {
		return nil, ErrOwnerNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get owner: %w", err)
	}
	return &o, nil
}

func (s *DBStore) GetOwnerByID(ctx context.Context, id string) (*Owner, error) {
	query := `
		SELECT id, email, COALESCE(username, 'admin'), password_hash, is_setup, created_at, updated_at,
			COALESCE(name, ''), COALESCE(role, 'owner'), COALESCE(is_active, true)
		FROM owners
		WHERE id = $1
	`
	var o Owner
	err := s.db.QueryRowContext(ctx, query, id).Scan(
		&o.ID, &o.Email, &o.Username, &o.PasswordHash, &o.IsSetup, &o.CreatedAt, &o.UpdatedAt,
		&o.Name, &o.Role, &o.IsActive,
	)
	if err != nil && (strings.Contains(err.Error(), "no such column") || strings.Contains(err.Error(), "does not exist")) {
		err = s.db.QueryRowContext(ctx, `
			SELECT id, email, COALESCE(username, 'admin'), password_hash, is_setup, created_at, updated_at
			FROM owners
			WHERE id = $1
		`, id).Scan(
			&o.ID, &o.Email, &o.Username, &o.PasswordHash, &o.IsSetup, &o.CreatedAt, &o.UpdatedAt,
		)
		o.Role = RoleOwner
		o.IsActive = true
	}
	if err == sql.ErrNoRows {
		return nil, ErrOwnerNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get owner by id: %w", err)
	}
	return &o, nil
}

func (s *DBStore) GetOwnerByEmailOrUsername(ctx context.Context, identifier string) (*Owner, error) {
	identifier = strings.TrimSpace(identifier)
	query := `
		SELECT id, email, COALESCE(username, 'admin'), password_hash, is_setup, created_at, updated_at,
			COALESCE(name, ''), COALESCE(role, 'owner'), COALESCE(is_active, true)
		FROM owners
		WHERE LOWER(email) = LOWER($1) OR LOWER(username) = LOWER($1)
		LIMIT 1
	`
	var o Owner
	err := s.db.QueryRowContext(ctx, query, identifier).Scan(
		&o.ID, &o.Email, &o.Username, &o.PasswordHash, &o.IsSetup, &o.CreatedAt, &o.UpdatedAt,
		&o.Name, &o.Role, &o.IsActive,
	)
	if err != nil && (strings.Contains(err.Error(), "no such column") || strings.Contains(err.Error(), "does not exist")) {
		err = s.db.QueryRowContext(ctx, `
			SELECT id, email, COALESCE(username, 'admin'), password_hash, is_setup, created_at, updated_at
			FROM owners
			WHERE LOWER(email) = LOWER($1) OR LOWER(username) = LOWER($1)
			LIMIT 1
		`, identifier).Scan(
			&o.ID, &o.Email, &o.Username, &o.PasswordHash, &o.IsSetup, &o.CreatedAt, &o.UpdatedAt,
		)
		o.Role = RoleOwner
		o.IsActive = true
	}
	if err == sql.ErrNoRows {
		return nil, ErrOwnerNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get owner by identifier: %w", err)
	}
	return &o, nil
}

func (s *DBStore) ListOwners(ctx context.Context) ([]*Owner, error) {
	query := `
		SELECT id, email, COALESCE(username, 'admin'), password_hash, is_setup, created_at, updated_at,
			COALESCE(name, ''), COALESCE(role, 'owner'), COALESCE(is_active, true)
		FROM owners
		ORDER BY created_at ASC
	`
	rows, err := s.db.QueryContext(ctx, query)
	hasNewCols := true
	if err != nil && (strings.Contains(err.Error(), "no such column") || strings.Contains(err.Error(), "does not exist")) {
		hasNewCols = false
		rows, err = s.db.QueryContext(ctx, `
			SELECT id, email, COALESCE(username, 'admin'), password_hash, is_setup, created_at, updated_at
			FROM owners
			ORDER BY created_at ASC
		`)
	}
	if err != nil {
		return nil, fmt.Errorf("list owners: %w", err)
	}
	defer rows.Close()

	var list []*Owner
	for rows.Next() {
		var o Owner
		if hasNewCols {
			if err := rows.Scan(
				&o.ID, &o.Email, &o.Username, &o.PasswordHash, &o.IsSetup, &o.CreatedAt, &o.UpdatedAt,
				&o.Name, &o.Role, &o.IsActive,
			); err != nil {
				return nil, fmt.Errorf("scan owner: %w", err)
			}
		} else {
			if err := rows.Scan(
				&o.ID, &o.Email, &o.Username, &o.PasswordHash, &o.IsSetup, &o.CreatedAt, &o.UpdatedAt,
			); err != nil {
				return nil, fmt.Errorf("scan owner: %w", err)
			}
			o.Role = RoleOwner
			o.IsActive = true
		}
		list = append(list, &o)
	}
	return list, nil
}

func (s *DBStore) DeleteOwner(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, "DELETE FROM owners WHERE id = $1", id)
	if err != nil {
		return fmt.Errorf("delete owner: %w", err)
	}
	return nil
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
			o.id, o.email, COALESCE(o.username, 'admin'), o.password_hash, o.is_setup, o.created_at, o.updated_at,
			COALESCE(o.name, ''), COALESCE(o.role, 'owner'), COALESCE(o.is_active, true)
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
		&o.Name,
		&o.Role,
		&o.IsActive,
	)
	if err != nil && (strings.Contains(err.Error(), "no such column") || strings.Contains(err.Error(), "does not exist")) {
		err = s.db.QueryRowContext(ctx, `
			SELECT 
				s.token_hash, s.owner_id, s.expires_at, s.created_at,
				o.id, o.email, COALESCE(o.username, 'admin'), o.password_hash, o.is_setup, o.created_at, o.updated_at
			FROM owner_sessions s
			JOIN owners o ON s.owner_id = o.id
			WHERE s.token_hash = $1
		`, tokenHash).Scan(
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
		o.Role = RoleOwner
		o.IsActive = true
	}
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

func (s *DBStore) CreateInvitation(ctx context.Context, inv *UserInvitation) error {
	query := `
		INSERT INTO user_invitations (id, token_hash, email, role, created_by, expires_at, accepted_at, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`
	_, err := s.db.ExecContext(ctx, query,
		inv.ID, inv.TokenHash, inv.Email, inv.Role, inv.CreatedBy,
		inv.ExpiresAt, inv.AcceptedAt, inv.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("create invitation: %w", err)
	}
	return nil
}

func (s *DBStore) GetInvitationByTokenHash(ctx context.Context, tokenHash string) (*UserInvitation, error) {
	query := `
		SELECT id, token_hash, email, role, COALESCE(created_by, ''), expires_at, accepted_at, created_at
		FROM user_invitations
		WHERE token_hash = $1
	`
	var inv UserInvitation
	err := s.db.QueryRowContext(ctx, query, tokenHash).Scan(
		&inv.ID, &inv.TokenHash, &inv.Email, &inv.Role, &inv.CreatedBy,
		&inv.ExpiresAt, &inv.AcceptedAt, &inv.CreatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, ErrInvitationNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get invitation by token hash: %w", err)
	}
	return &inv, nil
}

func (s *DBStore) GetInvitationByID(ctx context.Context, id string) (*UserInvitation, error) {
	query := `
		SELECT id, token_hash, email, role, COALESCE(created_by, ''), expires_at, accepted_at, created_at
		FROM user_invitations
		WHERE id = $1
	`
	var inv UserInvitation
	err := s.db.QueryRowContext(ctx, query, id).Scan(
		&inv.ID, &inv.TokenHash, &inv.Email, &inv.Role, &inv.CreatedBy,
		&inv.ExpiresAt, &inv.AcceptedAt, &inv.CreatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, ErrInvitationNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get invitation by id: %w", err)
	}
	return &inv, nil
}

func (s *DBStore) ListInvitations(ctx context.Context) ([]*UserInvitation, error) {
	query := `
		SELECT id, token_hash, email, role, COALESCE(created_by, ''), expires_at, accepted_at, created_at
		FROM user_invitations
		ORDER BY created_at DESC
	`
	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		if strings.Contains(err.Error(), "no such table") || strings.Contains(err.Error(), "does not exist") {
			return nil, nil
		}
		return nil, fmt.Errorf("list invitations: %w", err)
	}
	defer rows.Close()

	var list []*UserInvitation
	for rows.Next() {
		var inv UserInvitation
		if err := rows.Scan(
			&inv.ID, &inv.TokenHash, &inv.Email, &inv.Role, &inv.CreatedBy,
			&inv.ExpiresAt, &inv.AcceptedAt, &inv.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan invitation: %w", err)
		}
		list = append(list, &inv)
	}
	return list, nil
}

func (s *DBStore) AcceptInvitation(ctx context.Context, id string, acceptedAt time.Time) error {
	query := `UPDATE user_invitations SET accepted_at = $2 WHERE id = $1`
	res, err := s.db.ExecContext(ctx, query, id, acceptedAt)
	if err != nil {
		return fmt.Errorf("accept invitation: %w", err)
	}
	rows, err := res.RowsAffected()
	if err == nil && rows == 0 {
		return ErrInvitationNotFound
	}
	return nil
}

func (s *DBStore) DeleteInvitation(ctx context.Context, id string) error {
	query := `DELETE FROM user_invitations WHERE id = $1`
	_, err := s.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("delete invitation: %w", err)
	}
	return nil
}

func (s *DBStore) UpdateOwnerUsername(ctx context.Context, id, name string) error {
	result, err := s.db.ExecContext(ctx, `UPDATE owners SET username=$2,updated_at=$3 WHERE id=$1`, id, name, time.Now().UTC())
	if err != nil {
		return fmt.Errorf("update username: %w", err)
	}
	rows, err := result.RowsAffected()
	if err == nil && rows == 0 {
		return ErrOwnerNotFound
	}
	return err
}

func (s *DBStore) UpdateOwnerEmail(ctx context.Context, id, email string) error {
	result, err := s.db.ExecContext(ctx, `UPDATE owners SET email=$2,updated_at=$3 WHERE id=$1`, id, email, time.Now().UTC())
	if err != nil {
		return fmt.Errorf("update email: %w", err)
	}
	rows, err := result.RowsAffected()
	if err == nil && rows == 0 {
		return ErrOwnerNotFound
	}
	return err
}
