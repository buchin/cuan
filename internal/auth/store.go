package auth

import (
	"context"
	"errors"
)

var (
	ErrSetupAlreadyCompleted = errors.New("setup already completed: single owner already exists")
	ErrSetupRequired         = errors.New("setup required: no owner account exists")
	ErrInvalidCredentials    = errors.New("invalid email or password")
	ErrUnauthorized          = errors.New("unauthorized: invalid or expired session")
	ErrOwnerNotFound         = errors.New("owner not found")
	ErrInvalidInput          = errors.New("invalid input")
)

type Store interface {
	IsSetup(ctx context.Context) (bool, error)
	CountOwners(ctx context.Context) (int, error)
	CreateOwner(ctx context.Context, owner *Owner) error
	GetOwner(ctx context.Context) (*Owner, error)
	CreateSession(ctx context.Context, session *Session) error
	GetSession(ctx context.Context, tokenHash string) (*Session, *Owner, error)
	DeleteSession(ctx context.Context, tokenHash string) error
	DeleteExpiredSessions(ctx context.Context) error
}
