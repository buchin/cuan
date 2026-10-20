package task

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"time"
)

// TaskLaunch is durable intent to START an authorized task, not a task attempt.
// A route preflight can be repeated; a task attempt is still deduplicated by its
// original request ID. This record never authorizes scope by itself.
type TaskLaunch struct {
	ID, TaskID, RequestID, OwnerID, RoomID, SourceRequestID, AgentID string
	Revision, InstructionsHash, State, LeaseToken, SafeError         string
	AvailableAt, ExpiresAt, CreatedAt, UpdatedAt                     time.Time
	LeaseUntil                                                       *time.Time
	Checks                                                           int
}

var ErrLaunchLease = errors.New("launch lease is no longer owned")

type LaunchStore interface {
	SaveLaunch(context.Context, TaskLaunch) (*TaskLaunch, error)
	ClaimLaunch(context.Context, time.Time, string) (*TaskLaunch, error)
	FinishLaunch(context.Context, string, string, string, time.Time, string) error
	CancelLaunches(context.Context, string, time.Time) (int64, error)
	GetLaunch(context.Context, string) (*TaskLaunch, error)
}

func LaunchID(taskID, requestID string) string {
	return fmt.Sprintf("launch_%x", sha256.Sum256([]byte(taskID+"\x00"+requestID)))
}
func sameLaunch(a, b TaskLaunch) bool {
	return a.ID == b.ID && a.TaskID == b.TaskID && a.RequestID == b.RequestID && a.OwnerID == b.OwnerID && a.RoomID == b.RoomID && a.SourceRequestID == b.SourceRequestID && a.AgentID == b.AgentID && a.Revision == b.Revision && a.InstructionsHash == b.InstructionsHash
}
func normalizeLaunch(j TaskLaunch) (TaskLaunch, error) {
	if j.CreatedAt.IsZero() || j.AvailableAt.Before(j.CreatedAt) || !j.ExpiresAt.After(j.AvailableAt) || j.TaskID == "" || j.RequestID == "" || j.OwnerID == "" || j.RoomID == "" || j.SourceRequestID == "" || j.AgentID == "" || j.InstructionsHash == "" || len([]rune(j.Revision)) > 6000 || !j.ExpiresAt.After(j.CreatedAt) {
		return j, ErrInvalidTaskInput
	}
	j.ID = LaunchID(j.TaskID, j.RequestID)
	j.State = "queued"
	j.LeaseToken = ""
	j.LeaseUntil = nil
	j.Checks = 0
	j.SafeError = ""
	j.UpdatedAt = j.CreatedAt
	return j, nil
}
func launchStore(s *Service) (LaunchStore, error) {
	store, ok := s.store.(LaunchStore)
	if !ok {
		return nil, errors.New("durable task launch storage is unavailable")
	}
	return store, nil
}
func (s *Service) SaveLaunch(ctx context.Context, j TaskLaunch) (*TaskLaunch, error) {
	store, e := launchStore(s)
	if e != nil {
		return nil, e
	}
	return store.SaveLaunch(ctx, j)
}
func (s *Service) ClaimLaunch(ctx context.Context, now time.Time, token string) (*TaskLaunch, error) {
	store, e := launchStore(s)
	if e != nil {
		return nil, e
	}
	return store.ClaimLaunch(ctx, now, token)
}
func (s *Service) FinishLaunch(ctx context.Context, id, token, state string, next time.Time, safeError string) error {
	store, e := launchStore(s)
	if e != nil {
		return e
	}
	return store.FinishLaunch(ctx, id, token, state, next, safeError)
}
func (s *Service) CancelLaunches(ctx context.Context, taskID string, now time.Time) (int64, error) {
	store, e := launchStore(s)
	if e != nil {
		return 0, e
	}
	return store.CancelLaunches(ctx, taskID, now)
}
func (s *Service) GetLaunch(ctx context.Context, id string) (*TaskLaunch, error) {
	store, e := launchStore(s)
	if e != nil {
		return nil, e
	}
	return store.GetLaunch(ctx, id)
}
func validLaunchFinish(state string) bool {
	return state == "queued" || state == "started" || state == "stopped"
}
