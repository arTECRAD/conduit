package auth_test

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/arTECRAD/conduit/server/internal/auth"
)

const testSecret = "test-secret-key-for-unit-tests-only"

func TestGenerateAccessToken(t *testing.T) {
	userID := uuid.New()
	token, err := auth.GenerateAccessToken(userID, testSecret, 15*time.Minute)
	require.NoError(t, err)
	assert.NotEmpty(t, token)

	// Token should parse back with correct subject
	claims, err := auth.ValidateAccessToken(token, testSecret)
	require.NoError(t, err)
	assert.Equal(t, userID.String(), claims.Subject)
}

func TestValidateAccessToken(t *testing.T) {
	userID := uuid.New()

	t.Run("valid token passes", func(t *testing.T) {
		token, err := auth.GenerateAccessToken(userID, testSecret, 15*time.Minute)
		require.NoError(t, err)

		claims, err := auth.ValidateAccessToken(token, testSecret)
		require.NoError(t, err)
		assert.Equal(t, userID.String(), claims.Subject)
	})

	t.Run("expired token returns error", func(t *testing.T) {
		token, err := auth.GenerateAccessToken(userID, testSecret, -1*time.Minute)
		require.NoError(t, err)

		_, err = auth.ValidateAccessToken(token, testSecret)
		assert.Error(t, err)
	})

	t.Run("tampered token returns error", func(t *testing.T) {
		token, err := auth.GenerateAccessToken(userID, testSecret, 15*time.Minute)
		require.NoError(t, err)

		_, err = auth.ValidateAccessToken(token+"tampered", testSecret)
		assert.Error(t, err)
	})

	t.Run("wrong secret returns error", func(t *testing.T) {
		token, err := auth.GenerateAccessToken(userID, testSecret, 15*time.Minute)
		require.NoError(t, err)

		_, err = auth.ValidateAccessToken(token, "wrong-secret")
		assert.Error(t, err)
	})
}
