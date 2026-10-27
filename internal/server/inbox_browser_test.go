package server

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/buchin/cuan/internal/db"
	"github.com/buchin/cuan/internal/project"
)

// Explicit opt-in preview with fake agents, temporary HOME and in-memory data.
// It is never a provider-success test and never loads production configuration.
func TestInboxLifecycleBrowserPreview(t *testing.T) {
	port := os.Getenv("CUAN_INBOX_FIXTURE_PORT")
	if port == "" {
		t.Skip("opt-in browser fixture")
	}
	s, _, _, home := inboxFixture(t)
	bin := strings.Split(os.Getenv("PATH"), ":")[0]
	os.WriteFile(filepath.Join(bin, "agy"), []byte("#!/bin/sh\nsleep 8\nprintf 'Demo fixture: pesan diproses. Ini bukan respons provider nyata.'\n"), 0700)
	s.projectSvc.Store().CreateProject(context.Background(), &project.Project{ID: "fixture-project", Name: "Demo project (fixture)", ProjectType: "blank", CreatedAt: time.Now(), UpdatedAt: time.Now()})
	resolved := s.resolveConversationContext(context.Background(), "agent_lifecycle", "")
	now := time.Now().UTC()
	s.projectStudioRepo.CreateMessage(context.Background(), &db.ProjectMessage{ID: db.GenerateUUID(), RoomID: resolved.RoomID, AuthorType: "agent", AuthorName: "@lifecycle", Content: "Demo terisolasi. Semua respons adalah fixture, tanpa koneksi provider atau data production. Kirim pesan untuk melihat antrean dan progres, atau buat task dari pesan.", CreatedAt: now, UpdatedAt: now})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	s.StartInboxDispatcher(ctx)
	fmt.Printf("Fixture preview on port %s; HOME temporary; no real provider\n", port)
	_ = home
	if e := http.ListenAndServe(":"+port, s.Router()); e != nil {
		t.Fatal(e)
	}
}
