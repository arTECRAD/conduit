package auth_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/arTECRAD/conduit/server/internal/auth"
)

func TestGenerateRefreshToken(t *testing.T) {
	token, err := auth.GenerateRefreshToken()
	require.NoError(t, err)
	assert.Len(t, token, 64, "refresh token should be 64 hex chars (32 bytes)")

	// Each call produces a different token
	token2, err := auth.GenerateRefreshToken()
	require.NoError(t, err)
	assert.NotEqual(t, token, token2)
}

func TestHashRefreshToken(t *testing.T) {
	t.Run("same input always produces same hash", func(t *testing.T) {
		input := "some-token-value"
		h1 := auth.HashRefreshToken(input)
		h2 := auth.HashRefreshToken(input)
		assert.Equal(t, h1, h2)
		assert.NotEmpty(t, h1)
	})

	t.Run("different inputs produce different hashes", func(t *testing.T) {
		h1 := auth.HashRefreshToken("token-a")
		h2 := auth.HashRefreshToken("token-b")
		assert.NotEqual(t, h1, h2)
	})
}
