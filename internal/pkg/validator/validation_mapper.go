package validator

import (
	"fmt"
	"go-echo-boilerplate/internal/models"
	"strings"
	"time"

	v10 "github.com/go-playground/validator/v10"
)

// Validation tag constants for type safety
const (
	TagRequired              = "required"
	TagRequiredWithout       = "required_without"
	TagRequiredWithoutAll    = "required_without_all"
	TagOneOf                 = "oneof"
	TagMin                   = "min"
	TagMax                   = "max"
	TagEmail                 = "email"
	TagNoSpecial             = "nospecial"
	TagNoStartEndSpaces      = "noStartEndSpaces"
	TagDate                  = "date"
	TagDatetime              = "datetime"
	TagYYYMMDDFormat         = "yyymmddFormat"
	TagPhoneFormat           = "phoneFormat"
	TagEmailFormat           = "emailFormat"
	TagEmailPhoneFormat      = "emailPhoneFormat"
	TagNumeric               = "numeric"
	TagDateRange             = "daterange"
	TagYYYYMMDDNoExceedToday = "yyyymmddNoExceedToday"
	TagHHMMFormat            = "hhmmFormat"
	TagEmailOrPhoneField     = "emailOrPhoneField"
	TagPasswordMinLength     = "passwordMinLength"
	TagPasswordMaxLength     = "passwordMaxLength"
	TagPasswordStrength      = "passwordStrength"
)

// Error code constants
const (
	ErrorCodeMissingField = "MISSING_FIELD"
	ErrorCodeInvalidField = "INVALID_FIELD"
)

// validationErrorConfig holds the configuration for each validation error
type validationErrorConfig struct {
	Code           string
	MessageBuilder func(v10.FieldError) string
}

