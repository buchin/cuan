package auth

import (
	"context"
	"errors"
	"time"
)

var (
	ErrSetupAlreadyCompleted = errors.New("setup already completed: single owner already exists")
	ErrSetupRequired         = errors.New("setup required: no owner account exists")
	ErrInvalidCredentials    = errors.New("invalid email or password")
	ErrUnauthorized          = errors.New("unauthorized: invalid or expired session")
	ErrOwnerNotFound         = errors.New("owner not found")
	ErrInvalidInput          = errors.New("invalid input")
	ErrInvitationNotFound    = errors.New("invitation not found or invalid")
	ErrInvitationExpired     = errors.New("invitation has expired")
	ErrInvitationAccepted    = errors.New("invitation has already been accepted")
	ErrUserAlreadyExists     = errors.New("user with this email already exists")
	ErrForbidden             = errors.New("forbidden: insufficient permissions")
	ErrAccountDeactivated    = errors.New("account is deactivated")
)

type Store interface {
	IsSetup(ctx context.Context) (bool, error)
	CountOwners(ctx context.Context) (int, error)
	CreateOwner(ctx context.Context, owner *Owner) error
	GetOwner(ctx context.Context) (*Owner, error)
	UpdateOwnerPassword(ctx context.Context, ownerID string, passwordHash string) error
	CreateSession(ctx context.Context, session *Session) error
	GetSession(ctx context.Context, tokenHash string) (*Session, *Owner, error)
	DeleteSession(ctx context.Context, tokenHash string) error
	DeleteExpiredSessions(ctx context.Context) error

	// Multi-User and Invitations
	CreateUser(ctx context.Context, user *Owner) error
	GetOwnerByID(ctx context.Context, id string) (*Owner, error)
	GetOwnerByEmailOrUsername(ctx context.Context, identifier string) (*Owner, error)
	ListOwners(ctx context.Context) ([]*Owner, error)
	DeleteOwner(ctx context.Context, id string) error

	CreateInvitation(ctx context.Context, inv *UserInvitation) error
	GetInvitationByTokenHash(ctx context.Context, tokenHash string) (*UserInvitation, error)
	GetInvitationByID(ctx context.Context, id string) (*UserInvitation, error)
	ListInvitations(ctx context.Context) ([]*UserInvitation, error)
	AcceptInvitation(ctx context.Context, id string, acceptedAt time.Time) error
	DeleteInvitation(ctx context.Context, id string) error
}
