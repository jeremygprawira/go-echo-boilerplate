package validator

import "fmt"

// bcryptMaxBytes is bcrypt's hard input limit. golang.org/x/crypto v0.46.0+
// already rejects passwords over this length with ErrPasswordTooLong rather
// than truncating; this guard exists to give the client a clean, early 400
// instead of an opaque 500 from the hashing layer, and to protect against a
// future downgrade of the dependency reintroducing truncation behavior.
const bcryptMaxBytes = 72

// PasswordWithinBcryptLimit rejects passwords longer than bcrypt can safely hash.
func PasswordWithinBcryptLimit(password string) error {
	if len([]byte(password)) > bcryptMaxBytes {
		return fmt.Errorf("password must not exceed %d bytes", bcryptMaxBytes)
	}
	return nil
}