// validationErrorMap provides O(1) lookup for validation error messages
var validationErrorMap = map[string]validationErrorConfig{
	TagRequired: {
		Code:           ErrorCodeMissingField,
		MessageBuilder: func(fe v10.FieldError) string { return fmt.Sprintf("%s is required", fe.Field()) },
	},
	TagRequiredWithout: {
		Code: ErrorCodeMissingField,
		MessageBuilder: func(fe v10.FieldError) string {
			return fmt.Sprintf("%s is required when %s is not inputted", fe.Field(), strings.ReplaceAll(fe.Param(), " ", ", "))
		},
	},
	TagRequiredWithoutAll: {
		Code: ErrorCodeMissingField,
		MessageBuilder: func(fe v10.FieldError) string {
			return fmt.Sprintf("%s is required when %s is not inputted", fe.Field(), strings.ReplaceAll(fe.Param(), " ", ", "))
		},
	},
	TagOneOf: {
		Code: ErrorCodeInvalidField,
		MessageBuilder: func(fe v10.FieldError) string {
			return fmt.Sprintf("%s should be inputted either %s", fe.Field(), fe.Param())
		},
	},
	TagMin: {
		Code: ErrorCodeInvalidField,
		MessageBuilder: func(fe v10.FieldError) string {
			return fmt.Sprintf("%s must be at least %s characters", fe.Field(), fe.Param())
		},
	},
	TagMax: {
		Code: ErrorCodeInvalidField,
		MessageBuilder: func(fe v10.FieldError) string {
			return fmt.Sprintf("%s must be at most %s characters", fe.Field(), fe.Param())
		},
	},
	TagEmail: {
		Code: ErrorCodeInvalidField,
		MessageBuilder: func(fe v10.FieldError) string {
			return fmt.Sprintf("%s must be a valid email address", fe.Field())
		},
	},
	TagNoSpecial: {
		Code: ErrorCodeInvalidField,
		MessageBuilder: func(fe v10.FieldError) string {
			return fmt.Sprintf("%s must not contain special characters", fe.Field())
		},
	},
	TagNoStartEndSpaces: {
		Code: ErrorCodeInvalidField,
		MessageBuilder: func(fe v10.FieldError) string {
			return fmt.Sprintf("%s must not start or end with spaces", fe.Field())
		},
	},
	TagDate: {
		Code: ErrorCodeInvalidField,
		MessageBuilder: func(fe v10.FieldError) string {
			return fmt.Sprintf("%s must be a valid date in YYYY-MM-DD format", fe.Field())
		},
	},
	TagDatetime: {
		Code: ErrorCodeInvalidField,
		MessageBuilder: func(fe v10.FieldError) string {
			return fmt.Sprintf("%s must be a valid date in YYYY-MM-DD HH:MM:SS format", fe.Field())
		},
	},
	TagYYYMMDDFormat: {
		Code: ErrorCodeInvalidField,
		MessageBuilder: func(fe v10.FieldError) string {
			return fmt.Sprintf("%s must be a valid date in YYYY-MM-DD format", fe.Field())
		},
	},
	TagPhoneFormat: {
		Code: ErrorCodeInvalidField,
		MessageBuilder: func(fe v10.FieldError) string {
			return fmt.Sprintf("%s must be a valid phone number in format +628123456789", fe.Field())
		},
	},
	TagEmailFormat: {
		Code: ErrorCodeInvalidField,
		MessageBuilder: func(fe v10.FieldError) string {
			return fmt.Sprintf("%s must be a valid email address", fe.Field())
		},
	},
	TagEmailPhoneFormat: {
		Code: ErrorCodeInvalidField,
		MessageBuilder: func(fe v10.FieldError) string {
			return fmt.Sprintf("%s must be a valid email or phone number", fe.Field())
		},
	},
	TagNumeric: {
		Code: ErrorCodeInvalidField,
		MessageBuilder: func(fe v10.FieldError) string {
			return fmt.Sprintf("%s must be a valid number", fe.Field())
		},
	},
	TagDateRange: {
		Code: ErrorCodeInvalidField,
		MessageBuilder: func(fe v10.FieldError) string {
			return fmt.Sprintf("Your date range should be between %s and %s",
				time.Now().AddDate(0, -6, 0).Format("2006-01-02"),
				time.Now().Format("2006-01-02"))
		},
	},
	TagYYYYMMDDNoExceedToday: {
		Code: ErrorCodeInvalidField,
		MessageBuilder: func(fe v10.FieldError) string {
			return fmt.Sprintf("%s must be a valid date in YYYY-MM-DD format and not exceed today", fe.Field())
		},
	},
	TagHHMMFormat: {
		Code: ErrorCodeInvalidField,
		MessageBuilder: func(fe v10.FieldError) string {
			return fmt.Sprintf("%s must be a valid time in HH:MM format", fe.Field())
		},
	},
	TagEmailOrPhoneField: {
		Code:           ErrorCodeMissingField,
		MessageBuilder: func(fe v10.FieldError) string { return "Email or Phone Number cannot be empty" },
	},
	TagPasswordMinLength: {
		Code: ErrorCodeInvalidField,
		MessageBuilder: func(fe v10.FieldError) string {
			return fmt.Sprintf("%s must be at least 8 characters long", fe.Field())
		},
	},
	TagPasswordMaxLength: {
		Code: ErrorCodeInvalidField,
		MessageBuilder: func(fe v10.FieldError) string {
			return fmt.Sprintf("%s must not exceed 72 characters (bcrypt limit)", fe.Field())
		},
	},
	TagPasswordStrength: {
		Code: ErrorCodeInvalidField,
		MessageBuilder: func(fe v10.FieldError) string {
			return fmt.Sprintf("%s must contain at least one uppercase letter, one lowercase letter, one number, and one special character", fe.Field())
		},
	},
}

// ValidationMapper maps a validator.FieldError to an ErrorValidationResponse
func ValidationMapper(validationError v10.FieldError) models.ErrorValidationResponse {
	code, message := ErrorValidationMapping(validationError)

	return models.ErrorValidationResponse{
		Code:    code,
		Field:   validationError.Field(),
		Message: message,
	}
}

// ErrorValidationMapping returns the error code and message for a validation error
// Uses map-based lookup for O(1) performance instead of switch statement
func ErrorValidationMapping(validationError v10.FieldError) (string, string) {
	tag := validationError.Tag()

	// Use map lookup for known validation tags
	if config, exists := validationErrorMap[tag]; exists {
		return config.Code, config.MessageBuilder(validationError)
	}

	// Default case for unknown validation tags
	return ErrorCodeInvalidField, fmt.Sprintf("invalid input on field %s: %s", validationError.Field(), tag)
}
