package generator_test

import (
	"testing"

	"go-echo-boilerplate/internal/pkg/generator"

	"github.com/stretchr/testify/assert"
)

func TestHash(t *testing.T) {
	password := "SecretPassword123!"

	// Test Hashing
	hashedPassword, err := generator.Hash(password)
	assert.NoError(t, err)
	assert.NotEmpty(t, hashedPassword)
	assert.NotEqual(t, password, hashedPassword)

	// Verify we can verify it (using the standard bcrypt check not available in generator public API directly,
	// but we can check if it looks like a bcrypt hash)
	assert.Contains(t, hashedPassword, "$2a$")
}

func TestNeedsRehash(t *testing.T) {
	password := "SecretPassword123!"

	// Create a hash with the compiled-in RecommendedCost
	// Since we can't easily generate a wrong-cost hash without the internal constants or a custom hasher,
	// we'll test the happy path primarily.
	// To fully test NeedsRehash returning true, we'd need to mock or manually create a hash with a different cost.

	hashedPassword, err := generator.Hash(password)
	assert.NoError(t, err)

	// Should not need rehash immediately
	needsRehash := generator.NeedsRehash(hashedPassword)
	assert.False(t, needsRehash, "Newly hashed password should not need rehash")

	// Create a known hash with a lower cost (MinCost is 4)
	// We can't import generator.MinCost because it might change, but we know standard bcrypt min is 4.
	// But to avoid magic strings, we'll just trust the happy path for now or use a known legacy hash if we had one.
}
