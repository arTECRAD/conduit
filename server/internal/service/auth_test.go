package service_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/arTECRAD/conduit/server/internal/auth"
	"github.com/arTECRAD/conduit/server/internal/model"
	"github.com/arTECRAD/conduit/server/internal/repository"
	"github.com/arTECRAD/conduit/server/internal/service"
)

// fakeAuthQuerier is an in-memory implementation of service.AuthQuerier.
type fakeAuthQuerier struct {
	users         map[string]repository.User   // keyed by email
	refreshTokens map[string]repository.RefreshToken // keyed by token_hash
}

func newFakeAuthQuerier() *fakeAuthQuerier {
	return &fakeAuthQuerier{
		users:         make(map[string]repository.User),
		refreshTokens: make(map[string]repository.RefreshToken),
	}
}

var errNotFound = errors.New("not found")
var errDuplicate = errors.New("duplicate key value violates unique constraint (23505)")

func (f *fakeAuthQuerier) CreateUser(_ context.Context, arg repository.CreateUserParams) (repository.User, error) {
	if _, exists := f.users[arg.Email]; exists {
		return repository.User{}, errDuplicate
	}
	u := repository.User{
		ID:           uuid.New(),
		Email:        arg.Email,
		PasswordHash: arg.PasswordHash,
		DisplayName:  arg.DisplayName,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}
	f.users[arg.Email] = u
	return u, nil
}

func (f *fakeAuthQuerier) GetUserByEmail(_ context.Context, email string) (repository.User, error) {
	u, ok := f.users[email]
	if !ok {
		return repository.User{}, errNotFound
	}
	return u, nil
}

func (f *fakeAuthQuerier) GetUserByID(_ context.Context, id uuid.UUID) (repository.User, error) {
	for _, u := range f.users {
		if u.ID == id {
			return u, nil
		}
	}
	return repository.User{}, errNotFound
}

func (f *fakeAuthQuerier) CreateRefreshToken(_ context.Context, arg repository.CreateRefreshTokenParams) (repository.RefreshToken, error) {
	rt := repository.RefreshToken{
		ID:        uuid.New(),
		UserID:    arg.UserID,
		TokenHash: arg.TokenHash,
		ExpiresAt: arg.ExpiresAt,
		CreatedAt: time.Now(),
	}
	f.refreshTokens[arg.TokenHash] = rt
	return rt, nil
}

func (f *fakeAuthQuerier) GetRefreshTokenByHash(_ context.Context, hash string) (repository.RefreshToken, error) {
	rt, ok := f.refreshTokens[hash]
	if !ok {
		return repository.RefreshToken{}, errNotFound
	}
	return rt, nil
}

func (f *fakeAuthQuerier) DeleteRefreshToken(_ context.Context, hash string) error {
	delete(f.refreshTokens, hash)
	return nil
}

func (f *fakeAuthQuerier) DeleteRefreshTokensByUserID(_ context.Context, userID uuid.UUID) error {
	for h, rt := range f.refreshTokens {
		if rt.UserID == userID {
			delete(f.refreshTokens, h)
		}
	}
	return nil
}

func newTestAuthService(q service.AuthQuerier) *service.AuthService {
	return service.NewAuthService(q, "test-secret-key", 15*time.Minute, 720*time.Hour, 4)
}

func TestAuthService_Register(t *testing.T) {
	t.Run("happy path creates user and returns tokens", func(t *testing.T) {
		q := newFakeAuthQuerier()
		svc := newTestAuthService(q)

		resp, refreshToken, err := svc.Register(context.Background(), model.RegisterRequest{
			Email:    "test@example.com",
			Password: "password123",
		})
		require.NoError(t, err)
		assert.NotEmpty(t, resp.AccessToken)
		assert.NotEmpty(t, refreshToken)
		assert.Equal(t, "test@example.com", resp.User.Email)
	})

	t.Run("duplicate email returns ErrEmailTaken", func(t *testing.T) {
		q := newFakeAuthQuerier()
		svc := newTestAuthService(q)

		_, _, err := svc.Register(context.Background(), model.RegisterRequest{
			Email:    "dup@example.com",
			Password: "password123",
		})
		require.NoError(t, err)

		_, _, err = svc.Register(context.Background(), model.RegisterRequest{
			Email:    "dup@example.com",
			Password: "different",
		})
		assert.ErrorIs(t, err, service.ErrEmailTaken)
	})
}

