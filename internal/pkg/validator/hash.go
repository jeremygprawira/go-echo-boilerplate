package validator

import (
	"errors"
	"fmt"

	"golang.org/x/crypto/bcrypt"
)

// Hash reports whether password matches the bcrypt hashedPassword. A mismatch
// is (false, nil). A malformed hash is an error.
//
// Example:
//
//	match, err := validator.Hash("MyPassword123", storedHash)
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
