package validator

import (
	"errors"
	"fmt"

	"golang.org/x/crypto/bcrypt"
)

// Verify is a package-level convenience function that verifies a password against a hash.
// This allows you to verify passwords without creating a hasher instance first.
//
// Example:
//
//	match, err := generator.Verify("MyPassword123", storedHash)
//	if err != nil {
//	    log.Fatal(err)
//	}
//	if match {
//	    fmt.Println("Password is correct!")
//	}
func Hash(password, hashedPassword string) (bool, error) {
	// Compare password with hash
	err := bcrypt.CompareHashAndPassword([]byte(hashedPassword), []byte(password))
	if err != nil {
		if errors.Is(err, bcrypt.ErrMismatchedHashAndPassword) {
			// Password doesn't match - this is not an error, just return false
			return false, nil
		}
		// Actual error occurred (e.g., malformed hash)
		return false, fmt.Errorf("failed to verify password: %w", err)
	}

	return true, nil
}
