package auth

import "time"

type Owner struct {
	ID           string    `json:"id"`
	Email        string    `json:"email"`
	Username     string    `json:"username"`
	PasswordHash string    `json:"-"`
	IsSetup      bool      `json:"is_setup"`
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
