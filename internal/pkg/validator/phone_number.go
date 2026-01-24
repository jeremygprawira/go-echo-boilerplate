package validator

import (
	"errors"
	stringc "go-echo-boilerplate/internal/pkg/stringc"

	"github.com/nyaruka/phonenumbers"
)

const (
	DefaultCountryCode        = "ID"
	IDPhonePrefix             = "08"
	IDE164PhonePrefix         = "62"
	IDE164PhonePrefixWithPlus = "+62"
)

func PhoneNumber(countryCode, phoneNumber string) (isValid bool, err error) {
	if stringc.ContainsAlphabet(phoneNumber) {
		return false, errors.New("phone number contains alphabet")
	}

	if countryCode == "" {
		countryCode = DefaultCountryCode
	}

	// parse to phone struct format
	parsedPhoneNumber, err := phonenumbers.Parse(phoneNumber, countryCode)
	if err != nil {
		return false, err
	}

	// check if phone number valid
	if !phonenumbers.IsValidNumberForRegion(parsedPhoneNumber, countryCode) {
		return false, errors.New("invalid phone number")
	}

	return true, nil
}
