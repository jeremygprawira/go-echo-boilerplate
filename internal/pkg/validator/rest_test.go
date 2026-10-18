package validator

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCustomTags(t *testing.T) {
	cases := []struct {
		tag   string
		value string
		ok    bool
	}{
		{TagNoSpecial, "", true},
		{TagNoSpecial, "John Doe 2", true},
		{TagNoSpecial, "John!", false},
		{TagNoStartEndSpaces, "", true},
		{TagNoStartEndSpaces, "a b", true},
		{TagNoStartEndSpaces, " a", false},
		{TagNoStartEndSpaces, "a ", false},
		{TagDate, "2024-02-29", true},
		{TagDate, "29-02-2024", false},
		{TagDate, "x2024-02-29", false},
		{TagDate, "2024-02-29x", false},
		{TagDatetime, "2024-02-29 13:45:00", true},
		{TagDatetime, "2024-02-29", false},
		{TagDatetime, "2024-02-29 13:45:00x", false},
		{TagYYYMMDDFormat, "", true},
		{TagYYYMMDDFormat, "2024-02-29", true},
		{TagYYYMMDDFormat, "2023-02-29", false},
		{TagEmailFormat, "", true},
		{TagEmailFormat, "john@example.com", true},
		{TagEmailFormat, "john.example.com", false},
		{TagPhoneFormat, "", true},
		{TagPhoneFormat, "+6281234567890", true},
		{TagPhoneFormat, "+62812abc", false},
		{TagPhoneFormat, "123", false},
		{TagEmailPhoneFormat, "", true},
		{TagEmailPhoneFormat, "john@example.com", true},
		{TagEmailPhoneFormat, "+6281234567890", true},
		{TagEmailPhoneFormat, "not-a-contact", false},
		{TagPasswordMinLength, "", true},
		{TagPasswordMinLength, "12345678", true},
		{TagPasswordMinLength, "1234567", false},
		{TagPasswordMaxLength, strings.Repeat("a", 72), true},
		{TagPasswordMaxLength, strings.Repeat("a", 73), false},
		{TagPasswordStrength, "", true},
		{TagPasswordStrength, "Passw0rd!", true},
		{TagPasswordStrength, "passw0rd!", false},
		{TagPasswordStrength, "PASSW0RD!", false},
		{TagPasswordStrength, "Password!", false},
		{TagPasswordStrength, "Passw0rd", false},
	}

	for _, tc := range cases {
		t.Run(tc.tag+"/"+tc.value, func(t *testing.T) {
			err := valid.Var(tc.value, tc.tag)
			if tc.ok {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
			}
		})
	}
}

func TestEmailOrPhoneField(t *testing.T) {
	type contact struct {
		Email       string
		PhoneNumber string
		Check       string `validate:"emailOrPhoneField"`
	}

	cases := []struct {
		name  string
		input contact
		ok    bool
	}{
		{"email only", contact{Email: "john@example.com"}, true},
		{"phone only", contact{PhoneNumber: "+6281234567890"}, true},
		{"neither", contact{}, false},
		{"both", contact{Email: "john@example.com", PhoneNumber: "+6281234567890"}, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := Input(tc.input)
			if tc.ok {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
			}
		})
	}
}

func TestInput_MapsFieldErrorsToJSONNames(t *testing.T) {
	type request struct {
		Email string `json:"email" validate:"required"`
	}

	err := Input(request{})
	require.Error(t, err)
	require.Contains(t, err.Error(), "field: email")
	require.Contains(t, err.Error(), "code: "+ErrorCodeMissingField)
}
