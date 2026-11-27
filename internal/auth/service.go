package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
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
		Role:         RoleOwner,
		IsActive:     true,
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
		Name:     owner.Name,
		Role:     owner.Role,
		IsActive: owner.IsActive,
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

	owner, err := s.store.GetOwnerByEmailOrUsername(ctx, identifier)
	if err != nil {
		if errors.Is(err, ErrOwnerNotFound) {
			isSetup, _ := s.store.IsSetup(ctx)
			if !isSetup {
				return nil, ErrSetupRequired
			}
			return nil, ErrInvalidCredentials
		}
		return nil, fmt.Errorf("retrieve user: %w", err)
	}

	if !owner.IsActive {
		return nil, ErrAccountDeactivated
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
		Name:     owner.Name,
		Role:     owner.Role,
		IsActive: owner.IsActive,
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

	var owner *Owner
	var err error
	if ownerID != "" {
		owner, err = s.store.GetOwnerByID(ctx, ownerID)
	} else {
		owner, err = s.store.GetOwner(ctx)
	}
	if err != nil {
		if errors.Is(err, ErrOwnerNotFound) {
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
		Role:         RoleOwner,
		IsActive:     true,
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

	if !owner.IsActive {
		return nil, ErrUnauthorized
	}

	return owner, nil
}

// IssueEphemeralSession creates a short-lived session token scoped to an existing active user.
func (s *Service) IssueEphemeralSession(ctx context.Context, ownerID string, ttl time.Duration) (string, error) {
	ownerID = strings.TrimSpace(ownerID)
	if ownerID == "" {
		return "", ErrInvalidInput
	}
	if ttl <= 0 {
		ttl = 20 * time.Minute
	}
	owner, err := s.store.GetOwnerByID(ctx, ownerID)
	if err != nil {
		return "", err
	}
	if !owner.IsActive {
		return "", ErrAccountDeactivated
	}
	rawToken, tokenHash, err := generateTokenPair()
	if err != nil {
		return "", fmt.Errorf("generate session token: %w", err)
	}
	now := time.Now().UTC()
	session := &Session{
		TokenHash: tokenHash,
		OwnerID:   owner.ID,
		ExpiresAt: now.Add(ttl),
		CreatedAt: now,
	}
	if err := s.store.CreateSession(ctx, session); err != nil {
		return "", fmt.Errorf("persist session: %w", err)
	}
	return rawToken, nil
}

func (s *Service) Logout(ctx context.Context, rawToken string) error {
	rawToken = strings.TrimSpace(rawToken)
	if rawToken == "" {
		return nil
	}

	tokenHash := HashToken(rawToken)
	return s.store.DeleteSession(ctx, tokenHash)
}

// CreateInvite creates a new invitation for staff members (Owner only).
func (s *Service) CreateInvite(ctx context.Context, creatorID, email, role string) (*CreateInviteResponse, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	if !isValidEmail(email) {
		return nil, fmt.Errorf("%w: invalid email address format", ErrInvalidInput)
	}

	role = strings.ToLower(strings.TrimSpace(role))
	if role == "" {
		role = RoleMember
	}
	if role != RoleOwner && role != RoleMember {
		return nil, fmt.Errorf("%w: role must be either owner or member", ErrInvalidInput)
	}

	// Check if user already exists
	if _, err := s.store.GetOwnerByEmailOrUsername(ctx, email); err == nil {
		return nil, ErrUserAlreadyExists
	}

	rawToken, tokenHash, err := generateTokenPair()
	if err != nil {
		return nil, fmt.Errorf("generate invite token: %w", err)
	}

	inviteID, err := generateRandomHex(8)
	if err != nil {
		return nil, fmt.Errorf("generate invite id: %w", err)
	}
	inviteID = "inv_" + inviteID

	now := time.Now().UTC()
	inv := &UserInvitation{
		ID:        inviteID,
		TokenHash: tokenHash,
		Email:     email,
		Role:      role,
		CreatedBy: creatorID,
		ExpiresAt: now.Add(7 * 24 * time.Hour), // 7 days expiry
		CreatedAt: now,
	}

	if err := s.store.CreateInvitation(ctx, inv); err != nil {
		return nil, fmt.Errorf("persist invitation: %w", err)
	}

	return &CreateInviteResponse{
		Status:     "success",
		Invitation: *inv,
		Token:      rawToken,
		InviteURL:  "/invite/accept?token=" + rawToken,
	}, nil
}

// VerifyInvite checks if an invitation token is valid, unexpired, and unaccepted.
func (s *Service) VerifyInvite(ctx context.Context, rawToken string) (*UserInvitation, error) {
	rawToken = strings.TrimSpace(rawToken)
	if rawToken == "" {
		return nil, fmt.Errorf("%w: invitation token is required", ErrInvalidInput)
	}

	tokenHash := HashToken(rawToken)
	inv, err := s.store.GetInvitationByTokenHash(ctx, tokenHash)
	if err != nil {
		return nil, ErrInvitationNotFound
	}

	if inv.AcceptedAt != nil {
		return nil, ErrInvitationAccepted
	}

	if time.Now().UTC().After(inv.ExpiresAt) {
		return nil, ErrInvitationExpired
	}

	return inv, nil
}

// AcceptInvite activates a staff account via invitation token and logs them in.
func (s *Service) AcceptInvite(ctx context.Context, rawToken, name, password string) (*AuthResponse, error) {
	inv, err := s.VerifyInvite(ctx, rawToken)
	if err != nil {
		return nil, err
	}

	name = strings.TrimSpace(name)
	password = strings.TrimSpace(password)
	if len(password) < MinPasswordLen {
		return nil, fmt.Errorf("%w: password must be at least %d characters", ErrInvalidInput, MinPasswordLen)
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("hash password: %w", err)
	}

	userID, err := generateRandomHex(8)
	if err != nil {
		return nil, fmt.Errorf("generate user id: %w", err)
	}
	userID = "user_" + userID

	username := inv.Email
	if parts := strings.Split(inv.Email, "@"); len(parts) > 0 {
		username = parts[0]
	}
	if name != "" {
		cleanName := strings.ToLower(strings.ReplaceAll(name, " ", "_"))
		if len(cleanName) >= 3 {
			username = cleanName
		}
	}

	now := time.Now().UTC()
	user := &Owner{
		ID:           userID,
		Email:        inv.Email,
		Username:     username,
		PasswordHash: string(hash),
		IsSetup:      true,
		Name:         name,
		Role:         inv.Role,
		IsActive:     true,
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	if err := s.store.CreateUser(ctx, user); err != nil {
		return nil, fmt.Errorf("persist staff user: %w", err)
	}

	// Mark invitation accepted
	_ = s.store.AcceptInvitation(ctx, inv.ID, now)

	// Create initial session for the new staff member
	rawSessionToken, tokenHash, err := generateTokenPair()
	if err != nil {
		return nil, fmt.Errorf("generate session token: %w", err)
	}

	session := &Session{
		TokenHash: tokenHash,
		OwnerID:   user.ID,
		ExpiresAt: now.Add(SessionDuration),
		CreatedAt: now,
	}

	if err := s.store.CreateSession(ctx, session); err != nil {
		return nil, fmt.Errorf("persist session: %w", err)
	}

	userInfo := OwnerInfo{
		ID:       user.ID,
		Username: user.Username,
		Email:    user.Email,
		Name:     user.Name,
		Role:     user.Role,
		IsActive: user.IsActive,
	}

	return &AuthResponse{
		Token: rawSessionToken,
		Owner: userInfo,
		User:  userInfo,
	}, nil
}

// ListTeamMembers returns human staff members and pending invitations.
func (s *Service) ListTeamMembers(ctx context.Context) ([]TeamMemberDTO, []UserInvitation, error) {
	owners, err := s.store.ListOwners(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("list owners: %w", err)
	}

	invitations, err := s.store.ListInvitations(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("list invitations: %w", err)
	}

	members := make([]TeamMemberDTO, 0, len(owners))
	for _, o := range owners {
		status := "active"
		if !o.IsActive {
			status = "inactive"
		}
		role := o.Role
		if role == "" {
			role = RoleOwner
		}
		members = append(members, TeamMemberDTO{
			ID:        o.ID,
			Name:      o.Name,
			Username:  o.Username,
			Email:     o.Email,
			Role:      role,
			IsActive:  o.IsActive,
			Status:    status,
			CreatedAt: o.CreatedAt,
		})
	}

	// Filter pending invitations (unaccepted & unexpired)
	now := time.Now().UTC()
	pending := make([]UserInvitation, 0, len(invitations))
	for _, inv := range invitations {
		if inv.AcceptedAt == nil && inv.ExpiresAt.After(now) {
			pending = append(pending, *inv)
		}
	}

	return members, pending, nil
}

// DeleteInvite removes an invitation.
func (s *Service) DeleteInvite(ctx context.Context, inviteID string) error {
	return s.store.DeleteInvitation(ctx, inviteID)
}

// DeleteMember removes a team member.
func (s *Service) DeleteMember(ctx context.Context, memberID string) error {
	return s.store.DeleteOwner(ctx, memberID)
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
	var owner *Owner
	var err error
	if ownerID != "" {
		owner, err = s.store.GetOwnerByID(ctx, ownerID)
	} else {
		owner, err = s.store.GetOwner(ctx)
	}
	if err != nil {
		return err
	}
	if ownerID != "" && owner.ID != ownerID {
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
	return store.UpdateOwnerUsername(ctx, owner.ID, username)
}

// UpdateEmail preserves account IDs, credentials and sessions, updating sign-in email.
func (s *Service) UpdateEmail(ctx context.Context, ownerID, email, password string) error {
	email = strings.ToLower(strings.TrimSpace(email))
	if !isValidEmail(email) {
		return fmt.Errorf("%w: invalid email address format", ErrInvalidInput)
	}
	var owner *Owner
	var err error
	if ownerID != "" {
		owner, err = s.store.GetOwnerByID(ctx, ownerID)
	} else {
		owner, err = s.store.GetOwner(ctx)
	}
	if err != nil {
		return err
	}
	if ownerID != "" && owner.ID != ownerID {
		return ErrUnauthorized
	}
	if bcrypt.CompareHashAndPassword([]byte(owner.PasswordHash), []byte(strings.TrimSpace(password))) != nil {
		return ErrInvalidCredentials
	}
	store, ok := s.store.(interface {
		UpdateOwnerEmail(context.Context, string, string) error
	})
	if !ok {
		return fmt.Errorf("email editing is unavailable")
	}
	return store.UpdateOwnerEmail(ctx, owner.ID, email)
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
	return OwnerInfo{
		ID:       owner.ID,
		Username: owner.Username,
		Email:    owner.Email,
		Name:     owner.Name,
		Role:     owner.Role,
		IsActive: owner.IsActive,
	}, nil
}
