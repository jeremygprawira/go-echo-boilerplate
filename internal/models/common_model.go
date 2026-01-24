package models

type PhoneNumber struct {
	CountryCode string `json:"countryCode" validate:"omitempty" example:"ID"`
	Number      string `json:"number" validate:"omitempty,phoneFormat" example:"6281234567890"`
}
