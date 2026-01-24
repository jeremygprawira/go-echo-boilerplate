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

// Hash is a package-level convenience function that hashes a password using the default hasher.
// This allows you to hash passwords without creating a hasher instance first.
//
// Example:
//
//	hash, err := generator.Hash("MyPassword123")
//	if err != nil {
//	    log.Fatal(err)
//	}
//
// For custom cost factors, use NewBcryptHasherWithCost instead.
func Hash(password string) (string, error) {
	// Generate hash with embedded salt using recommended cost
	hashedBytes, err := bcrypt.GenerateFromPassword([]byte(password), RecommendedCost)
	if err != nil {
		return "", fmt.Errorf("failed to hash password: %w", err)
	}

	return string(hashedBytes), nil
}

// NeedsRehash is a package-level convenience function that checks if a hash needs regeneration.
// This allows you to check rehash status without creating a hasher instance first.
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
