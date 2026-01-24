package validator

import (
	"strings"

	v10 "github.com/go-playground/validator/v10"
)

// Password validation constants
const (
	// MinPasswordLength is the minimum secure password length
	MinPasswordLength = 8
	// MaxPasswordLength is bcrypt's maximum (72 bytes)
	MaxPasswordLength = 72
)

// registerPasswordValidations registers all password-related custom validators
func registerPasswordValidations() {
	registerPasswordMinLength()
	registerPasswordMaxLength()
	registerPasswordStrength()
}

// registerPasswordMinLength validates minimum password length
func registerPasswordMinLength() {
	if err := valid.RegisterValidation("passwordMinLength", func(fl v10.FieldLevel) bool {
		str, ok := getStringValue(fl)
		if !ok {
			return false
		}

		if str == "" {
			return true // Empty strings handled by 'required' tag
		}

		return len(str) >= MinPasswordLength
	}); err != nil {
		panic(err)
	}
}

// registerPasswordMaxLength validates maximum password length (bcrypt limit)
func registerPasswordMaxLength() {
	if err := valid.RegisterValidation("passwordMaxLength", func(fl v10.FieldLevel) bool {
		str, ok := getStringValue(fl)
		if !ok {
			return false
		}

		if str == "" {
			return true // Empty strings handled by 'required' tag
		}

		return len(str) <= MaxPasswordLength
	}); err != nil {
		panic(err)
	}
}

// registerPasswordStrength validates password complexity
// Requires at least one uppercase, one lowercase, one number, and one special character
func registerPasswordStrength() {
	if err := valid.RegisterValidation("passwordStrength", func(fl v10.FieldLevel) bool {
		str, ok := getStringValue(fl)
		if !ok {
			return false
		}

		if str == "" {
			return true // Empty strings handled by 'required' tag
		}

		var (
			hasUpper   bool
			hasLower   bool
			hasNumber  bool
			hasSpecial bool
		)

		for _, char := range str {
			switch {
			case char >= 'A' && char <= 'Z':
				hasUpper = true
			case char >= 'a' && char <= 'z':
				hasLower = true
			case char >= '0' && char <= '9':
				hasNumber = true
			case strings.ContainsRune("!@#$%^&*()_+-=[]{}|;:,.<>?", char):
				hasSpecial = true
			}
		}

		return hasUpper && hasLower && hasNumber && hasSpecial
	}); err != nil {
		panic(err)
	}
}
