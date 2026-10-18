package agent

import (
	"context"
	"testing"
	"time"
)

func TestDetectAgentBinaries(t *testing.T) {
	bins := DetectAgentBinaries()
	if len(bins) == 0 {
		t.Fatalf("expected at least standard shell candidates in detection list")
	}

	foundBashOrSh := false
	for _, b := range bins {
		if (b.Name == "bash" || b.Name == "sh") && b.Available {
			foundBashOrSh = true
			if b.Path == "" {
				t.Errorf("expected non-empty path for available binary %s", b.Name)
			}
		}
	}

	if !foundBashOrSh {
		t.Errorf("expected bash or sh to be available on host system")
	}
}

func TestSessionStore_CreateAndGet(t *testing.T) {
	ctx := context.Background()
	store := NewMemorySessionStore()

	sess := &AgentSession{
		ID:         "asess_test1",
		MemberID:   "buchin",
		AgentID:    "worker-flash",
		AgentName:  "Antigravity Flash",
		Provider:   "antigravity",
		BinaryType: "agy",
		DeviceCode: "CUAN-1234",
	}

	if err := store.CreateSession(ctx, sess); err != nil {
		t.Fatalf("failed to create session: %v", err)
	}

	// Retrieve by ID
	got, err := store.GetSession(ctx, "asess_test1")
	if err != nil {
		t.Fatalf("failed to get session: %v", err)
	}
	if got.MemberID != "buchin" || got.AgentID != "worker-flash" {
		t.Errorf("unexpected session data: %+v", got)
	}
	if got.Token == "" {
		t.Errorf("expected generated token")
	}

	// Retrieve by Token
	gotByToken, err := store.GetSessionByToken(ctx, got.Token)
	if err != nil {
		t.Fatalf("failed to get session by token: %v", err)
	}
	if gotByToken.ID != sess.ID {
		t.Errorf("expected matching ID: %s vs %s", gotByToken.ID, sess.ID)
	}

	// Retrieve by DeviceCode
	gotByCode, err := store.GetSessionByDeviceCode(ctx, "CUAN-1234")
	if err != nil {
		t.Fatalf("failed to get session by device code: %v", err)
	}
	if gotByCode.ID != sess.ID {
		t.Errorf("expected matching ID: %s vs %s", gotByCode.ID, sess.ID)
	}
}

func TestSessionStore_AuthorizeDevicePairing(t *testing.T) {
	ctx := context.Background()
	store := NewMemorySessionStore()

	req := AuthorizePairingRequest{
		DeviceCode: "CUAN-8892",
		MemberID:   "po-buchin",
		AgentID:    "sm-agent",
		AgentName:  "Scrum Master Agent",
		Provider:   "antigravity",
		BinaryType: "agy",
	}

	sess, err := store.AuthorizeDevicePairing(ctx, req)
	if err != nil {
		t.Fatalf("AuthorizeDevicePairing failed: %v", err)
	}
	if sess.Token == "" {
		t.Errorf("expected token generated")
	}
	if sess.MemberID != "po-buchin" {
		t.Errorf("unexpected member id: %s", sess.MemberID)
	}

	// Validate token
	validated, err := store.ValidateSession(ctx, sess.Token)
	if err != nil {
		t.Fatalf("ValidateSession failed: %v", err)
	}
	if validated.ID != sess.ID {
		t.Errorf("expected matching ID")
	}

	// List sessions for member
	list, err := store.ListSessions(ctx, "po-buchin")
	if err != nil {
		t.Fatalf("ListSessions failed: %v", err)
	}
	if len(list) != 1 {
		t.Errorf("expected 1 session, got %d", len(list))
	}
}

func TestSessionStore_RevocationAndExpiry(t *testing.T) {
	ctx := context.Background()
	store := NewMemorySessionStore()

	now := time.Now().UTC()
	sess := &AgentSession{
		ID:        "asess_expired",
		MemberID:  "buchin",
		AgentID:   "worker-pro",
		Token:     "token_expired_123",
		Status:    SessionStatusActive,
		CreatedAt: now.Add(-2 * time.Hour),
		ExpiresAt: now.Add(-1 * time.Hour), // Expired
	}
	_ = store.CreateSession(ctx, sess)

	_, err := store.ValidateSession(ctx, "token_expired_123")
	if err != ErrSessionExpired {
		t.Fatalf("expected ErrSessionExpired, got %v", err)
	}

	// Test revocation
	validSess := &AgentSession{
		ID:        "asess_revoke",
		MemberID:  "buchin",
		AgentID:   "worker-pro",
		Token:     "token_revoke_123",
		Status:    SessionStatusActive,
		CreatedAt: now,
		ExpiresAt: now.Add(1 * time.Hour),
	}
	_ = store.CreateSession(ctx, validSess)

	if err := store.RevokeSession(ctx, "asess_revoke"); err != nil {
		t.Fatalf("failed to revoke session: %v", err)
	}

	_, err = store.ValidateSession(ctx, "token_revoke_123")
	if err != ErrSessionRevoked {
		t.Fatalf("expected ErrSessionRevoked, got %v", err)
	}
}
