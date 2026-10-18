package generator

import (
	"fmt"

	"golang.org/x/crypto/bcrypt"
)

// Default bcrypt cost factors
const (
	// MinCost is the minimum allowable cost for bcrypt (for testing only)
	MinCost = bcrypt.MinCost // 4

	// DefaultCost provides a good balance between security and performance
	// This should take approximately 250ms to hash on modern hardware
	DefaultCost = bcrypt.DefaultCost // 10

	// MaxCost is the maximum allowable cost for bcrypt
	// Warning: Higher costs significantly increase computation time
	MaxCost = bcrypt.MaxCost // 31

	// RecommendedCost for production environments (2024+)
	// Adjust based on your security requirements and server capacity
	RecommendedCost = 12
)

// Hash hashes a password with bcrypt at RecommendedCost.
//
// Example:
//
//	hash, err := generator.Hash("MyPassword123")
//	if err != nil {
//	    log.Fatal(err)
//	}
func Hash(password string) (string, error) {
	if len([]byte(password)) > 72 {
		return "", fmt.Errorf("password exceeds bcrypt's 72-byte limit")
	}

	// Generate hash with embedded salt using recommended cost
	hashedBytes, err := bcrypt.GenerateFromPassword([]byte(password), RecommendedCost)
	if err != nil {
		return "", fmt.Errorf("failed to hash password: %w", err)
	}

	return string(hashedBytes), nil
}

// NeedsRehash reports whether a stored hash was made at a cost other than
// RecommendedCost, or cannot be read, so it should be hashed again at login.
//
// Example:
//
//	if generator.NeedsRehash(storedHash) {
//	    newHash, _ := generator.Hash(password)
//	    // Update database with newHash
//	}
func NeedsRehash(hashedPassword string) bool {
	// Extract cost from the hash
	cost, err := bcrypt.Cost([]byte(hashedPassword))
	if err != nil {
		// If we can't determine the cost, assume it needs rehashing
		return true
	}

	// Check if the cost matches the recommended cost
	return cost != RecommendedCost
}
