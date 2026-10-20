package project

import (
	"context"
	"github.com/buchin/cuan/internal/crypto"
	"testing"
)

func TestSharedAccessRotationAndLegacy(t *testing.T) {
	s := NewService(NewMemoryStore(), nil, "fixture-secret", t.TempDir())
	token := "first"
	s.SetSharedRepositoryAccess(func(context.Context) (string, error) { return token, nil })
	got, err := s.repositoryToken(context.Background(), "")
	if err != nil || got != "first" {
		t.Fatal("shared access missing")
	}
	token = "rotated"
	got, err = s.repositoryToken(context.Background(), "")
	if err != nil || got != "rotated" {
		t.Fatal("rotation ignored")
	}
	legacy, _ := crypto.Encrypt("legacy", "fixture-secret")
	got, err = s.repositoryToken(context.Background(), legacy)
	if err != nil || got != "legacy" {
		t.Fatal("explicit project access overwritten")
	}
	token = ""
	got, err = s.repositoryToken(context.Background(), "")
	if err != nil || got != "" {
		t.Fatal("disconnection ignored")
	}
}
