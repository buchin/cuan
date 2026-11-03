package project

import (
	"context"
	"testing"
)

func TestTaskPushForbiddenBeforeRepositoryOrCredentials(t *testing.T) {
	s := &Service{}
	if _, err := s.PushTaskBranch(context.Background(), "task", "project", "Jangan push atau deploy."); err == nil {
		t.Fatal("forbidden push reached repository access")
	}
}
