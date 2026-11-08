package server

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/buchin/cuan/internal/db"
	"github.com/buchin/cuan/internal/diagnostics"
)

func TestInboxCRMSnapshotsArePerTurnAndDoNotBorrowProfileState(t *testing.T) {
	s, _ := setupTestChatServer(t)
	ctx := context.Background()
	c, err := s.crmUpsert(ctx, "one", "Dojo", "One", "one@example.invalid")
	if err != nil {
		t.Fatal(err)
	}
	other, err := s.crmUpsert(ctx, "two", "Dojo", "Two", "two@example.invalid")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.memorySave(ctx, ContactMemory{ID: db.GenerateUUID(), ContactID: c.ID, Kind: "fact", Body: "recorded fact", CreatedAt: time.Now(), UpdatedAt: time.Now()}); err != nil {
		t.Fatal(err)
	}
	prepare := func(room, request string) (string, func()) {
		ctx := context.WithValue(ctx, tasteMemberKey{}, "owner")
		ctx = context.WithValue(ctx, inboxRequestTraceKey{}, inboxRequestTrace{RoomID: room, RequestID: request, AgentID: "agent"})
		path, cleanup, err := s.prepareInboxCRMSnapshot(ctx)
		if err != nil {
			t.Fatal(err)
		}
		return path, cleanup
	}
	first, cleanup := prepare(c.SessionUUID, "one")
	defer cleanup()
	second, cleanup2 := prepare(other.SessionUUID, "two")
	defer cleanup2()
	if first == second || filepath.Dir(first) == filepath.Dir(second) {
		t.Fatal("snapshots shared across requests")
	}
	for path, id := range map[string]string{first: c.ID, second: other.ID} {
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		var snapshot diagnostics.InboxCRMSnapshot
		if json.Unmarshal(raw, &snapshot) != nil || snapshot.CurrentContactID != id || snapshot.OwnerID != "owner" {
			t.Fatal("identity binding lost")
		}
		if id == c.ID && !strings.Contains(string(snapshot.Memories[id]), "recorded fact") {
			t.Fatal("current notes not fetched")
		}
		info, _ := os.Stat(path)
		if info.Mode().Perm() != 0600 {
			t.Fatal("snapshot not private")
		}
	}
	// Only this snapshot is mounted; the second conversation and native profile
	// directory are not mounted as CRM data. No native Owner configuration needed.
	home := t.TempDir()
	args, err := inboxSandboxArgs("/bin/sh", home, "", nil, first)
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(args, " ")
	if !strings.Contains(joined, "--ro-bind "+first+" /cuan-inbox-crm.json") || strings.Contains(joined, second) {
		t.Fatal("CRM mount not per execution")
	}
	config, err := prepareInboxMCPBoundary(home, true)
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := os.ReadFile(config)
	if !strings.Contains(string(raw), "--inbox-crm") || strings.Contains(string(raw), "auth_token") {
		t.Fatal("wrong CRM discovery")
	}
	cleanup()
	if _, err := os.Stat(first); !os.IsNotExist(err) {
		t.Fatal("snapshot retained")
	}
	if _, err := os.Stat(second); err != nil {
		t.Fatal("cleanup touched another turn")
	}
}
