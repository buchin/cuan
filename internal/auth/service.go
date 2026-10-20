package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/mail"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
)

const (
	SessionDuration = 7 * 24 * time.Hour
	MinPasswordLen  = 8
)

type Service struct {
	store Store
}

func NewService(store Store) *Service {
	return &Service{store: store}
}

func (s *Service) Status(ctx context.Context) (StatusResponse, error) {
	isSetup, err := s.store.IsSetup(ctx)
	if err != nil {
		return StatusResponse{}, fmt.Errorf("check status: %w", err)
	}
	return StatusResponse{
		SetupRequired:   !isSetup,
		OwnerConfigured: isSetup,
	}, nil
}

func (s *Service) Setup(ctx context.Context, email, password string) (*AuthResponse, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	password = strings.TrimSpace(password)

	if !isValidEmail(email) {
		return nil, fmt.Errorf("%w: invalid email address format", ErrInvalidInput)
	}
	if len(password) < MinPasswordLen {
		return nil, fmt.Errorf("%w: password must be at least %d characters", ErrInvalidInput, MinPasswordLen)
	}

	count, err := s.store.CountOwners(ctx)
	if err != nil {
		return nil, fmt.Errorf("count owners: %w", err)
	}
	if count > 0 {
		return nil, ErrSetupAlreadyCompleted
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("hash password: %w", err)
	}

	ownerID, err := generateRandomHex(8)
	if err != nil {
		return nil, fmt.Errorf("generate owner id: %w", err)
	}
	ownerID = "owner_" + ownerID

	username := email
	if parts := strings.Split(email, "@"); len(parts) > 0 {
		username = parts[0]
	}

	now := time.Now().UTC()
	owner := &Owner{
		ID:           ownerID,
		Email:        email,
		Username:     username,
		PasswordHash: string(hash),
		IsSetup:      true,
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	if err := s.store.CreateOwner(ctx, owner); err != nil {
		return nil, fmt.Errorf("persist owner: %w", err)
	}

	// Create initial session
	rawToken, tokenHash, err := generateTokenPair()
	if err != nil {
		return nil, fmt.Errorf("generate session token: %w", err)
	}

	session := &Session{
		TokenHash: tokenHash,
		OwnerID:   owner.ID,
		ExpiresAt: now.Add(SessionDuration),
		CreatedAt: now,
	}

	if err := s.store.CreateSession(ctx, session); err != nil {
		return nil, fmt.Errorf("persist session: %w", err)
	}

	ownerInfo := OwnerInfo{
		ID:       owner.ID,
		Username: owner.Username,
		Email:    owner.Email,
	}

	return &AuthResponse{
		Token: rawToken,
		Owner: ownerInfo,
		User:  ownerInfo,
	}, nil
}

func (s *Service) Login(ctx context.Context, identifier, password string) (*AuthResponse, error) {
	identifier = strings.TrimSpace(identifier)
	password = strings.TrimSpace(password)

	if identifier == "" || password == "" {
		return nil, fmt.Errorf("%w: identifier and password are required", ErrInvalidInput)
	}

	owner, err := s.store.GetOwner(ctx)
	if err != nil {
		if err == ErrOwnerNotFound {
			return nil, ErrSetupRequired
		}
		return nil, fmt.Errorf("retrieve owner: %w", err)
	}

	match := strings.EqualFold(owner.Email, identifier) ||
		(owner.Username != "" && strings.EqualFold(owner.Username, identifier))
	if !match {
		return nil, ErrInvalidCredentials
	}

	if err := bcrypt.CompareHashAndPassword([]byte(owner.PasswordHash), []byte(password)); err != nil {
		return nil, ErrInvalidCredentials
	}

	rawToken, tokenHash, err := generateTokenPair()
	if err != nil {
		return nil, fmt.Errorf("generate session token: %w", err)
	}

	now := time.Now().UTC()
	session := &Session{
		TokenHash: tokenHash,
		OwnerID:   owner.ID,
		ExpiresAt: now.Add(SessionDuration),
		CreatedAt: now,
	}

	if err := s.store.CreateSession(ctx, session); err != nil {
		return nil, fmt.Errorf("persist session: %w", err)
	}

	ownerInfo := OwnerInfo{
		ID:       owner.ID,
		Username: owner.Username,
		Email:    owner.Email,
	}

	return &AuthResponse{
		Token: rawToken,
		Owner: ownerInfo,
		User:  ownerInfo,
	}, nil
}

func (s *Service) ChangePassword(ctx context.Context, ownerID, currentPassword, newPassword string) error {
	currentPassword = strings.TrimSpace(currentPassword)
	newPassword = strings.TrimSpace(newPassword)

	if currentPassword == "" || newPassword == "" {
		return fmt.Errorf("%w: current and new passwords are required", ErrInvalidInput)
	}
	if len(newPassword) < MinPasswordLen {
		return fmt.Errorf("%w: password must be at least %d characters", ErrInvalidInput, MinPasswordLen)
	}

	owner, err := s.store.GetOwner(ctx)
	if err != nil {
		if err == ErrOwnerNotFound {
			return ErrOwnerNotFound
		}
		return fmt.Errorf("retrieve owner: %w", err)
	}

	if ownerID != "" && owner.ID != ownerID {
		return ErrUnauthorized
	}

	if err := bcrypt.CompareHashAndPassword([]byte(owner.PasswordHash), []byte(currentPassword)); err != nil {
		return ErrInvalidCredentials
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("hash password: %w", err)
	}

	if err := s.store.UpdateOwnerPassword(ctx, owner.ID, string(hash)); err != nil {
		return fmt.Errorf("update owner password: %w", err)
	}

	return nil
}

func (s *Service) AutoSeedDefaultAdmin(ctx context.Context) error {
	count, err := s.store.CountOwners(ctx)
	if err != nil {
		return fmt.Errorf("count owners: %w", err)
	}
	if count > 0 {
		return nil
	}

	hash, err := bcrypt.GenerateFromPassword([]byte("whosyourdaddy"), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("hash default password: %w", err)
	}

	now := time.Now().UTC()
	owner := &Owner{
		ID:           "owner_admin",
		Email:        "admin@cuan.local",
		Username:     "admin",
		PasswordHash: string(hash),
		IsSetup:      true,
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	return s.store.CreateOwner(ctx, owner)
}

func (s *Service) ValidateToken(ctx context.Context, rawToken string) (*Owner, error) {
	rawToken = strings.TrimSpace(rawToken)
	if rawToken == "" {
		return nil, ErrUnauthorized
	}

	tokenHash := HashToken(rawToken)
	sess, owner, err := s.store.GetSession(ctx, tokenHash)
	if err != nil {
		return nil, ErrUnauthorized
	}

	if time.Now().UTC().After(sess.ExpiresAt) {
		_ = s.store.DeleteSession(ctx, tokenHash)
		return nil, ErrUnauthorized
	}

	return owner, nil
}

func (s *Service) Logout(ctx context.Context, rawToken string) error {
	rawToken = strings.TrimSpace(rawToken)
	if rawToken == "" {
		return nil
	}

	tokenHash := HashToken(rawToken)
	return s.store.DeleteSession(ctx, tokenHash)
}

func HashToken(token string) string {
	h := sha256.Sum256([]byte(token))
	return hex.EncodeToString(h[:])
}

func generateTokenPair() (rawToken string, tokenHash string, err error) {
	raw, err := generateRandomHex(32)
	if err != nil {
		return "", "", err
	}
	return raw, HashToken(raw), nil
}

func generateRandomHex(bytesCount int) (string, error) {
	b := make([]byte, bytesCount)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func isValidEmail(email string) bool {
	if len(email) < 3 || len(email) > 254 {
		return false
	}
	_, err := mail.ParseAddress(email)
	if err != nil {
		return false
	}
	parts := strings.Split(email, "@")
	return len(parts) == 2 && strings.Contains(parts[1], ".")
}

// UpdateUsername preserves account IDs, credentials and sessions.
func (s *Service) UpdateUsername(ctx context.Context, ownerID, username, password string) error {
	username = strings.ToLower(strings.TrimSpace(username))
	if len(username) < 3 || len(username) > 32 {
		return fmt.Errorf("%w: username must contain 3–32 letters, numbers, dots, underscores or hyphens", ErrInvalidInput)
	}
	for _, r := range username {
		if !(r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '.' || r == '_' || r == '-') {
			return fmt.Errorf("%w: username must contain letters, numbers, dots, underscores or hyphens", ErrInvalidInput)
		}
	}
	owner, err := s.store.GetOwner(ctx)
	if err != nil {
		return err
	}
	if owner.ID != ownerID {
		return ErrUnauthorized
	}
	if bcrypt.CompareHashAndPassword([]byte(owner.PasswordHash), []byte(strings.TrimSpace(password))) != nil {
		return ErrInvalidCredentials
	}
	store, ok := s.store.(interface {
		UpdateOwnerUsername(context.Context, string, string) error
	})
	if !ok {
		return fmt.Errorf("username editing is unavailable")
	}
	return store.UpdateOwnerUsername(ctx, ownerID, username)
}

func (s *Service) OwnerID(ctx context.Context) (string, error) {
	owner, err := s.store.GetOwner(ctx)
	if err != nil {
		return "", err
	}
	return owner.ID, nil
}

// OwnerIdentity returns only public identity fields, never password/session data.
func (s *Service) OwnerIdentity(ctx context.Context) (OwnerInfo, error) {
	owner, err := s.store.GetOwner(ctx)
	if err != nil {
		return OwnerInfo{}, err
	}
	return OwnerInfo{ID: owner.ID, Username: owner.Username, Email: owner.Email}, nil
}
