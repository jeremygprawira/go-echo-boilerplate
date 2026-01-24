package formatter

import (
	"errors"
	"go-echo-boilerplate/internal/models"
	"go-echo-boilerplate/internal/pkg/stringc"

	"github.com/nyaruka/phonenumbers"
)

const (
	DefaultCountryCode        = "ID"
	IDPhonePrefix             = "08"
	IDE164PhonePrefix         = "62"
	IDE164PhonePrefixWithPlus = "+62"
)

func PhoneNumber(phoneNumber models.PhoneNumber) (formattedPhoneNumber *string, err error) {
	if stringc.ContainsAlphabet(phoneNumber.Number) {
		return nil, errors.New("phone number contains alphabet")
	}

	if phoneNumber.CountryCode == "" {
		phoneNumber.CountryCode = DefaultCountryCode
	}

	// parse to phone struct format
	parsedPhoneNumber, err := phonenumbers.Parse(phoneNumber.Number, phoneNumber.CountryCode)
	if err != nil {
		return nil, err
	}

	// check if phone number valid
	if !phonenumbers.IsValidNumberForRegion(parsedPhoneNumber, phoneNumber.CountryCode) {
		return nil, errors.New("invalid phone number")
	}

	// format to e164
	e164PhoneNumber := phonenumbers.Format(parsedPhoneNumber, phonenumbers.E164)

	return &e164PhoneNumber, nil
}
