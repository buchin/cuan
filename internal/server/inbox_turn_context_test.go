package server

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/buchin/cuan/internal/db"
	"github.com/buchin/cuan/internal/project"
	cuanPrompt "github.com/buchin/cuan/internal/prompt"
)

func TestReviewTurnUsesBoundSnapshotAndRepositoryWithoutConversationInspection(t *testing.T) {
	s, d, q := stalledReviewFixture(t, "queued")
	ctx := context.Background()
	root := t.TempDir()
	guidance := "REQUIRED_BOUND_REPOSITORY_GUIDANCE"
	if err := os.WriteFile(filepath.Join(root, "AGENTS.md"), []byte(guidance), 0600); err != nil {
		t.Fatal(err)
	}
	projects := project.NewMemoryStore()
	if err := projects.CreateProject(ctx, &project.Project{ID: d.ProjectID, Name: "Bound project", LocalPath: root}); err != nil {
		t.Fatal(err)
	}
	s.projectSvc = project.NewService(projects, nil, "", t.TempDir())
	probe := filepath.Join(t.TempDir(), "git-calls")
	bin := t.TempDir()
	if err := os.WriteFile(filepath.Join(bin, "git"), []byte("#!/bin/sh\nprintf call >> \"$CUAN_REVIEW_GIT_PROBE\"\nexit 1\n"), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("CUAN_REVIEW_GIT_PROBE", probe)
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	// Unrelated history must not be read into an automatic result review.
	if err := s.projectStudioRepo.CreateMessage(ctx, &db.ProjectMessage{ID: db.GenerateUUID(), RoomID: q.RoomID, AuthorType: "agent", Content: "UNRELATED_CHAT_HISTORY"}); err != nil {
		t.Fatal(err)
	}
	turn, err := s.prepareInboxTurnContext(ctx, q)
	if err != nil || !turn.Review || turn.Workspace != root {
		t.Fatal("lost native snapshot or exact project root", turn, err)
	}
	if strings.Contains(turn.Prompt, "Conversation history") || strings.Contains(turn.Prompt, "UNRELATED_CHAT_HISTORY") || strings.Contains(turn.Prompt, "Git Working State") {
		t.Fatal("review replayed conversational project analysis")
	}
	if !strings.Contains(turn.Prompt, "original_request") || !strings.Contains(turn.Prompt, "acceptance_checklist") || !strings.Contains(cuanPrompt.GetReviewPromptAdditions(turn.Workspace), guidance) {
		t.Fatal("review lost original acceptance or repository constraints")
	}
	if _, err := os.Stat(probe); !os.IsNotExist(err) {
		t.Fatal("snapshot review ran disposable project Git inspection", err)
	}
	q.MessageID = db.GenerateUUID()
	if err := s.projectStudioRepo.CreateMessage(ctx, &db.ProjectMessage{ID: q.MessageID, RoomID: q.RoomID, AuthorType: "user", AuthorID: "cuan_work_review", Content: "Review /tasks/" + d.ID}); err != nil {
		t.Fatal(err)
	}
	turn, err = s.prepareInboxTurnContext(ctx, q)
	if err != nil || turn.Review || !strings.Contains(turn.Prompt, "Conversation history") {
		t.Fatal("user text spoofed snapshot review", err)
	}
}
