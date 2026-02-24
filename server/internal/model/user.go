package model

import (
	"time"

	"github.com/google/uuid"
)

// User represents a Conduit platform user.
type User struct {
	ID           uuid.UUID `json:"id"`
	Email        string    `json:"email"`
	PasswordHash string    `json:"-"`
	DisplayName  string    `json:"display_name"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// UserProfile is the public-facing representation of a user (no sensitive fields).
type UserProfile struct {
	ID          uuid.UUID `json:"id"`
	Email       string    `json:"email"`
	DisplayName string    `json:"display_name"`
	CreatedAt   time.Time `json:"created_at"`
}

// UpdateUserRequest is the request body for updating user profile.
type UpdateUserRequest struct {
	DisplayName string `json:"display_name"`
}
