package auth

import (
	"context"
	"database/sql"
	"testing"

	_ "modernc.org/sqlite"
)

func TestMultiUser_InviteAcceptLoginCycle_Memory(t *testing.T) {
	store := NewMemoryStore()
	svc := NewService(store)
	testInviteAcceptLoginCycle(t, svc)
}

func TestMultiUser_InviteAcceptLoginCycle_DB(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)

	schema := []string{
		`CREATE TABLE owners (
			id TEXT PRIMARY KEY,
			email TEXT,
			username TEXT UNIQUE,
			password_hash TEXT,
			is_setup BOOLEAN,
			name TEXT,
			role TEXT DEFAULT 'owner',
			is_active BOOLEAN DEFAULT 1,
			created_at DATETIME,
			updated_at DATETIME
		)`,
		`CREATE TABLE owner_sessions (
			token_hash TEXT PRIMARY KEY,
			owner_id TEXT,
			expires_at DATETIME,
			created_at DATETIME
		)`,
		`CREATE TABLE user_invitations (
			id TEXT PRIMARY KEY,
			token_hash TEXT UNIQUE,
			email TEXT,
			role TEXT DEFAULT 'member',
			created_by TEXT,
			expires_at DATETIME,
			accepted_at DATETIME,
			created_at DATETIME
		)`,
	}
	for _, stmt := range schema {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatalf("setup db: %v", err)
		}
	}

	store := NewDBStore(db)
	svc := NewService(store)
	testInviteAcceptLoginCycle(t, svc)
}

func testInviteAcceptLoginCycle(t *testing.T, svc *Service) {
	t.Helper()
	ctx := context.Background()

	// 1. Initial Setup: Owner registers
	ownerSetup, err := svc.Setup(ctx, "boss@company.local", "bossPassword123")
	if err != nil {
		t.Fatalf("setup failed: %v", err)
	}
	if ownerSetup.Owner.Role != RoleOwner {
		t.Errorf("expected owner role %q, got %q", RoleOwner, ownerSetup.Owner.Role)
	}

	// 2. Owner creates invite for staff member
	inviteResp, err := svc.CreateInvite(ctx, ownerSetup.Owner.ID, "staff@company.local", RoleMember)
	if err != nil {
		t.Fatalf("create invite failed: %v", err)
	}
	if inviteResp.Token == "" {
		t.Errorf("expected non-empty invite token")
	}
	if inviteResp.Invitation.Role != RoleMember {
		t.Errorf("expected role member, got %q", inviteResp.Invitation.Role)
	}

	// 3. Verify invite token
	verified, err := svc.VerifyInvite(ctx, inviteResp.Token)
	if err != nil {
		t.Fatalf("verify invite failed: %v", err)
	}
	if verified.Email != "staff@company.local" {
		t.Errorf("expected email staff@company.local, got %s", verified.Email)
	}

	// 4. Staff accepts invitation
	acceptResp, err := svc.AcceptInvite(ctx, inviteResp.Token, "Budi Staff", "staffPassword123")
	if err != nil {
		t.Fatalf("accept invite failed: %v", err)
	}
	if acceptResp.Token == "" {
		t.Errorf("expected non-empty session token on accept")
	}
	if acceptResp.User.Role != RoleMember {
		t.Errorf("expected user role member, got %q", acceptResp.User.Role)
	}
	if acceptResp.User.Name != "Budi Staff" {
		t.Errorf("expected name Budi Staff, got %q", acceptResp.User.Name)
	}

	// 5. Token from accept is valid
	staffUser, err := svc.ValidateToken(ctx, acceptResp.Token)
	if err != nil {
		t.Fatalf("validate accepted session failed: %v", err)
	}
	if staffUser.Role != RoleMember {
		t.Errorf("expected validated user role member, got %q", staffUser.Role)
	}

	// 6. Second accept fails (already accepted)
	_, err = svc.AcceptInvite(ctx, inviteResp.Token, "Budi Again", "staffPassword123")
	if err == nil {
		t.Errorf("expected error on duplicate accept, got nil")
	}

	// 7. Staff logs out and logs back in with email
	_ = svc.Logout(ctx, acceptResp.Token)
	loginResp, err := svc.Login(ctx, "staff@company.local", "staffPassword123")
	if err != nil {
		t.Fatalf("staff login failed: %v", err)
	}
	if loginResp.User.Role != RoleMember {
		t.Errorf("expected login role member, got %q", loginResp.User.Role)
	}
	if loginResp.User.Email != "staff@company.local" {
		t.Errorf("expected login email staff@company.local, got %q", loginResp.User.Email)
	}

	// 8. Staff session validates
	loggedInUser, err := svc.ValidateToken(ctx, loginResp.Token)
	if err != nil {
		t.Fatalf("validate login session failed: %v", err)
	}
	if loggedInUser.ID != loginResp.User.ID {
		t.Errorf("user ID mismatch")
	}

	// 9. List team members shows both owner and member
	members, pending, err := svc.ListTeamMembers(ctx)
	if err != nil {
		t.Fatalf("list team members failed: %v", err)
	}
	if len(members) != 2 {
		t.Errorf("expected 2 team members, got %d", len(members))
	}
	if len(pending) != 0 {
		t.Errorf("expected 0 pending invites, got %d", len(pending))
	}
}
