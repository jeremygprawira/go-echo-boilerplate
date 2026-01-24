package validator

import (
	"errors"
	"go-echo-boilerplate/internal/models"
	"net/mail"
	"reflect"
	"regexp"
	"strings"
	"time"

	v10 "github.com/go-playground/validator/v10"
	"github.com/hashicorp/go-multierror"
)

var (
	// valid is the singleton validator instance
	valid = v10.New()

	// Pre-compiled regex patterns for performance
	noSpecialCharsPattern *regexp.Regexp
	datePattern           *regexp.Regexp
	datetimePattern       *regexp.Regexp
)

// Validate validates a struct and returns validation errors
func Input(request interface{}) error {
	var errs *multierror.Error
	if err := valid.Struct(request); err != nil {
		// This check is only needed when your code could produce
		// an invalid value for validation such as an interface with a nil
		// value. Most including myself do not usually have code like this.
		if _, ok := err.(*v10.InvalidValidationError); ok {
			errs = multierror.Append(errs, models.ErrorValidationResponse{
				Message: err.Error(),
			})
			return errs.ErrorOrNil()
		}

		var validatorErrs v10.ValidationErrors
		if errors.As(err, &validatorErrs) {
			for _, validatorErr := range validatorErrs {
				validationError := ValidationMapper(validatorErr)
				errs = multierror.Append(errs, validationError)
			}
		}
	}

	return errs.ErrorOrNil()
}

// init registers all custom validation functions and compiles regex patterns
// when the package is initialized
func init() {
	// Configure validator to use JSON tag names in error messages
	valid.RegisterTagNameFunc(func(fld reflect.StructField) string {
		name := strings.SplitN(fld.Tag.Get("json"), ",", 2)[0]
		if name == "-" {
			return ""
		}
		return name
	})

	// Pre-compile regex patterns for performance
	noSpecialCharsPattern = regexp.MustCompile(`^[a-zA-Z0-9 ]*$`)
	datePattern = regexp.MustCompile(`\d{4}-\d{2}-\d{2}`)
	datetimePattern = regexp.MustCompile(`\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2}`)

	// Register custom validation functions
	registerNoSpecialCharacters()
	registerNoSpacesAtStartOrEnd()
	registerDate()
	registerDatetime()
	registerEmailFormat()
	registerPhoneFormat()
	registerEmailPhoneFormat()
	registeryyyymmddFormat()
	registerEmailOrPhoneField()
	registerPasswordValidations()
}

// Helper function to safely get string value from field
func getStringValue(fl v10.FieldLevel) (string, bool) {
	field := fl.Field()

	// Handle different types safely
	switch field.Kind() {
	case reflect.String:
		return field.String(), true
	case reflect.Interface:
		if field.IsNil() {
			return "", true
		}
		if str, ok := field.Interface().(string); ok {
			return str, true
		}
	}

	return "", false
}

// Helper function to check if string is empty or whitespace
func isEmptyOrWhitespace(s string) bool {
	return s == "" || strings.TrimSpace(s) == ""
}

func registerNoSpecialCharacters() {
	if err := valid.RegisterValidation("nospecial", func(fl v10.FieldLevel) bool {
		str, ok := getStringValue(fl)
		if !ok {
			return false // Invalid type
		}

		if str == "" {
			return true // Empty strings are valid
		}

		trimmedInput := strings.TrimSpace(str)
		return noSpecialCharsPattern.MatchString(trimmedInput)
	}); err != nil {
		panic(err)
	}
}

func registerNoSpacesAtStartOrEnd() {
	if err := valid.RegisterValidation("noStartEndSpaces", func(fl v10.FieldLevel) bool {
		str, ok := getStringValue(fl)
		if !ok {
			return false // Invalid type
		}

		if str == "" {
			return true // Empty strings are valid
		}

		return str[0] != ' ' && str[len(str)-1] != ' '
	}); err != nil {
		panic(err)
	}
}

func registerDate() {
	if err := valid.RegisterValidation("date", func(fl v10.FieldLevel) bool {
		str, ok := getStringValue(fl)
		if !ok {
			return false // Invalid type
		}

		return datePattern.MatchString(str)
	}); err != nil {
		panic(err)
	}
}

func registerDatetime() {
	if err := valid.RegisterValidation("datetime", func(fl v10.FieldLevel) bool {
		str, ok := getStringValue(fl)
		if !ok {
			return false // Invalid type
		}

		return datetimePattern.MatchString(str)
	}); err != nil {
		panic(err)
	}
}

func registerEmailFormat() {
	if err := valid.RegisterValidation("emailFormat", func(fl v10.FieldLevel) bool {
		str, ok := getStringValue(fl)
		if !ok {
			return false // Invalid type
		}

		if str == "" {
			return true // Empty strings are valid
		}

		trimmedInput := strings.TrimSpace(str)
		_, err := mail.ParseAddress(trimmedInput)
		return err == nil
	}); err != nil {
		panic(err)
	}
}

func registerPhoneFormat() {
	if err := valid.RegisterValidation("phoneFormat", func(fl v10.FieldLevel) bool {
		str, ok := getStringValue(fl)
		if !ok {
			return false // Invalid type
		}

		if str == "" {
			return true // Empty strings are valid
		}

		isValid, err := PhoneNumber("", str)
		if err != nil {
			return false
		}

		return isValid
	}); err != nil {
		panic(err)
	}
}

func registerEmailPhoneFormat() {
	if err := valid.RegisterValidation("emailPhoneFormat", func(fl v10.FieldLevel) bool {
		str, ok := getStringValue(fl)
		if !ok {
			return false // Invalid type
		}

		if str == "" {
			return true // Empty strings are valid
		}

		trimmedInput := strings.TrimSpace(str)

		// Check if it's a valid email
		_, emailErr := mail.ParseAddress(trimmedInput)
		if emailErr == nil {
			return true
		}

		// Check if it's a valid phone number
		isValidPhone, phoneErr := PhoneNumber("", trimmedInput)
		if phoneErr == nil && isValidPhone {
			return true
		}

		return false
	}); err != nil {
		panic(err)
	}
}

func registeryyyymmddFormat() {
	if err := valid.RegisterValidation("yyymmddFormat", func(fl v10.FieldLevel) bool {
		str, ok := getStringValue(fl)
		if !ok {
			return false // Invalid type
		}

		if str == "" {
			return true // Empty strings are valid
		}

		_, err := time.Parse("2006-01-02", str)
		return err == nil // Returns true if date is valid
	}); err != nil {
		panic(err)
	}
}

func registerEmailOrPhoneField() {
	if err := valid.RegisterValidation("emailOrPhoneField", func(fl v10.FieldLevel) bool {
		parent := fl.Parent()

		// Safely get Email field
		emailField := parent.FieldByName("Email")
		if !emailField.IsValid() {
			return false
		}
		email := emailField.String()

		// Safely get PhoneNumber field
		phoneField := parent.FieldByName("PhoneNumber")
		if !phoneField.IsValid() {
			return false
		}
		phone := phoneField.String()

		// Exactly one of email or phone must be filled
		if (email != "" && phone == "") || (email == "" && phone != "") {
			return true
		}

		// If neither is filled, it's invalid
		if email == "" && phone == "" {
			return false
		}

		// If both are filled, it's valid (you can adjust this logic if needed)
		return true
	}); err != nil {
		panic(err)
	}
}
