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

	now := time.Now().UTC()
	owner := &Owner{
		ID:           ownerID,
		Email:        email,
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

	return &AuthResponse{
		Token: rawToken,
		Owner: OwnerInfo{
			ID:    owner.ID,
			Email: owner.Email,
		},
	}, nil
}

func (s *Service) Login(ctx context.Context, email, password string) (*AuthResponse, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	password = strings.TrimSpace(password)

	if email == "" || password == "" {
		return nil, fmt.Errorf("%w: email and password are required", ErrInvalidInput)
	}

	owner, err := s.store.GetOwner(ctx)
	if err != nil {
		if err == ErrOwnerNotFound {
			return nil, ErrSetupRequired
		}
		return nil, fmt.Errorf("retrieve owner: %w", err)
	}

	if owner.Email != email {
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

	return &AuthResponse{
		Token: rawToken,
		Owner: OwnerInfo{
			ID:    owner.ID,
			Email: owner.Email,
		},
	}, nil
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
