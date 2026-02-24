package auth_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"

	"github.com/arTECRAD/conduit/server/internal/auth"
)

const testBcryptCost = 4 // low cost for fast tests

func TestHashPassword(t *testing.T) {
	hash, err := auth.HashPassword("correct-horse-battery-staple", testBcryptCost)
	require.NoError(t, err)
	assert.NotEmpty(t, hash)
	assert.NotEqual(t, "correct-horse-battery-staple", hash)
}

func TestVerifyPassword(t *testing.T) {
	t.Run("correct password verifies", func(t *testing.T) {
		hash, err := auth.HashPassword("my-password", testBcryptCost)
		require.NoError(t, err)

		err = auth.VerifyPassword(hash, "my-password")
		assert.NoError(t, err)
	})

	t.Run("wrong password fails", func(t *testing.T) {
		hash, err := auth.HashPassword("my-password", testBcryptCost)
		require.NoError(t, err)

		err = auth.VerifyPassword(hash, "wrong-password")
		assert.ErrorIs(t, err, bcrypt.ErrMismatchedHashAndPassword)
	})
}
