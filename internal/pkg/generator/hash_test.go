package generator_test

import (
	"testing"

	"go-echo-boilerplate/internal/pkg/generator"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"
)

func TestHash(t *testing.T) {
	password := "SecretPassword123!"

	hashedPassword, err := generator.Hash(password)
	require.NoError(t, err)
	assert.NoError(t, bcrypt.CompareHashAndPassword([]byte(hashedPassword), []byte(password)))

	cost, err := bcrypt.Cost([]byte(hashedPassword))
	require.NoError(t, err)
	assert.Equal(t, generator.RecommendedCost, cost)
}

func TestNeedsRehash(t *testing.T) {
	current, err := generator.Hash("SecretPassword123!")
	require.NoError(t, err)
	assert.False(t, generator.NeedsRehash(current), "a hash at RecommendedCost is current")

	legacy, err := bcrypt.GenerateFromPassword([]byte("SecretPassword123!"), bcrypt.MinCost)
	require.NoError(t, err)
	assert.True(t, generator.NeedsRehash(string(legacy)), "a hash at another cost needs a rehash")

	assert.True(t, generator.NeedsRehash("not-a-bcrypt-hash"), "an unreadable hash needs a rehash")
}
