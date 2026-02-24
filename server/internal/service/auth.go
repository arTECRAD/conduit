package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"

	"github.com/arTECRAD/conduit/server/internal/auth"
	"github.com/arTECRAD/conduit/server/internal/model"
	"github.com/arTECRAD/conduit/server/internal/repository"
)

// ErrInvalidCredentials is returned when login credentials are incorrect.
var ErrInvalidCredentials = errors.New("invalid credentials")

// ErrEmailTaken is returned when a registration email already exists.
var ErrEmailTaken = errors.New("email already in use")

// ErrTokenInvalid is returned when a refresh token is invalid or expired.
var ErrTokenInvalid = errors.New("invalid or expired refresh token")

// AuthQuerier is the subset of repository.Queries used by AuthService.
type AuthQuerier interface {
	CreateUser(ctx context.Context, arg repository.CreateUserParams) (repository.User, error)
	GetUserByEmail(ctx context.Context, email string) (repository.User, error)
	GetUserByID(ctx context.Context, id uuid.UUID) (repository.User, error)
	CreateRefreshToken(ctx context.Context, arg repository.CreateRefreshTokenParams) (repository.RefreshToken, error)
	GetRefreshTokenByHash(ctx context.Context, tokenHash string) (repository.RefreshToken, error)
	DeleteRefreshToken(ctx context.Context, tokenHash string) error
	DeleteRefreshTokensByUserID(ctx context.Context, userID uuid.UUID) error
}

// AuthService handles authentication business logic.
type AuthService struct {
	q              AuthQuerier
	jwtSecret      string
	accessTokenTTL time.Duration
	refreshTTL     time.Duration
	bcryptCost     int
}

// NewAuthService creates a new AuthService.
func NewAuthService(q AuthQuerier, jwtSecret string, accessTokenTTL, refreshTTL time.Duration, bcryptCost int) *AuthService {
	return &AuthService{
		q:              q,
		jwtSecret:      jwtSecret,
		accessTokenTTL: accessTokenTTL,
		refreshTTL:     refreshTTL,
		bcryptCost:     bcryptCost,
	}
}

// Register creates a new user and returns tokens.
func (s *AuthService) Register(ctx context.Context, req model.RegisterRequest) (*model.AuthResponse, string, error) {
	hash, err := auth.HashPassword(req.Password, s.bcryptCost)
	if err != nil {
		return nil, "", fmt.Errorf("register: %w", err)
	}

	user, err := s.q.CreateUser(ctx, repository.CreateUserParams{
		Email:        req.Email,
		PasswordHash: hash,
		DisplayName:  req.DisplayName,
	})
	if err != nil {
		if isDuplicateKeyError(err) {
			return nil, "", ErrEmailTaken
		}
		return nil, "", fmt.Errorf("create user: %w", err)
	}

	accessToken, refreshToken, err := s.issueTokens(ctx, user.ID)
	if err != nil {
		return nil, "", err
	}

	return &model.AuthResponse{
		AccessToken: accessToken,
		User: model.UserProfile{
			ID:          user.ID,
			Email:       user.Email,
			DisplayName: user.DisplayName,
			CreatedAt:   user.CreatedAt,
		},
	}, refreshToken, nil
}

// Login authenticates a user and returns tokens.
func (s *AuthService) Login(ctx context.Context, req model.LoginRequest) (*model.AuthResponse, string, error) {
	user, err := s.q.GetUserByEmail(ctx, req.Email)
	if err != nil {
		slog.WarnContext(ctx, "login: user not found", "email", req.Email)
		return nil, "", ErrInvalidCredentials
	}

	if err := auth.VerifyPassword(user.PasswordHash, req.Password); err != nil {
		return nil, "", ErrInvalidCredentials
	}

	accessToken, refreshToken, err := s.issueTokens(ctx, user.ID)
	if err != nil {
		return nil, "", err
	}

	return &model.AuthResponse{
		AccessToken: accessToken,
		User: model.UserProfile{
			ID:          user.ID,
			Email:       user.Email,
			DisplayName: user.DisplayName,
			CreatedAt:   user.CreatedAt,
		},
	}, refreshToken, nil
}

