package model

// RegisterRequest is the request body for user registration.
type RegisterRequest struct {
	Email       string `json:"email"        validate:"required,email"`
	Password    string `json:"password"     validate:"required,min=8"`
	DisplayName string `json:"display_name"`
}

// LoginRequest is the request body for user login.
type LoginRequest struct {
	Email    string `json:"email"    validate:"required,email"`
	Password string `json:"password" validate:"required"`
}

// AuthResponse is returned on successful login or registration.
type AuthResponse struct {
	AccessToken string      `json:"access_token"`
	User        UserProfile `json:"user"`
}

// RefreshResponse is returned when a refresh token is exchanged.
type RefreshResponse struct {
	AccessToken string `json:"access_token"`
}
