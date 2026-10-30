package auth

import "time"

type Owner struct {
	ID           string    `json:"id"`
	Email        string    `json:"email"`
	Username     string    `json:"username"`
	PasswordHash string    `json:"-"`
	IsSetup      bool      `json:"is_setup"`
	Name         string    `json:"name,omitempty"`
	Role         string    `json:"role"`
	IsActive     bool      `json:"is_active"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type Session struct {
	TokenHash string    `json:"-"`
	OwnerID   string    `json:"owner_id"`
	ExpiresAt time.Time `json:"expires_at"`
	CreatedAt time.Time `json:"created_at"`
}

type SetupRequest struct {
	Username string `json:"username,omitempty"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

type LoginRequest struct {
	Username string `json:"username,omitempty"`
	Email    string `json:"email,omitempty"`
	Password string `json:"password"`
}

type ChangePasswordRequest struct {
	CurrentPassword string `json:"current_password"`
	NewPassword     string `json:"new_password"`
}

type OwnerInfo struct {
	ID       string `json:"id"`
	Username string `json:"username,omitempty"`
	Email    string `json:"email"`
	Name     string `json:"name,omitempty"`
	Role     string `json:"role,omitempty"`
	IsActive bool   `json:"is_active,omitempty"`
}

type AuthResponse struct {
	Token string    `json:"token"`
	Owner OwnerInfo `json:"owner"`
	User  OwnerInfo `json:"user"`
}

type StatusResponse struct {
	SetupRequired   bool `json:"setup_required"`
	OwnerConfigured bool `json:"owner_configured"`
}

// UserInvitation represents a staff member invitation.
type UserInvitation struct {
	ID         string     `json:"id"`
	TokenHash  string     `json:"-"`
	Email      string     `json:"email"`
	Role       string     `json:"role"`
	CreatedBy  string     `json:"created_by,omitempty"`
	ExpiresAt  time.Time  `json:"expires_at"`
	AcceptedAt *time.Time `json:"accepted_at,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
}

// CreateInviteRequest payload to invite a new team member.
type CreateInviteRequest struct {
	Email string `json:"email"`
	Role  string `json:"role,omitempty"`
}

// CreateInviteResponse returned when an invitation is generated.
type CreateInviteResponse struct {
	Status     string         `json:"status"`
	Invitation UserInvitation `json:"invitation"`
	Token      string         `json:"token"`
	InviteURL  string         `json:"invite_url"`
}

// AcceptInviteRequest payload when a staff member accepts their invitation.
type AcceptInviteRequest struct {
	Token    string `json:"token"`
	Name     string `json:"name"`
	Password string `json:"password"`
}

// TeamMemberDTO represents a human staff member in team listings.
type TeamMemberDTO struct {
	ID        string    `json:"id"`
	Name      string    `json:"name,omitempty"`
	Username  string    `json:"username,omitempty"`
	Email     string    `json:"email"`
	Role      string    `json:"role"`
	IsActive  bool      `json:"is_active"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
}

// TeamMembersResponse represents the response containing staff members and pending invites.
type TeamMembersResponse struct {
	Status      string           `json:"status"`
	Members     []TeamMemberDTO  `json:"members"`
	Invitations []UserInvitation `json:"invitations"`
}