// Refresh rotates a refresh token and issues new tokens.
func (s *AuthService) Refresh(ctx context.Context, rawToken string) (*model.RefreshResponse, string, error) {
	hash := auth.HashRefreshToken(rawToken)

	rt, err := s.q.GetRefreshTokenByHash(ctx, hash)
	if err != nil {
		return nil, "", ErrTokenInvalid
	}
	if time.Now().After(rt.ExpiresAt) {
		_ = s.q.DeleteRefreshToken(ctx, hash)
		return nil, "", ErrTokenInvalid
	}

	// Rotate: delete old, issue new
	if err := s.q.DeleteRefreshToken(ctx, hash); err != nil {
		return nil, "", fmt.Errorf("delete old refresh token: %w", err)
	}

	accessToken, newRefreshToken, err := s.issueTokens(ctx, rt.UserID)
	if err != nil {
		return nil, "", err
	}

	return &model.RefreshResponse{AccessToken: accessToken}, newRefreshToken, nil
}

// Logout invalidates a refresh token.
func (s *AuthService) Logout(ctx context.Context, rawToken string) error {
	hash := auth.HashRefreshToken(rawToken)
	// Idempotent: ignore not-found errors
	if err := s.q.DeleteRefreshToken(ctx, hash); err != nil {
		slog.WarnContext(ctx, "logout: token not found (idempotent)", "err", err)
	}
	return nil
}

func (s *AuthService) issueTokens(ctx context.Context, userID uuid.UUID) (accessToken, refreshToken string, err error) {
	accessToken, err = auth.GenerateAccessToken(userID, s.jwtSecret, s.accessTokenTTL)
	if err != nil {
		return "", "", fmt.Errorf("generate access token: %w", err)
	}

	refreshToken, err = auth.GenerateRefreshToken()
	if err != nil {
		return "", "", fmt.Errorf("generate refresh token: %w", err)
	}

	hash := auth.HashRefreshToken(refreshToken)
	_, err = s.q.CreateRefreshToken(ctx, repository.CreateRefreshTokenParams{
		UserID:    userID,
		TokenHash: hash,
		ExpiresAt: time.Now().Add(s.refreshTTL),
	})
	if err != nil {
		return "", "", fmt.Errorf("store refresh token: %w", err)
	}

	return accessToken, refreshToken, nil
}

// isDuplicateKeyError checks if the error is a PostgreSQL unique violation.
func isDuplicateKeyError(err error) bool {
	return err != nil && (contains(err.Error(), "duplicate key") || contains(err.Error(), "23505"))
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsStr(s, substr))
}

func containsStr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// GetUserByID returns a user's profile by ID.
func (s *AuthService) GetUserByID(ctx context.Context, userID uuid.UUID) (*model.UserProfile, error) {
	user, err := s.q.GetUserByID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("get user: %w", err)
	}
	return &model.UserProfile{
		ID:          user.ID,
		Email:       user.Email,
		DisplayName: user.DisplayName,
		CreatedAt:   user.CreatedAt,
	}, nil
}

// ValidateAccessToken validates a JWT and returns the user ID.
func (s *AuthService) ValidateAccessToken(tokenString string) (uuid.UUID, error) {
	claims, err := auth.ValidateAccessToken(tokenString, s.jwtSecret)
	if err != nil {
		return uuid.UUID{}, fmt.Errorf("validate access token: %w", err)
	}
	userID, err := uuid.Parse(claims.Subject)
	if err != nil {
		return uuid.UUID{}, fmt.Errorf("parse user ID from token: %w", err)
	}
	return userID, nil
}

// bcrypt is imported for ErrMismatchedHashAndPassword — keep this explicit.
var _ = bcrypt.ErrMismatchedHashAndPassword