func TestAuthService_Login(t *testing.T) {
	q := newFakeAuthQuerier()
	svc := newTestAuthService(q)

	_, _, err := svc.Register(context.Background(), model.RegisterRequest{
		Email:    "login@example.com",
		Password: "correct-password",
	})
	require.NoError(t, err)

	t.Run("correct credentials return tokens", func(t *testing.T) {
		resp, refreshToken, err := svc.Login(context.Background(), model.LoginRequest{
			Email:    "login@example.com",
			Password: "correct-password",
		})
		require.NoError(t, err)
		assert.NotEmpty(t, resp.AccessToken)
		assert.NotEmpty(t, refreshToken)
	})

	t.Run("wrong password returns ErrInvalidCredentials", func(t *testing.T) {
		_, _, err := svc.Login(context.Background(), model.LoginRequest{
			Email:    "login@example.com",
			Password: "wrong-password",
		})
		assert.ErrorIs(t, err, service.ErrInvalidCredentials)
	})

	t.Run("unknown email returns ErrInvalidCredentials", func(t *testing.T) {
		_, _, err := svc.Login(context.Background(), model.LoginRequest{
			Email:    "nobody@example.com",
			Password: "whatever",
		})
		assert.ErrorIs(t, err, service.ErrInvalidCredentials)
	})
}

func TestAuthService_Refresh(t *testing.T) {
	q := newFakeAuthQuerier()
	svc := newTestAuthService(q)

	_, refreshToken, err := svc.Register(context.Background(), model.RegisterRequest{
		Email:    "refresh@example.com",
		Password: "password123",
	})
	require.NoError(t, err)

	t.Run("valid token rotates correctly", func(t *testing.T) {
		resp, newToken, err := svc.Refresh(context.Background(), refreshToken)
		require.NoError(t, err)
		assert.NotEmpty(t, resp.AccessToken)
		assert.NotEmpty(t, newToken)
		assert.NotEqual(t, refreshToken, newToken)

		// Old token should now be invalid
		_, _, err = svc.Refresh(context.Background(), refreshToken)
		assert.ErrorIs(t, err, service.ErrTokenInvalid)
	})
}

func TestAuthService_Logout(t *testing.T) {
	q := newFakeAuthQuerier()
	svc := newTestAuthService(q)

	_, refreshToken, err := svc.Register(context.Background(), model.RegisterRequest{
		Email:    "logout@example.com",
		Password: "password123",
	})
	require.NoError(t, err)

	t.Run("valid token deleted", func(t *testing.T) {
		err := svc.Logout(context.Background(), refreshToken)
		require.NoError(t, err)

		// Token should now be invalid
		_, _, err = svc.Refresh(context.Background(), refreshToken)
		assert.ErrorIs(t, err, service.ErrTokenInvalid)
	})

	t.Run("unknown token returns no error (idempotent)", func(t *testing.T) {
		err := svc.Logout(context.Background(), "nonexistent-token")
		assert.NoError(t, err)
	})
}

// Verify that the auth package's token can be used to validate.
func TestAuthService_ValidateAccessToken(t *testing.T) {
	q := newFakeAuthQuerier()
	svc := newTestAuthService(q)

	resp, _, err := svc.Register(context.Background(), model.RegisterRequest{
		Email:    "validate@example.com",
		Password: "password123",
	})
	require.NoError(t, err)

	userID, err := svc.ValidateAccessToken(resp.AccessToken)
	require.NoError(t, err)
	assert.Equal(t, resp.User.ID, userID)

	// Refresh token hash doesn't validate as a JWT
	_, err = svc.ValidateAccessToken(auth.HashRefreshToken("not-a-jwt"))
	assert.Error(t, err)
}
