package validator

import "fmt"

// bcryptMaxBytes is bcrypt's hard input limit; bytes beyond it are silently
// truncated, so two different long passwords could hash identically.
const bcryptMaxBytes = 72

// PasswordWithinBcryptLimit rejects passwords longer than bcrypt can safely hash.
func PasswordWithinBcryptLimit(password string) error {
	if len([]byte(password)) > bcryptMaxBytes {
		return fmt.Errorf("password must not exceed %d bytes", bcryptMaxBytes)
	}
	return nil
}
